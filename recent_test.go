package main

import (
	"os"
	"path/filepath"
	"testing"
)

func names(dests []Destination) []string {
	out := make([]string, len(dests))
	for i, d := range dests {
		out[i] = d.Name
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestBumpMovesToFrontWithoutDuplicating(t *testing.T) {
	cases := []struct {
		nom     string
		start   []string
		chosen  string
		attendu []string
	}{
		{"déjà en tête", []string{"a", "b", "c"}, "a", []string{"a", "b", "c"}},
		{"au milieu", []string{"a", "b", "c"}, "b", []string{"b", "a", "c"}},
		{"jamais vu", []string{"a", "b"}, "z", []string{"z", "a", "b"}},
		{"liste vide", nil, "a", []string{"a"}},
	}
	for _, c := range cases {
		t.Run(c.nom, func(t *testing.T) {
			if got := Bump(c.start, c.chosen); !equal(got, c.attendu) {
				t.Fatalf("attendu %v, obtenu %v", c.attendu, got)
			}
		})
	}
}

// The order file outlives the config: a destination deleted from the TOML
// must vanish from the list rather than show up as an entry that cannot be
// launched.
func TestOrderPutsRecentFirstAndDropsUnknowns(t *testing.T) {
	dests := []Destination{{Name: "a"}, {Name: "b"}, {Name: "c"}}

	got := names(Order(dests, []string{"c", "disparue", "a"}))
	if attendu := []string{"c", "a", "b"}; !equal(got, attendu) {
		t.Fatalf("attendu %v, obtenu %v", attendu, got)
	}

	// Never used: config order stands, nothing is lost.
	got = names(Order(dests, nil))
	if attendu := []string{"a", "b", "c"}; !equal(got, attendu) {
		t.Fatalf("attendu %v, obtenu %v", attendu, got)
	}
}

func TestRecentSurvivesARoundTripAndAMissingFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sous-dossier", "recent")

	// A first run has no state file, and that is not an error.
	if got := LoadRecent(p); len(got) != 0 {
		t.Fatalf("fichier absent doit donner une liste vide, obtenu %v", got)
	}
	// SaveRecent creates the directory: ~/.local/state/tuna does not exist
	// on a fresh machine.
	if err := SaveRecent(p, []string{"b", "a"}); err != nil {
		t.Fatal(err)
	}
	if got := LoadRecent(p); !equal(got, []string{"b", "a"}) {
		t.Fatalf("attendu [b a], obtenu %v", got)
	}

	// A corrupt file is not worth an error either: the worst outcome is a
	// list in the wrong order, and refusing to start over it would be absurd.
	if err := os.WriteFile(p, []byte("\n\n  \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := LoadRecent(p); len(got) != 0 {
		t.Fatalf("lignes vides doivent être ignorées, obtenu %v", got)
	}
}
