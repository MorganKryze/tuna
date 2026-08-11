package main

import "testing"

func demo() Picker {
	return Picker{All: []Destination{
		{Name: "hyperviseur", Desc: "Cockpit"},
		{Name: "gateway", Desc: "Duplicati"},
		{Name: "vm-backup", Desc: "Duplicati d'une VM"},
	}}
}

func TestCursorStopsAtBothEnds(t *testing.T) {
	p := demo()
	// Up at the top stays at the top rather than wrapping: wrapping in a
	// three-line list reads as a glitch, not as a feature.
	if next, _, _ := p.Update(KeyUp, 0); next.Cursor != 0 {
		t.Fatalf("haut en tête doit rester à 0, obtenu %d", next.Cursor)
	}
	for range 5 {
		p, _, _ = p.Update(KeyDown, 0)
	}
	if p.Cursor != 2 {
		t.Fatalf("bas en fin de liste doit rester à 2, obtenu %d", p.Cursor)
	}
}

func TestFilterNarrowsAndKeepsTheCursorValid(t *testing.T) {
	p := demo()
	p.Cursor = 2 // on vm-backup
	for _, r := range "hyper" {
		p, _, _ = p.Update(KeyRune, r)
	}
	if got := p.Matches(); len(got) != 1 || got[0].Name != "hyperviseur" {
		t.Fatalf("le filtre doit garder la seule correspondance, obtenu %v", got)
	}
	// The cursor pointed at the third row, and there is only one row left.
	if p.Cursor != 0 {
		t.Fatalf("le curseur doit rester dans la liste filtrée, obtenu %d", p.Cursor)
	}
}

// A filter matching nothing must not make Enter select a row that is not
// there. This is the crash the test exists to prevent.
func TestAFilterMatchingNothingSelectsNothing(t *testing.T) {
	p := demo()
	for _, r := range "zzz" {
		p, _, _ = p.Update(KeyRune, r)
	}
	if len(p.Matches()) != 0 {
		t.Fatal("le filtre ne devait rien laisser")
	}
	next, chosen, done := p.Update(KeyEnter, 0)
	if done || chosen != "" {
		t.Fatalf("Entrée sur une liste vide ne doit rien choisir, obtenu %q done=%v", chosen, done)
	}
	if next.Cursor != 0 {
		t.Fatalf("curseur invalide : %d", next.Cursor)
	}
}

func TestBackspaceWidensAgain(t *testing.T) {
	p := demo()
	for _, r := range "hyper" {
		p, _, _ = p.Update(KeyRune, r)
	}
	for range 5 {
		p, _, _ = p.Update(KeyBackspace, 0)
	}
	if len(p.Matches()) != 3 {
		t.Fatalf("filtre vidé doit tout remontrer, obtenu %d", len(p.Matches()))
	}
	// Backspace on an empty filter is a no-op, not a panic.
	if next, _, _ := p.Update(KeyBackspace, 0); next.Filter != "" {
		t.Fatalf("filtre attendu vide, obtenu %q", next.Filter)
	}
}

func TestEnterAndEscape(t *testing.T) {
	p := demo()
	p, _, _ = p.Update(KeyDown, 0)
	_, chosen, done := p.Update(KeyEnter, 0)
	if !done || chosen != "gateway" {
		t.Fatalf("Entrée doit choisir la ligne sous le curseur, obtenu %q done=%v", chosen, done)
	}

	_, chosen, done = demo().Update(KeyEsc, 0)
	if !done || chosen != "" {
		t.Fatalf("Échap doit terminer sans choisir, obtenu %q done=%v", chosen, done)
	}
}

// readKey is the one parser in the program, and its hard case is that an
// arrow and a bare Escape start with the same byte: read it wrong and Escape
// either never fires or fires on every arrow.
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
		{"caractère", []byte{'h'}, KeyRune, 'h'},
		{"rien", nil, KeyNone, 0},
		// Left and right are ESC [ C/D: recognised as a sequence, then
		// ignored. Falling through to KeyEsc would quit the picker.
		{"flèche droite ignorée", []byte{0x1b, '[', 'C'}, KeyNone, 0},
		// A control byte with no meaning must not become a filter character.
		{"octet de contrôle", []byte{0x04}, KeyNone, 0},
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

func TestFilterMatchesTheDescriptionToo(t *testing.T) {
	p := demo()
	for _, r := range "duplicati" {
		p, _, _ = p.Update(KeyRune, r)
	}
	if got := p.Matches(); len(got) != 2 {
		t.Fatalf("le filtre doit aussi lire desc, obtenu %d correspondances", len(got))
	}
}
