package ui

import (
	"os"
	"strings"
	"testing"
)

func TestFitAndFitRight(t *testing.T) {
	cases := []struct {
		in         string
		w          int
		fit, right string
	}{
		{"abc", 5, "abc  ", "  abc"},
		{"abc", 3, "abc", "abc"},
		{"abcdef", 4, "abc…", "abc…"},
		{"abc", 1, "…", "…"},
		{"abc", 0, "", ""},
		{"abc", -1, "", ""},
		{"", 3, "   ", "   "},
		// Accents are one column each: pad by bytes and this drifts by four.
		{"éééé", 6, "éééé  ", "  éééé"},
		{"ééééé", 3, "éé…", "éé…"},
	}
	for _, c := range cases {
		if got := Fit(c.in, c.w); got != c.fit {
			t.Errorf("Fit(%q,%d) = %q, attendu %q", c.in, c.w, got, c.fit)
		}
		if got := FitRight(c.in, c.w); got != c.right {
			t.Errorf("FitRight(%q,%d) = %q, attendu %q", c.in, c.w, got, c.right)
		}
	}
}

// Fit is what keeps a line inside the terminal, so the property that matters
// is the one about columns, not about any particular string.
func TestFitAlwaysYieldsExactlyWColumns(t *testing.T) {
	for _, s := range []string{"", "a", "abc", "éàü", strings.Repeat("long ", 20)} {
		for w := 1; w <= 30; w++ {
			if got := Runes(Fit(s, w)); got != w {
				t.Fatalf("Fit(%q,%d) fait %d colonnes", s, w, got)
			}
		}
	}
}

func TestWrapIsANoOpWithoutColour(t *testing.T) {
	if got := Theme(false).Wrap(Bold, "x"); got != "x" {
		t.Errorf("sans couleur, Wrap doit rendre le texte nu, obtenu %q", got)
	}
	if got := Theme(true).Wrap(Bold, "x"); got != Bold+"x"+Reset {
		t.Errorf("avec couleur, Wrap doit encadrer, obtenu %q", got)
	}
	// Styling nothing would emit codes that reset nothing, which is only a
	// way to make an empty cell non-empty.
	if got := Theme(true).Wrap(Bold, ""); got != "" {
		t.Errorf("une chaîne vide doit rester vide, obtenu %q", got)
	}
}

// The rule has three ways to say no and only one to say yes, and the one that
// is easiest to get wrong is NO_COLOR: the variable counts by being set, not
// by being set to something in particular.
func TestColorOKSaysNo(t *testing.T) {
	// A pipe is never a terminal, so this is the "yes" branch's own answer.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	// Set before unsetting: t.Setenv is what registers the cleanup that puts
	// the developer's own environment back.
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm")
	unset(t, "NO_COLOR")
	if ColorOK(w) {
		t.Error("un tube n'est pas un terminal")
	}

	for _, v := range []string{"", "0", "1", "false"} {
		t.Setenv("NO_COLOR", v)
		if ColorOK(w) {
			t.Errorf("NO_COLOR=%q doit couper la couleur", v)
		}
	}

	unset(t, "NO_COLOR")
	t.Setenv("TERM", "dumb")
	if ColorOK(w) {
		t.Error("TERM=dumb doit couper la couleur")
	}
}

func unset(t *testing.T, key string) {
	t.Helper()
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
}
