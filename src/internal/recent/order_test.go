package recent

import (
	"testing"

	"github.com/MorganKryze/tuna/src/internal/config"
)

func TestBumpMovesToFrontWithoutDuplicating(t *testing.T) {
	cases := []struct {
		name   string
		start  []string
		chosen string
		want   []string
	}{
		{"already at the front", []string{"a", "b", "c"}, "a", []string{"a", "b", "c"}},
		{"in the middle", []string{"a", "b", "c"}, "b", []string{"b", "a", "c"}},
		{"never seen", []string{"a", "b"}, "z", []string{"z", "a", "b"}},
		{"empty list", nil, "a", []string{"a"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Bump(c.start, c.chosen); !equal(got, c.want) {
				t.Fatalf("want %v, got %v", c.want, got)
			}
		})
	}
}

// The order file outlives the config: a destination deleted from the TOML
// must vanish from the list rather than show up as an entry that cannot be
// launched.
func TestOrderPutsRecentFirstAndDropsUnknowns(t *testing.T) {
	dests := []config.Destination{{Name: "a"}, {Name: "b"}, {Name: "c"}}

	got := names(Order(dests, []string{"c", "gone", "a"}))
	if want := []string{"c", "a", "b"}; !equal(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}

	// Never used: config order stands, nothing is lost.
	got = names(Order(dests, nil))
	if want := []string{"a", "b", "c"}; !equal(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}

	// A name twice over in a hand-edited file must not duplicate the row.
	got = names(Order(dests, []string{"b", "b", "a"}))
	if want := []string{"b", "a", "c"}; !equal(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}
