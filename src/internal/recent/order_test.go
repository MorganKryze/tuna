package recent

import (
	"testing"

	"github.com/MorganKryze/tuna/src/internal/config"
)

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
	dests := []config.Destination{{Name: "a"}, {Name: "b"}, {Name: "c"}}

	got := names(Order(dests, []string{"c", "disparue", "a"}))
	if attendu := []string{"c", "a", "b"}; !equal(got, attendu) {
		t.Fatalf("attendu %v, obtenu %v", attendu, got)
	}

	// Never used: config order stands, nothing is lost.
	got = names(Order(dests, nil))
	if attendu := []string{"a", "b", "c"}; !equal(got, attendu) {
		t.Fatalf("attendu %v, obtenu %v", attendu, got)
	}

	// A name twice over in a hand-edited file must not duplicate the row.
	got = names(Order(dests, []string{"b", "b", "a"}))
	if attendu := []string{"b", "a", "c"}; !equal(got, attendu) {
		t.Fatalf("attendu %v, obtenu %v", attendu, got)
	}
}
