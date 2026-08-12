// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

package pick

import (
	"regexp"
	"strings"
	"testing"

	"github.com/MorganKryze/tunny/internal/config"
	"github.com/MorganKryze/tunny/internal/ui"
)

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plain(s string) string { return ansi.ReplaceAllString(s, "") }

func real() []config.Destination {
	return []config.Destination{
		{Name: "hypervisor", Desc: "Cockpit and Komodo, the admin root", Forward: []config.Forward{
			{Local: 9090, To: "127.0.0.1:9090", Label: "Cockpit"},
			{Local: 9120, To: "10.10.10.10:9120", Label: "Komodo"},
		}},
		{Name: "gateway", Desc: "Duplicati on the gateway", Forward: []config.Forward{
			{Local: 8200, To: "127.0.0.1:8200", Label: "Duplicati"},
		}},
		{Name: "control-plane", Desc: "Duplicati on the control plane, Komodo's Mongo", Forward: []config.Forward{
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
		"  ❯ ▏type to filter                                                          3/3\r\n" +
		"\r\n" +
		"  ▌ hypervisor      Cockpit and Komodo, the admin root                9090  9120\r\n" +
		"    gateway         Duplicati on the gateway                                8200\r\n" +
		"    control-plane   Duplicati on the control plane, Komodo's Mongo          8201\r\n" +
		"\r\n" +
		"    ↑↓ move    ⏎ open    ⎋ cancel\r\n"
	if got != want {
		t.Fatalf("unexpected frame.\n--- got ---\n%s\n--- want ---\n%s", got, want)
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
			{Local: 65535, To: "127.0.0.1:1", Label: "A label that is also rather long"},
		},
	})
	for width := 1; width <= 200; width++ {
		for _, filter := range []string{"", "dupl", "zzz"} {
			p := Picker{All: long, Filter: filter}
			for _, l := range strings.Split(plain(p.Frame(width, 24, false)), "\r\n") {
				if n := ui.Runes(l); n > width {
					t.Fatalf("width %d, filter %q: a line %d columns wide: %q", width, filter, n, l)
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
		t.Fatalf("colour moves columns.\n--- with ---\n%s\n--- without ---\n%s", got, want)
	}
}

func TestNoColourMeansNoEscapeCodes(t *testing.T) {
	frame := Picker{All: real(), Filter: "dupl"}.Frame(80, 24, false)
	if ansi.MatchString(frame) {
		t.Fatalf("an ANSI sequence survived color=false: %q", frame)
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
			t.Fatalf("Lines=%d, real lines=%d", got, want)
		}
		if !strings.HasSuffix(frame, "\r\n") {
			t.Fatal("a frame has to end on a line ending, or the last one never gets erased")
		}
	}
}

// The columns give way in a fixed order: labels first, then the ports
// entirely. The description is what tells two duplicati apart, so it is the
// last thing squeezed.
func TestColumnsGiveWayInOrder(t *testing.T) {
	cases := []struct {
		width   int
		wantIn  string
		wantOut string
	}{
		{120, "Cockpit 9090", ""},
		{70, "9090  9120", "Cockpit 9090"},
		{40, "", "9090"},
	}
	for _, c := range cases {
		frame := plain(Picker{All: real()}.Frame(c.width, 24, false))
		if c.wantIn != "" && !strings.Contains(frame, c.wantIn) {
			t.Errorf("width %d: want %q in the frame:\n%s", c.width, c.wantIn, frame)
		}
		if c.wantOut != "" && strings.Contains(frame, c.wantOut) {
			t.Errorf("width %d: %q should be gone:\n%s", c.width, c.wantOut, frame)
		}
	}
}

// A truncated cell has to say so, otherwise a description reads as complete
// when it is not.
func TestTruncationIsVisible(t *testing.T) {
	frame := plain(Picker{All: real()}.Frame(50, 24, false))
	if !strings.Contains(frame, "…") {
		t.Fatalf("at 50 columns a description has to be cut, and say so:\n%s", frame)
	}
}

// Accented text is where a byte-counting layout falls apart: "é" is one
// column and two bytes, so padding by len() drifts a column per accent.
func TestAccentsDoNotShiftColumns(t *testing.T) {
	dests := []config.Destination{
		{Name: "aaaaaaaaaa", Desc: "no accent", Forward: []config.Forward{{Local: 1, To: "h:1"}}},
		{Name: "ééééééééée", Desc: "with accents", Forward: []config.Forward{{Local: 2, To: "h:2"}}},
	}
	got := rows(Picker{All: dests}.Frame(60, 24, false))
	if len(got) != 2 {
		t.Fatalf("want 2 rows, got %d: %q", len(got), got)
	}
	if a, b := strings.Index(got[0], "no accent"), strings.Index(got[1], "with accents"); ui.Runes(got[0][:a]) != ui.Runes(got[1][:b]) {
		t.Fatalf("the descriptions do not start at the same column:\n%q\n%q", got[0], got[1])
	}
}

// Highlighting is decoration: it must not move anything.
func TestHighlightKeepsTheVisibleWidth(t *testing.T) {
	for _, s := range []string{"Duplicati du gateway", "aucune correspondance", ""} {
		for _, f := range []string{"dupl", "GATEWAY", "z"} {
			got := plain(highlight(s, f, ui.Theme(true)))
			if got != s {
				t.Fatalf("highlight(%q, %q) alters the text: %q", s, f, got)
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
			t.Errorf("window(%d,%d,%d) = (%d,%d), want (%d,%d)",
				c.cursor, c.total, c.rows, first, shown, c.first, c.shown)
		}
		if c.cursor < first || c.cursor >= first+shown {
			t.Errorf("window(%d,%d,%d) leaves the cursor outside the window", c.cursor, c.total, c.rows)
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
	if !strings.Contains(frame, "more") {
		t.Fatalf("hidden rows have to be announced:\n%s", frame)
	}
	if n := strings.Count(frame, "\r\n"); n > 12 {
		t.Fatalf("the frame overflows the screen: %d lines for a height of 12", n)
	}
}

// A destination whose local port is already taken is a destination that
// cannot open. Saying so in the list is the whole point: the alternative is
// finding out from ssh, after a banner has promised a tunnel.
func TestBusyPortsAreAnnouncedInTheList(t *testing.T) {
	p := Picker{All: real(), Busy: map[string][]int{"control-plane": {8201}}}
	frame := plain(p.Frame(80, 24, false))

	if !strings.Contains(frame, "● 8201 in use") {
		t.Fatalf("a taken port has to be announced:\n%s", frame)
	}
	// Only the destination concerned: the others still advertise their ports.
	if !strings.Contains(frame, "8200") {
		t.Errorf("free destinations keep their ports:\n%s", frame)
	}
	if strings.Count(frame, "in use") != 1 {
		t.Errorf("only one row should be marked:\n%s", frame)
	}
}

// The warning is longer than the port numbers it replaces, so the column has
// to be sized with it — otherwise it is the thing that overflows the row.
func TestABusyColumnStillFits(t *testing.T) {
	busy := map[string][]int{"hyperviseur": {9090, 9120}, "gateway": {8200}, "control-plane": {8201}}
	for width := 1; width <= 200; width++ {
		p := Picker{All: real(), Busy: busy}
		for _, l := range strings.Split(plain(p.Frame(width, 24, false)), "\r\n") {
			if n := ui.Runes(l); n > width {
				t.Fatalf("width %d: a line %d columns wide: %q", width, n, l)
			}
		}
	}
}

// Widths 1 to 5 used to panic: the prompt's column budget went negative and
// the slice that keeps the tail of a long filter ran off the end, for an empty
// filter as much as a typed one. `size()` passes on whatever the terminal
// reports with no floor, so a four-column pane reached it.
func TestNarrowTerminalsSaySoInsteadOfDrawingOrPanicking(t *testing.T) {
	for width := 1; width < 20; width++ {
		for _, filter := range []string{"", "d", "duplicati-and-then-some"} {
			frame := plain(Picker{All: real(), Filter: filter}.Frame(width, 24, false))
			if !strings.Contains(frame, "too narrow") && !strings.Contains(frame, "…") {
				t.Errorf("width %d, filter %q: want a notice, got %q", width, filter, frame)
			}
			for _, l := range strings.Split(frame, "\r\n") {
				if n := ui.Runes(l); n > width {
					t.Fatalf("width %d: a line %d columns wide: %q", width, n, l)
				}
			}
		}
	}
	// And at the first usable width the list is back.
	if frame := plain(Picker{All: real()}.Frame(20, 24, false)); strings.Contains(frame, "too narrow") {
		t.Errorf("width 20 has to draw the list:\n%s", frame)
	}
}

// The only test of highlight asserted that it does not change the text, which
// `return s` satisfies. This one asserts it does something, and does it in the
// right place.
func TestHighlightMarksTheMatchAndNothingElse(t *testing.T) {
	got := highlight("Duplicati on the gateway", "gate", ui.Theme(true))
	if !strings.Contains(got, ui.Underline+"gate"+ui.NoUnderline) {
		t.Fatalf("the match has to be underlined, got %q", got)
	}
	if strings.Count(got, ui.Underline) != 1 {
		t.Errorf("only the match, and only once: %q", got)
	}
	// Case folded on the needle, and the text keeps the case it was written in.
	if got := highlight("Duplicati", "DUPL", ui.Theme(true)); !strings.Contains(got, ui.Underline+"Dupl") {
		t.Errorf("matching ignores case, the text does not: %q", got)
	}
	// Colour off means no codes at all, whatever matched.
	if got := highlight("Duplicati", "dupl", ui.Theme(false)); got != "Duplicati" {
		t.Errorf("without colour there is nothing to mark, got %q", got)
	}
}
