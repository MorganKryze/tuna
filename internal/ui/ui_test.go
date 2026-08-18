// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

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
			t.Errorf("Fit(%q,%d) = %q, want %q", c.in, c.w, got, c.fit)
		}
		if got := FitRight(c.in, c.w); got != c.right {
			t.Errorf("FitRight(%q,%d) = %q, want %q", c.in, c.w, got, c.right)
		}
	}
}

// Fit is what keeps a line inside the terminal, so the property that matters
// is the one about columns, not about any particular string.
func TestFitAlwaysYieldsExactlyWColumns(t *testing.T) {
	for _, s := range []string{
		"", "a", "abc", "éàü", strings.Repeat("long ", 20),
		// Double-width, so the padding has to be counted rather than
		// assumed, and a cut can land on a character that will not fit.
		"世界", "a世b界c", "🚀 deploy", "e\u0301 combining",
	} {
		for w := 1; w <= 30; w++ {
			if got := Width(Fit(s, w)); got != w {
				t.Fatalf("Fit(%q,%d) is %d columns wide", s, w, got)
			}
		}
	}
}

func TestWrapIsANoOpWithoutColour(t *testing.T) {
	if got := Theme(false).Wrap(Bold, "x"); got != "x" {
		t.Errorf("without colour, Wrap has to return bare text, got %q", got)
	}
	if got := Theme(true).Wrap(Bold, "x"); got != Bold+"x"+Reset {
		t.Errorf("with colour, Wrap has to bracket the text, got %q", got)
	}
	// Styling nothing would emit codes that reset nothing, which is only a
	// way to make an empty cell non-empty.
	if got := Theme(true).Wrap(Bold, ""); got != "" {
		t.Errorf("an empty string has to stay empty, got %q", got)
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
		t.Error("a pipe is not a terminal")
	}

	for _, v := range []string{"", "0", "1", "false"} {
		t.Setenv("NO_COLOR", v)
		if ColorOK(w) {
			t.Errorf("NO_COLOR=%q has to switch colour off", v)
		}
	}

	unset(t, "NO_COLOR")
	t.Setenv("TERM", "dumb")
	if ColorOK(w) {
		t.Error("TERM=dumb has to switch colour off")
	}

	// CLICOLOR_FORCE overrides the pipe and the dumb terminal, and NO_COLOR
	// overrides it in turn: between two people asking for opposite things,
	// the one asking for less wins.
	t.Setenv("CLICOLOR_FORCE", "1")
	if !ColorOK(w) {
		t.Error("CLICOLOR_FORCE has to force colour through a pipe")
	}
	t.Setenv("CLICOLOR_FORCE", "0")
	if ColorOK(w) {
		t.Error(`CLICOLOR_FORCE="0" forces nothing`)
	}
	t.Setenv("CLICOLOR_FORCE", "1")
	t.Setenv("NO_COLOR", "")
	if ColorOK(w) {
		t.Error("NO_COLOR has to win over CLICOLOR_FORCE")
	}
}

func unset(t *testing.T, key string) {
	t.Helper()
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
}

// HideControlEcho is called on whatever stdin happens to be, including a pipe
// or a file. The branch that matters here is the one where there is no
// terminal to change: it has to hand back a restore function anyway, because
// every call site defers it without looking.
func TestHideControlEchoSurvivesTheAbsenceOfATerminal(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	restore := HideControlEcho(r)
	if restore == nil {
		t.Fatal("a nil restore would panic a defer")
	}
	restore()
	restore() // and twice, because a defer can outlive an early return
}

// Width is the measurement the whole layout is built on: every column budget
// in render.go is a subtraction from it. Byte length and rune count are both
// wrong, and they are wrong in opposite directions.
func TestWidthCountsColumns(t *testing.T) {
	for _, c := range []struct {
		in   string
		want int
	}{
		{"", 0},
		{"abc", 3},
		// Two bytes, one column. Measuring in bytes drifts right by one per
		// accent, which is what padding with len() used to do.
		{"éàü", 3},
		// One rune, two columns. Measuring in runes drifts left by one per
		// ideograph, and a row that drifts left wraps.
		{"世界", 4},
		{"\uFF21", 2},  // fullwidth Latin capital A
		{"　", 2},       // ideographic space
		{"e\u0301", 1}, // e plus a combining acute: one glyph, one column
		{"🚀", 2},
		{"👨\u200d💻", 2}, // joined into one glyph, so one glyph's worth of columns
		{"a\tb", 2},     // a control character is consumed by the terminal
	} {
		if got := Width(c.in); got != c.want {
			t.Errorf("Width(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

// FitTail is the filter line: what has just been typed is what someone is
// looking at, so the end is the half that survives.
func TestFitTailKeepsTheEnd(t *testing.T) {
	for _, c := range []struct {
		in   string
		w    int
		want string
	}{
		{"abc", 5, "abc  "},
		{"abcdef", 4, "…def"},
		{"abc", 1, "…"},
		{"abc", 0, ""},
		// The ideograph will not fit beside the ellipsis, so it goes whole
		// and its column becomes a space: half a character is a column the
		// terminal fills however it likes.
		{"a世界", 4, "…界 "},
		{"a世界", 3, "…界"},
		{"a世界", 2, "… "},
	} {
		if got := FitTail(c.in, c.w); got != c.want {
			t.Errorf("FitTail(%q,%d) = %q, want %q", c.in, c.w, got, c.want)
		}
		if got := Width(FitTail(c.in, c.w)); c.w > 0 && got != c.w {
			t.Errorf("FitTail(%q,%d) is %d columns wide", c.in, c.w, got)
		}
	}
}
