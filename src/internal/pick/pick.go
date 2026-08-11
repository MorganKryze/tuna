// Package pick is the destination picker. It receives a list already ordered
// by recency and returns a name: it never sorts, and it never reads the state
// file.
//
// The split inside the package is what makes it testable. pick.go is a pure
// function of state and keystroke, keys.go turns bytes into keystrokes, and
// term.go is the only file that touches a terminal.
package pick

import (
	"strings"

	"github.com/MorganKryze/tuna/src/internal/config"
)

type Key int

const (
	KeyNone Key = iota
	KeyUp
	KeyDown
	KeyEnter
	KeyEsc
	KeyBackspace
	KeyRune
)

// Picker is the whole state of the list: what it shows, what has been typed,
// and where the cursor is. Update is a pure function of it, which is why the
// navigation can be tested without a terminal.
type Picker struct {
	All    []config.Destination // already ordered by recency
	Filter string
	Cursor int
	// Busy maps a destination name to the local ports already taken, probed
	// once before the picker opened. A missing key means nothing is in the
	// way. It is informational only: the authoritative check happens again
	// at launch, because a port can be taken while the list is on screen.
	Busy map[string][]int
}

// Matches is a plain substring match on name and description, case folded.
// Not fuzzy: with under a dozen destinations, fuzzy matching only adds ways
// to be surprised by the ranking.
func (p Picker) Matches() []config.Destination {
	if p.Filter == "" {
		return p.All
	}
	needle := strings.ToLower(p.Filter)
	var out []config.Destination
	for _, d := range p.All {
		hay := strings.ToLower(d.Name + " " + d.Desc)
		if strings.Contains(hay, needle) {
			out = append(out, d)
		}
	}
	return out
}

// Update applies one keystroke. It returns the next state, the chosen name
// (empty if none) and whether the picker is finished.
func (p Picker) Update(k Key, r rune) (Picker, string, bool) {
	switch k {
	case KeyEsc:
		return p, "", true

	case KeyEnter:
		matches := p.Matches()
		// Enter on an empty list selects nothing rather than indexing into
		// it. Typing a filter that matches nothing is easy to do by accident.
		if len(matches) == 0 {
			return p, "", false
		}
		return p, matches[p.Cursor].Name, true

	case KeyUp:
		if p.Cursor > 0 {
			p.Cursor--
		}

	case KeyDown:
		if p.Cursor < len(p.Matches())-1 {
			p.Cursor++
		}

	case KeyBackspace:
		// Byte-wise, which is safe because readKey only ever emits ASCII:
		// a filter is typed against destination names, and those are ASCII
		// by convention. Accept a multi-byte rune here and this line starts
		// cutting UTF-8 in half.
		if p.Filter != "" {
			p.Filter = p.Filter[:len(p.Filter)-1]
		}

	case KeyRune:
		p.Filter += string(r)
	}

	// Editing the filter changes how many rows exist under the cursor, so it
	// is clamped after every keystroke rather than in each branch.
	if n := len(p.Matches()); p.Cursor >= n {
		p.Cursor = max(n-1, 0)
	}
	return p, "", false
}
