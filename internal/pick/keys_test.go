// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

package pick

import "testing"

func TestReadKeyTellsArrowsFromEscape(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want Key
		rune rune
	}{
		{"arrow up", []byte{0x1b, '[', 'A'}, KeyUp, 0},
		{"arrow down", []byte{0x1b, '[', 'B'}, KeyDown, 0},
		{"bare escape", []byte{0x1b}, KeyEsc, 0},
		// In raw mode the terminal stops generating signals, so Ctrl-C is a
		// byte to read rather than a SIGINT to catch.
		{"Ctrl-C", []byte{0x03}, KeyEsc, 0},
		{"enter", []byte{'\r'}, KeyEnter, 0},
		{"enter, unix", []byte{'\n'}, KeyEnter, 0},
		{"backspace", []byte{0x7f}, KeyBackspace, 0},
		{"backspace, old", []byte{0x08}, KeyBackspace, 0},
		{"a character", []byte{'h'}, KeyRune, 'h'},
		{"a dash", []byte{'-'}, KeyRune, '-'},
		{"nothing", nil, KeyNone, 0},
		// Left and right are ESC [ C/D: recognised as a sequence, then
		// ignored. Falling through to KeyEsc would quit the picker.
		{"arrow right, ignored", []byte{0x1b, '[', 'C'}, KeyNone, 0},
		{"arrow left, ignored", []byte{0x1b, '[', 'D'}, KeyNone, 0},
		// A control byte with no meaning must not become a filter character.
		{"a control byte", []byte{0x04}, KeyNone, 0},
		// A truncated escape sequence is a bare Escape, which is the read
		// that lets Escape work at all: it never arrives with anything after.
		{"a truncated sequence", []byte{0x1b, '['}, KeyEsc, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			k, r := readKey(c.in)
			if k != c.want || r != c.rune {
				t.Fatalf("want (%v, %q), got (%v, %q)", c.want, c.rune, k, r)
			}
		})
	}
}

// readKey used to cast the first byte to a rune, so "é" became "Ã" and its
// second byte vanished. The config file this filters against ships accented
// descriptions, so the case is not hypothetical.
func TestReadKeyDecodesUTF8(t *testing.T) {
	for _, c := range []struct {
		name string
		in   string
		want rune
	}{
		{"accented", "é", 'é'},
		{"beyond the BMP", "🐟", '🐟'},
		{"plain ASCII still works", "h", 'h'},
	} {
		t.Run(c.name, func(t *testing.T) {
			k, r := readKey([]byte(c.in))
			if k != KeyRune || r != c.want {
				t.Fatalf("want (KeyRune, %q), got (%v, %q)", c.want, k, r)
			}
		})
	}
	// Half of a rune is not a character, and must not become one.
	if k, _ := readKey([]byte{0xC3}); k != KeyNone {
		t.Errorf("a stray continuation byte must not reach the filter, got %v", k)
	}
}
