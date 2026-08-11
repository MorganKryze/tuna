package pick

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/term"

	"github.com/MorganKryze/tuna/src/internal/config"
	"github.com/MorganKryze/tuna/src/internal/ui"
)

// ErrNoChoice is what Escape produces: not a failure, just nothing to do.
var ErrNoChoice = errors.New("aucune destination choisie")

// Pick draws the list and returns the chosen name, or ErrNoChoice.
//
// This is the only function in the package that touches a terminal, and the
// only one with no test: everything it does beyond the read-draw loop lives
// in pick.go, keys.go and render.go, where it is exercised without a TTY.
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

	out := os.Stderr
	color := ui.ColorOK(out)
	p := Picker{All: dests}
	buf := make([]byte, 8)
	for {
		// Re-read the size every frame: a window resized mid-pick would
		// otherwise keep drawing to the width it had when it started.
		width, height := size(tty)
		frame := p.Frame(width, height, color)
		fmt.Fprint(out, hideCursor+frame+showCursor)

		n, err := tty.Read(buf)
		if err != nil {
			windBack(out, Lines(frame))
			return "", err
		}
		// Wind back the frame that is on screen, before the keystroke can
		// change how tall the next one is.
		windBack(out, Lines(frame))

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

const (
	hideCursor = "\x1b[?25l"
	showCursor = "\x1b[?25h"
)

// windBack moves up n lines and clears everything below, erasing the frame
// currently on screen.
func windBack(w *os.File, n int) {
	if n > 0 {
		fmt.Fprintf(w, "\x1b[%dF\x1b[J", n)
	}
}

func size(tty *os.File) (width, height int) {
	width, height, err := term.GetSize(int(tty.Fd()))
	if err != nil {
		return 80, 24
	}
	return width, height
}
