// Package ui holds the few primitives every piece of tuna's output shares:
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
	Underline   = "\x1b[4m"
	NoUnderline = "\x1b[24m"
)

// Theme turns colour on or off for everything at once.
type Theme bool

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
// a terminal that says it cannot, and never colour a pipe.
//
// https://no-color.org — the presence of the variable is the signal, whatever
// its value, which is why this looks for the key rather than for "1".
func ColorOK(f *os.File) bool {
	if _, set := os.LookupEnv("NO_COLOR"); set {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// Runes counts columns, not bytes. "é" is one column and two bytes, so a
// layout that pads with len() drifts by one column per accent.
func Runes(s string) int { return utf8.RuneCountInString(s) }

// Fit pads or truncates to w columns, marking a truncation with an ellipsis.
//
// Truncating matters beyond looks: in the picker, a row wider than the
// terminal wraps, which makes the frame taller than it is counted to be and
// leaves the wind-back tearing the screen apart on every keystroke.
func Fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	switch n := Runes(s); {
	case n == w:
		return s
	case n < w:
		return s + strings.Repeat(" ", w-n)
	case w == 1:
		return "…"
	default:
		return string([]rune(s)[:w-1]) + "…"
	}
}

// FitRight is Fit for a column of numbers, which read as a column only when
// their last digits line up.
func FitRight(s string, w int) string {
	if n := Runes(s); n < w {
		return strings.Repeat(" ", w-n) + s
	}
	return Fit(s, w)
}
