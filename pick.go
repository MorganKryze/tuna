package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

type Key int

const (
	KeyNone Key = iota
	KeyUp
	KeyDown
	KeyEnter
	KeyEsc
	KeyBackspace
	KeyRune
)

// Picker is the whole state of the list: what it shows, what has been typed,
// and where the cursor is. Update is a pure function of it, which is why the
// navigation can be tested without a terminal.
type Picker struct {
	All    []Destination // already ordered by recency
	Filter string
	Cursor int
}

// Matches is a plain substring match on name and description, case folded.
// Not fuzzy: with under a dozen destinations, fuzzy matching only adds ways
// to be surprised by the ranking.
func (p Picker) Matches() []Destination {
	if p.Filter == "" {
		return p.All
	}
	needle := strings.ToLower(p.Filter)
	var out []Destination
	for _, d := range p.All {
		hay := strings.ToLower(d.Name + " " + d.Desc)
		if strings.Contains(hay, needle) {
			out = append(out, d)
		}
	}
	return out
}

// Update applies one keystroke. It returns the next state, the chosen name
// (empty if none) and whether the picker is finished.
func (p Picker) Update(k Key, r rune) (Picker, string, bool) {
	switch k {
	case KeyEsc:
		return p, "", true

	case KeyEnter:
		matches := p.Matches()
		// Enter on an empty list selects nothing rather than indexing into
		// it. Typing a filter that matches nothing is easy to do by accident.
		if len(matches) == 0 {
			return p, "", false
		}
		return p, matches[p.Cursor].Name, true

	case KeyUp:
		if p.Cursor > 0 {
			p.Cursor--
		}

	case KeyDown:
		if p.Cursor < len(p.Matches())-1 {
			p.Cursor++
		}

	case KeyBackspace:
		// Byte-wise, which is safe because readKey only ever emits ASCII:
		// a filter is typed against destination names, and those are ASCII
		// by convention. Accept a multi-byte rune here and this line starts
		// cutting UTF-8 in half.
		if p.Filter != "" {
			p.Filter = p.Filter[:len(p.Filter)-1]
		}

	case KeyRune:
		p.Filter += string(r)
	}

	// Editing the filter changes how many rows exist under the cursor, so it
	// is clamped after every keystroke rather than in each branch.
	if n := len(p.Matches()); p.Cursor >= n {
		p.Cursor = max(n-1, 0)
	}
	return p, "", false
}

// ErrNoChoice is what Escape produces: not a failure, just nothing to do.
var ErrNoChoice = errors.New("aucune destination choisie")

// readKey turns raw bytes into a Key. Arrow keys arrive as ESC [ A, and a
// bare ESC arrives alone, so a lone 0x1b with nothing behind it is Escape.
func readKey(buf []byte) (Key, rune) {
	switch {
	case len(buf) == 0:
		return KeyNone, 0
	case buf[0] == 0x1b && len(buf) >= 3 && buf[1] == '[':
		switch buf[2] {
		case 'A':
			return KeyUp, 0
		case 'B':
			return KeyDown, 0
		}
		return KeyNone, 0
	case buf[0] == 0x1b:
		return KeyEsc, 0
	case buf[0] == '\r' || buf[0] == '\n':
		return KeyEnter, 0
	case buf[0] == 0x7f || buf[0] == 0x08:
		return KeyBackspace, 0
	// Ctrl-C in raw mode is a byte, not a signal: the terminal is not
	// generating signals while we hold it.
	case buf[0] == 0x03:
		return KeyEsc, 0
	case buf[0] >= 0x20:
		return KeyRune, rune(buf[0])
	}
	return KeyNone, 0
}

// render draws the list and returns how many lines it wrote, which is what
// the caller needs to wind it back. Recomputing that number after the next
// keystroke would be wrong: a keystroke that changes the filter changes how
// many rows there are, and the drawing to undo is the one already on screen.
func (p Picker) render(w *os.File) int {
	matches := p.Matches()
	fmt.Fprintf(w, "\x1b[?25l") // hide the cursor while redrawing
	fmt.Fprintf(w, "Destination : %s\r\n", p.Filter)
	for i, d := range matches {
		marker := "  "
		if i == p.Cursor {
			marker = "> "
		}
		ports := make([]string, 0, len(d.Forward))
		for _, f := range d.Forward {
			label := f.Label
			if label == "" {
				label = fmt.Sprintf("%d", f.Local)
			}
			ports = append(ports, fmt.Sprintf("%s:%d", label, f.Local))
		}
		fmt.Fprintf(w, "%s%-14s %s  [%s]\r\n", marker, d.Name, d.Desc, strings.Join(ports, " "))
	}
	if len(matches) == 0 {
		fmt.Fprintf(w, "  (aucune correspondance)\r\n")
	}
	fmt.Fprintf(w, "\x1b[?25h") // show it again
	return 1 + max(len(matches), 1)
}

// Pick draws the list and returns the chosen name, or ErrNoChoice.
func Pick(dests []Destination) (string, error) {
	tty := os.Stdin
	if !term.IsTerminal(int(tty.Fd())) {
		return "", errors.New("pas de terminal : donne le nom de la destination en argument")
	}
	state, err := term.MakeRaw(int(tty.Fd()))
	if err != nil {
		return "", err
	}
	// Restoring the terminal is not optional: leaving raw mode on means the
	// shell that follows has no echo and no line editing.
	defer term.Restore(int(tty.Fd()), state)

	p := Picker{All: dests}
	buf := make([]byte, 8)
	for {
		drawn := p.render(os.Stderr)
		n, err := tty.Read(buf)
		if err != nil {
			return "", err
		}
		// Wind back the frame that is on screen, before the keystroke can
		// change how tall the next one is.
		fmt.Fprintf(os.Stderr, "\x1b[%dF\x1b[J", drawn)

		k, r := readKey(buf[:n])
		next, chosen, done := p.Update(k, r)
		p = next
		if done {
			if chosen == "" {
				return "", ErrNoChoice
			}
			return chosen, nil
		}
	}
}
