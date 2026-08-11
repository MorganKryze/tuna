package pick

import "testing"

func TestReadKeyTellsArrowsFromEscape(t *testing.T) {
	cases := []struct {
		nom     string
		in      []byte
		attendu Key
		rune    rune
	}{
		{"flèche haut", []byte{0x1b, '[', 'A'}, KeyUp, 0},
		{"flèche bas", []byte{0x1b, '[', 'B'}, KeyDown, 0},
		{"Échap seul", []byte{0x1b}, KeyEsc, 0},
		// In raw mode the terminal stops generating signals, so Ctrl-C is a
		// byte to read rather than a SIGINT to catch.
		{"Ctrl-C", []byte{0x03}, KeyEsc, 0},
		{"Entrée", []byte{'\r'}, KeyEnter, 0},
		{"Entrée unix", []byte{'\n'}, KeyEnter, 0},
		{"Retour arrière", []byte{0x7f}, KeyBackspace, 0},
		{"Retour arrière ancien", []byte{0x08}, KeyBackspace, 0},
		{"caractère", []byte{'h'}, KeyRune, 'h'},
		{"tiret", []byte{'-'}, KeyRune, '-'},
		{"rien", nil, KeyNone, 0},
		// Left and right are ESC [ C/D: recognised as a sequence, then
		// ignored. Falling through to KeyEsc would quit the picker.
		{"flèche droite ignorée", []byte{0x1b, '[', 'C'}, KeyNone, 0},
		{"flèche gauche ignorée", []byte{0x1b, '[', 'D'}, KeyNone, 0},
		// A control byte with no meaning must not become a filter character.
		{"octet de contrôle", []byte{0x04}, KeyNone, 0},
		// A truncated escape sequence is a bare Escape, which is the read
		// that lets Escape work at all: it never arrives with anything after.
		{"séquence tronquée", []byte{0x1b, '['}, KeyEsc, 0},
	}
	for _, c := range cases {
		t.Run(c.nom, func(t *testing.T) {
			k, r := readKey(c.in)
			if k != c.attendu || r != c.rune {
				t.Fatalf("attendu (%v, %q), obtenu (%v, %q)", c.attendu, c.rune, k, r)
			}
		})
	}
}
