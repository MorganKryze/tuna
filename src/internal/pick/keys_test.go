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
