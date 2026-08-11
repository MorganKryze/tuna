package pick

import (
	"regexp"
	"strings"
	"testing"

	"github.com/MorganKryze/tuna/src/internal/config"
	"github.com/MorganKryze/tuna/src/internal/ui"
)

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plain(s string) string { return ansi.ReplaceAllString(s, "") }

func real() []config.Destination {
	return []config.Destination{
		{Name: "hyperviseur", Desc: "Cockpit + Komodo — racine d'admin", Forward: []config.Forward{
			{Local: 9090, To: "127.0.0.1:9090", Label: "Cockpit"},
			{Local: 9120, To: "10.10.10.10:9120", Label: "Komodo"},
		}},
		{Name: "gateway", Desc: "Duplicati du gateway", Forward: []config.Forward{
			{Local: 8200, To: "127.0.0.1:8200", Label: "Duplicati"},
		}},
		{Name: "control-plane", Desc: "Duplicati du control-plane (Mongo Komodo)", Forward: []config.Forward{
			{Local: 8201, To: "127.0.0.1:8200", Label: "Duplicati"},
		}},
	}
}

// rows is the body of a frame, by position: blank, prompt, blank, rows…,
// blank, hint, and a trailing empty element from the final line ending.
func rows(frame string) []string {
	lines := strings.Split(plain(frame), "\r\n")
	return lines[3 : len(lines)-3]
}

// The whole frame, once, at a known size. A golden test earns its keep here:
// column alignment is the kind of thing that is obvious to a human and
// invisible to an assertion about substrings.
func TestFrameLooksLikeThis(t *testing.T) {
	got := plain(Picker{All: real()}.Frame(80, 24, false))
	want := "" +
		"\r\n" +
		"  ❯ ▏tapez pour filtrer                                                      3/3\r\n" +
		"\r\n" +
		"  ▌ hyperviseur     Cockpit + Komodo — racine d'admin                 9090  9120\r\n" +
		"    gateway         Duplicati du gateway                                    8200\r\n" +
		"    control-plane   Duplicati du control-plane (Mongo Komodo)               8201\r\n" +
		"\r\n" +
		"    ↑↓ naviguer    ⏎ ouvrir    ⎋ annuler\r\n"
	if got != want {
		t.Fatalf("rendu inattendu.\n--- obtenu ---\n%s\n--- attendu ---\n%s", got, want)
	}
}

// The single most important property of the renderer, and not an aesthetic
// one: a row wider than the terminal wraps, the frame becomes taller than
// Lines reports, and the wind-back then eats the wrong lines and tears the
// screen apart on every keystroke.
func TestNoLineEverExceedsTheWidth(t *testing.T) {
	long := append(real(), config.Destination{
		Name: "une-destination-au-nom-vraiment-tres-long",
		Desc: strings.Repeat("description interminable ", 8),
		Forward: []config.Forward{
			{Local: 65535, To: "127.0.0.1:1", Label: "Un label lui aussi très long"},
		},
	})
	for width := 20; width <= 200; width++ {
		for _, filter := range []string{"", "dupl", "zzz"} {
			p := Picker{All: long, Filter: filter}
			for _, l := range strings.Split(plain(p.Frame(width, 24, false)), "\r\n") {
				if n := ui.Runes(l); n > width {
					t.Fatalf("largeur %d, filtre %q : ligne de %d colonnes : %q", width, filter, n, l)
				}
			}
		}
	}
}

// Colour must never change the geometry: the escape codes are invisible, so
// the same frame with and without them has to lay out identically.
func TestColourDoesNotChangeTheLayout(t *testing.T) {
	p := Picker{All: real(), Filter: "dupl", Cursor: 1}
	if got, want := plain(p.Frame(80, 24, true)), plain(p.Frame(80, 24, false)); got != want {
		t.Fatalf("la couleur déplace des colonnes.\n--- avec ---\n%s\n--- sans ---\n%s", got, want)
	}
}

func TestNoColourMeansNoEscapeCodes(t *testing.T) {
	frame := Picker{All: real(), Filter: "dupl"}.Frame(80, 24, false)
	if ansi.MatchString(frame) {
		t.Fatalf("une séquence ANSI a survécu à color=false : %q", frame)
	}
}

// Lines is what the caller winds back, so it has to count the frame that was
// actually written — off by one and every keystroke shifts the screen.
func TestLinesCountsTheFrame(t *testing.T) {
	for _, p := range []Picker{
		{All: real()},
		{All: real(), Filter: "zzz"},
		{All: nil},
	} {
		frame := p.Frame(80, 24, false)
		if got, want := Lines(frame), strings.Count(frame, "\r\n"); got != want {
			t.Fatalf("Lines=%d, lignes réelles=%d", got, want)
		}
		if !strings.HasSuffix(frame, "\r\n") {
			t.Fatal("une frame doit finir par une fin de ligne, sinon la dernière n'est pas effacée")
		}
	}
}

// The columns give way in a fixed order: labels first, then the ports
// entirely. The description is what tells two duplicati apart, so it is the
// last thing squeezed.
func TestColumnsGiveWayInOrder(t *testing.T) {
	cases := []struct {
		largeur       int
		attenduDedans string
		attenduDehors string
	}{
		{120, "Cockpit 9090", ""},
		{70, "9090  9120", "Cockpit 9090"},
		{40, "", "9090"},
	}
	for _, c := range cases {
		frame := plain(Picker{All: real()}.Frame(c.largeur, 24, false))
		if c.attenduDedans != "" && !strings.Contains(frame, c.attenduDedans) {
			t.Errorf("largeur %d : %q attendu dans le rendu :\n%s", c.largeur, c.attenduDedans, frame)
		}
		if c.attenduDehors != "" && strings.Contains(frame, c.attenduDehors) {
			t.Errorf("largeur %d : %q ne devait plus être là :\n%s", c.largeur, c.attenduDehors, frame)
		}
	}
}

// A truncated cell has to say so, otherwise a description reads as complete
// when it is not.
func TestTruncationIsVisible(t *testing.T) {
	frame := plain(Picker{All: real()}.Frame(50, 24, false))
	if !strings.Contains(frame, "…") {
		t.Fatalf("à 50 colonnes une description doit être coupée et le montrer :\n%s", frame)
	}
}

// Accented text is where a byte-counting layout falls apart: "é" is one
// column and two bytes, so padding by len() drifts a column per accent.
func TestAccentsDoNotShiftColumns(t *testing.T) {
	dests := []config.Destination{
		{Name: "aaaaaaaaaa", Desc: "sans accent", Forward: []config.Forward{{Local: 1, To: "h:1"}}},
		{Name: "ééééééééée", Desc: "avec accents", Forward: []config.Forward{{Local: 2, To: "h:2"}}},
	}
	got := rows(Picker{All: dests}.Frame(60, 24, false))
	if len(got) != 2 {
		t.Fatalf("attendu 2 lignes, obtenu %d : %q", len(got), got)
	}
	if a, b := strings.Index(got[0], "sans"), strings.Index(got[1], "avec"); ui.Runes(got[0][:a]) != ui.Runes(got[1][:b]) {
		t.Fatalf("les descriptions ne commencent pas à la même colonne :\n%q\n%q", got[0], got[1])
	}
}

// Highlighting is decoration: it must not move anything.
func TestHighlightKeepsTheVisibleWidth(t *testing.T) {
	for _, s := range []string{"Duplicati du gateway", "aucune correspondance", ""} {
		for _, f := range []string{"dupl", "GATEWAY", "z"} {
			got := plain(highlight(s, f, ui.Theme(true)))
			if got != s {
				t.Fatalf("highlight(%q, %q) altère le texte : %q", s, f, got)
			}
		}
	}
}

func TestWindowKeepsTheCursorVisible(t *testing.T) {
	cases := []struct{ cursor, total, rows, first, shown int }{
		{0, 3, 10, 0, 3},   // everything fits
		{0, 20, 5, 0, 5},   // top of a long list
		{10, 20, 5, 8, 5},  // middle: cursor centred
		{19, 20, 5, 15, 5}, // bottom: no scrolling past the end
	}
	for _, c := range cases {
		first, shown := window(c.cursor, c.total, c.rows)
		if first != c.first || shown != c.shown {
			t.Errorf("window(%d,%d,%d) = (%d,%d), attendu (%d,%d)",
				c.cursor, c.total, c.rows, first, shown, c.first, c.shown)
		}
		if c.cursor < first || c.cursor >= first+shown {
			t.Errorf("window(%d,%d,%d) laisse le curseur hors de la fenêtre", c.cursor, c.total, c.rows)
		}
	}
}

// A list taller than the terminal has to say what it is not showing, or the
// count in the header is the only clue that anything is missing.
func TestAShortTerminalSaysWhatIsHidden(t *testing.T) {
	var many []config.Destination
	for i := range 20 {
		many = append(many, config.Destination{
			Name:    strings.Repeat("d", i%5+3),
			Forward: []config.Forward{{Local: 8000 + i, To: "h:1"}},
		})
	}
	frame := plain(Picker{All: many}.Frame(80, 12, false))
	if !strings.Contains(frame, "de plus") {
		t.Fatalf("les lignes cachées doivent être annoncées :\n%s", frame)
	}
	if n := strings.Count(frame, "\r\n"); n > 12 {
		t.Fatalf("la frame déborde de l'écran : %d lignes pour 12 de haut", n)
	}
}
