package pick

import (
	"testing"

	"github.com/MorganKryze/tuna/src/internal/config"
)

func demo() Picker {
	return Picker{All: []config.Destination{
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
	// Down on an empty list must not walk the cursor off the end either.
	if next, _, _ = next.Update(KeyDown, 0); next.Cursor != 0 {
		t.Fatalf("bas sur une liste vide doit rester à 0, obtenu %d", next.Cursor)
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

func TestFilterMatchesTheDescriptionToo(t *testing.T) {
	p := demo()
	for _, r := range "duplicati" {
		p, _, _ = p.Update(KeyRune, r)
	}
	if got := p.Matches(); len(got) != 2 {
		t.Fatalf("le filtre doit aussi lire desc, obtenu %d correspondances", len(got))
	}
}

// A filtered Enter must select what is under the cursor in the *filtered*
// list, not the row that sat at that index before typing.
func TestEnterSelectsFromTheFilteredList(t *testing.T) {
	p := demo()
	for _, r := range "duplicati" {
		p, _, _ = p.Update(KeyRune, r)
	}
	p, _, _ = p.Update(KeyDown, 0)
	if _, chosen, _ := p.Update(KeyEnter, 0); chosen != "vm-backup" {
		t.Fatalf("attendu vm-backup, obtenu %q", chosen)
	}
}

// An unmapped key must leave the state exactly as it was: no filter change,
// no cursor move, no selection.
func TestAnUnknownKeyChangesNothing(t *testing.T) {
	p := demo()
	p, _, _ = p.Update(KeyDown, 0)
	next, chosen, done := p.Update(KeyNone, 0)
	if done || chosen != "" || next.Cursor != 1 || next.Filter != "" {
		t.Fatalf("KeyNone doit être neutre, obtenu %+v chosen=%q done=%v", next, chosen, done)
	}
}
