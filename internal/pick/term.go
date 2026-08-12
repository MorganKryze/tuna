// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

package pick

import (
	"context"
	"errors"
	"fmt"
	"os"

	"golang.org/x/term"

	"github.com/MorganKryze/tunny/internal/config"
	"github.com/MorganKryze/tunny/internal/ui"
)

// ErrNoChoice is what Escape produces: not a failure, just nothing to do.
var ErrNoChoice = errors.New("no destination chosen")

// Pick draws the list and returns the chosen name, or ErrNoChoice.
//
// This is the only function in the package that touches a terminal, and the
// only one with no test: everything it does beyond the read-draw loop lives
// in pick.go, keys.go and render.go, where it is exercised without a TTY.
//
// The drawing goes to stderr rather than stdout, so tunny stays usable in a
// pipe without the menu ending up in the data.
func Pick(ctx context.Context, dests []config.Destination, busy map[string][]int) (string, error) {
	tty := os.Stdin
	if !term.IsTerminal(int(tty.Fd())) {
		return "", errors.New("not a terminal: pass the destination name as an argument")
	}
	state, err := term.MakeRaw(int(tty.Fd()))
	if err != nil {
		return "", err
	}
	// Restoring the terminal is not optional: leaving raw mode on means the
	// shell that follows has no echo and no line editing.
	defer term.Restore(int(tty.Fd()), state)

	// A signal is not a keystroke, and tty.Read blocks until one arrives. The
	// read happens in its own goroutine so the loop can watch the context too:
	// without it, a SIGTERM while the list is open leaves tunny waiting for a
	// key that will never come, holding a terminal in raw mode.
	//
	// The goroutine outlives this function, parked in a read nobody will
	// answer. That is deliberate: it happens once, on the way out of the
	// program, and closing the terminal underneath it would be worse.
	keys, readErr := make(chan []byte), make(chan error, 1)
	go func() {
		for {
			buf := make([]byte, 8)
			n, err := tty.Read(buf)
			if err != nil {
				readErr <- err
				return
			}
			keys <- buf[:n]
		}
	}()

	out := os.Stderr
	color := ui.ColorOK(out)
	p := Picker{All: dests, Busy: busy}
	for {
		// Re-read the size every frame: a window resized mid-pick would
		// otherwise keep drawing to the width it had when it started.
		width, height := size(tty)
		frame := p.Frame(width, height, color)
		fmt.Fprint(out, hideCursor+frame+showCursor)

		var buf []byte
		select {
		case <-ctx.Done():
			windBack(out, Lines(frame))
			// Asking tunny to stop is not a failure, and it is not a choice
			// either. The caller treats both the same way.
			return "", ErrNoChoice
		case err := <-readErr:
			windBack(out, Lines(frame))
			return "", err
		case buf = <-keys:
		}
		// Wind back the frame that is on screen, before the keystroke can
		// change how tall the next one is.
		windBack(out, Lines(frame))

		k, r := readKey(buf)
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
