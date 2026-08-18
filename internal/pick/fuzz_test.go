// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

package pick

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/MorganKryze/tunny/internal/ui"
)

// The picker is the one part of tunny that takes bytes straight off a
// terminal, and a terminal sends more than letters: arrows, function keys,
// mouse reports, a paste arriving in one read, half a character arriving in
// the next. Every bug the renderer has had came in through here — the panic
// between one and five columns, the accent turned into a stray byte — and
// each was found by a hand-written case that happened to be tried.
//
// The invariant is the one the whole drawing depends on: no line is ever
// wider than the terminal. A wider one wraps, a wrapped frame is taller than
// Lines counts it, and the wind-back then erases the wrong lines on every
// keystroke.
func FuzzTheFrameNeverOverflows(f *testing.F) {
	f.Add([]byte("\x1b[A"), uint8(80), uint8(24)) // an arrow
	f.Add([]byte("\x1b"), uint8(1), uint8(1))     // a bare Escape, in one column
	f.Add([]byte("\x1b[1;2C"), uint8(20), uint8(3))
	f.Add([]byte("dupl"), uint8(21), uint8(5))
	f.Add([]byte("é"), uint8(24), uint8(24))
	f.Add([]byte{0xC3}, uint8(5), uint8(24)) // half of an accent
	f.Add([]byte("世界"), uint8(23), uint8(6)) // two columns per rune
	f.Add([]byte{0x7F, 0x7F}, uint8(80), uint8(0))

	dests := realDests()
	f.Fuzz(func(t *testing.T, keys []byte, w, h uint8) {
		width, height := int(w), int(h)
		p := Picker{All: dests, Busy: map[string][]int{"gateway": {8200}}}
		for len(keys) > 0 {
			// Eight bytes at a time, because that is the buffer term.go
			// reads into: a longer paste arrives as several reads, and a
			// sequence can be split across two of them.
			n := min(len(keys), 8)
			k, r := readKey(keys[:n])
			keys = keys[n:]

			// readKey either recognises a character or says it did not. A
			// rune that is neither is one the filter would carry as broken
			// UTF-8 until something downstream sliced it in half.
			if k == KeyRune && !utf8.ValidRune(r) {
				t.Fatalf("readKey returned an invalid rune %U", r)
			}

			var done bool
			p, _, done = p.Update(k, r)
			frame := p.Frame(width, height, true)
			if width > 0 {
				for _, l := range strings.Split(plain(frame), "\r\n") {
					if got := ui.Width(l); got > width {
						t.Fatalf("width %d: a line %d columns wide: %q", width, got, l)
					}
				}
			}
			if done {
				return
			}
		}
	})
}
