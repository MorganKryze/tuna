package pick

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/MorganKryze/tuna/src/internal/config"
)

// ErrNoChoice is what Escape produces: not a failure, just nothing to do.
var ErrNoChoice = errors.New("aucune destination choisie")

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
//
// This is the only function in the package that touches a terminal, and the
// only one with no test: everything it does beyond the read-draw loop lives
// in pick.go and keys.go, where it is exercised without a TTY.
//
// The drawing goes to stderr rather than stdout, so tuna stays usable in a
// pipe without the menu ending up in the data.
func Pick(dests []config.Destination) (string, error) {
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
