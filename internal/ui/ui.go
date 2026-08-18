// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

// Package ui holds the few primitives every piece of tunny's output shares:
// the escape codes, the rule for when to emit them at all, and padding that
// counts columns rather than bytes.
//
// It exists because two callers need the same answers — the picker draws a
// table, and the command prints a banner and a reconnection notice — and a
// second copy of "is colour allowed here" is a second copy that will disagree.
package ui

import (
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/term"
)

// The basic SGR set, nothing more. Every terminal made since the eighties has
// these; a 256-colour palette would only add ways to be unreadable on
// somebody else's background.
const (
	Reset       = "\x1b[0m"
	Dim         = "\x1b[2m"
	Bold        = "\x1b[1m"
	Accent      = "\x1b[36m"
	Warn        = "\x1b[33m"
	Ok          = "\x1b[32m"
	Err         = "\x1b[31m"
	Underline   = "\x1b[4m"
	NoUnderline = "\x1b[24m"
)

// Theme turns colour on or off for everything at once.
type Theme bool

// On reports whether to emit escape codes at all.
func (t Theme) On() bool { return bool(t) }

// Wrap styles s, or hands it back untouched when colour is off. An empty
// string stays empty: styling nothing only emits codes that reset nothing.
func (t Theme) Wrap(code, s string) string {
	if !t.On() || s == "" {
		return s
	}
	return code + s + Reset
}

// ColorOK reports whether to emit escape codes to f: honour NO_COLOR, honour
// a terminal that says it cannot, never colour a pipe — and let a caller that
// knows better say so.
//
// https://no-color.org — the presence of the variable is the signal, whatever
// its value, which is why this looks for the key rather than for "1". It wins
// over CLICOLOR_FORCE: between two people asking for opposite things, the one
// asking for less is the one to obey.
//
// CLICOLOR_FORCE is what lets colour survive a pipe, which is how the README's
// screenshot is made from the real output rather than redrawn by hand.
func ColorOK(f *os.File) bool {
	if _, set := os.LookupEnv("NO_COLOR"); set {
		return false
	}
	if v := os.Getenv("CLICOLOR_FORCE"); v != "" && v != "0" {
		return true
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// zwj joins two emoji into one glyph: 👨 + ZWJ + 💻 draws as one, in the
// columns of one.
const zwj = 0x200D

// wide is East Asian Wide and Fullwidth, by block rather than by codepoint.
//
// The exact table is Unicode data that would need re-generating with every
// release, and this is a terminal layout, not a text renderer. Rounding to
// whole blocks makes a handful of unassigned codepoints count as two columns
// when they draw as one — and that is the safe direction: a row measured
// wider than it draws comes out a column short, while one measured narrower
// wraps, and a wrapped row makes the frame taller than Lines counts it,
// which leaves the wind-back erasing the wrong lines on every keystroke.
//
// ponytail: the ceiling is the emoji below U+1F000 — ✅, ⚡, ⭐ — which have
// no width anyone agrees on: they are Ambiguous in Unicode's own table and
// real terminals differ. They count as one column here. Add the ranges if a
// description ever starts with one.
var wide = &unicode.RangeTable{
	R16: []unicode.Range16{
		{Lo: 0x1100, Hi: 0x115F, Stride: 1}, // Hangul Jamo
		{Lo: 0x2E80, Hi: 0x303E, Stride: 1}, // CJK radicals through CJK symbols
		{Lo: 0x3041, Hi: 0x33FF, Stride: 1}, // kana, Hangul compatibility
		{Lo: 0x3400, Hi: 0x4DBF, Stride: 1}, // CJK extension A
		{Lo: 0x4E00, Hi: 0x9FFF, Stride: 1}, // CJK unified ideographs
		{Lo: 0xA000, Hi: 0xA4CF, Stride: 1}, // Yi
		{Lo: 0xA960, Hi: 0xA97F, Stride: 1}, // Hangul Jamo extended-A
		{Lo: 0xAC00, Hi: 0xD7A3, Stride: 1}, // Hangul syllables
		{Lo: 0xF900, Hi: 0xFAFF, Stride: 1}, // CJK compatibility ideographs
		{Lo: 0xFE10, Hi: 0xFE19, Stride: 1}, // vertical forms
		{Lo: 0xFE30, Hi: 0xFE6F, Stride: 1}, // CJK compatibility forms
		{Lo: 0xFF00, Hi: 0xFF60, Stride: 1}, // fullwidth forms
		{Lo: 0xFFE0, Hi: 0xFFE6, Stride: 1}, // fullwidth signs
	},
	R32: []unicode.Range32{
		{Lo: 0x16FE0, Hi: 0x1B2FB, Stride: 1}, // Tangut, Nushu, kana supplement
		{Lo: 0x1F200, Hi: 0x1F64F, Stride: 1}, // pictographs and emoticons
		{Lo: 0x1F680, Hi: 0x1F6FF, Stride: 1}, // transport and map
		{Lo: 0x1F900, Hi: 0x1F9FF, Stride: 1}, // supplemental symbols
		{Lo: 0x1FA70, Hi: 0x1FAFF, Stride: 1}, // symbols extended-A
		{Lo: 0x20000, Hi: 0x3FFFD, Stride: 1}, // CJK extension B onwards
	},
}

// runeWidth is how many columns one rune takes on its own, before anything
// it might be joined to.
func runeWidth(r rune) int {
	switch {
	case r < 0x20 || (r >= 0x7F && r < 0xA0):
		return 0 // a control character is consumed, not drawn
	case unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf):
		// Combining marks sit on the letter before them, and the format
		// characters — variation selectors, the joiner, the soft hyphen —
		// are instructions rather than glyphs. "é" written as e + U+0301 is
		// one column, the same as the single codepoint.
		return 0
	case unicode.Is(wide, r):
		return 2
	default:
		return 1
	}
}

// Width counts terminal columns, which is neither bytes nor runes. "é" is two
// bytes and one column; "世" is one rune and two. A layout measured in either
// of the other two drifts, and a row that drifts past the right edge wraps.
func Width(s string) int {
	w, join := 0, false
	for _, r := range s {
		cw := runeWidth(r)
		if join && cw > 0 {
			cw = 0 // this one joins the glyph before it rather than adding to it
		}
		join = r == zwj
		w += cw
	}
	return w
}

// Fit pads or truncates to w columns, marking a truncation with an ellipsis.
//
// Truncating matters beyond looks: in the picker, a row wider than the
// terminal wraps, which makes the frame taller than it is counted to be and
// leaves the wind-back tearing the screen apart on every keystroke.
func Fit(s string, w int) string {
	switch n := Width(s); {
	case w <= 0:
		return ""
	case n == w:
		return s
	case n < w:
		return s + strings.Repeat(" ", w-n)
	case w == 1:
		return "…"
	}
	// Take runes while they fit beside the ellipsis. A double-width character
	// straddling the edge is dropped whole and its column made up with a
	// space: half a character is a column the terminal fills however it likes.
	//
	// ponytail: a cut landing inside a joined sequence leaves the join
	// dangling, which draws as the two halves rather than the one glyph. It
	// is a truncation either way.
	cut, used, join := 0, 0, false
	for i, r := range s {
		cw := runeWidth(r)
		if join && cw > 0 {
			cw = 0
		}
		join = r == zwj
		if used+cw > w-1 {
			break
		}
		used, cut = used+cw, i+utf8.RuneLen(r)
	}
	return s[:cut] + "…" + strings.Repeat(" ", w-1-used)
}

// FitTail is Fit for a field somebody is typing into: it keeps the last w
// columns and marks the cut with a leading ellipsis, because the end of the
// line is where the cursor is and what they are looking at.
func FitTail(s string, w int) string {
	switch n := Width(s); {
	case w <= 0:
		return ""
	case n <= w:
		return s + strings.Repeat(" ", w-n)
	case w == 1:
		return "…"
	}
	rs := []rune(s)
	used, i := 0, len(rs)
	for i > 0 {
		cw := runeWidth(rs[i-1])
		if used+cw > w-1 {
			break
		}
		used, i = used+cw, i-1
	}
	return "…" + string(rs[i:]) + strings.Repeat(" ", w-1-used)
}

// FitRight is Fit for a column of numbers, which read as a column only when
// their last digits line up.
func FitRight(s string, w int) string {
	if n := Width(s); n < w {
		return strings.Repeat(" ", w-n) + s
	}
	return Fit(s, w)
}
