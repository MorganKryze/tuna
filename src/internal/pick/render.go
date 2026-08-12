package pick

import (
	"fmt"
	"strings"

	"github.com/MorganKryze/tuna/src/internal/config"
	"github.com/MorganKryze/tuna/src/internal/ui"
)

const (
	gutter   = 4 // two of indent, then "▌ " or "  "
	gap      = 3 // between columns; three reads as a column, two as a typo
	minNameW = 8
	// The narrowest width that can hold the chrome without wrapping.
	minWidth = 20
	maxNameW = 22
	// Below this the ports column gives up its labels, and below it again
	// the whole column goes. The description is what tells two duplicati
	// apart, so it is the last thing to be squeezed.
	roomyDescW = 40
	minDescW   = 12
	// Blank, prompt, blank, rows…, blank, hint.
	chromeLines = 5
)

// Frame renders the whole picker as one string.
//
// It is pure: width, height and colour arrive as arguments rather than being
// read from a terminal, which is what lets the layout be tested at any size
// and looked at with `tuna --preview`. term.go is the only caller that knows
// where those numbers come from.
func (p Picker) Frame(width, height int, color bool) string {
	if width <= 0 {
		width = 80
	}
	t := ui.Theme(color)

	// Under this, the chrome alone overflows: the prompt needs 5 columns for
	// "  ❯ ▏" and a row needs 4 before its first letter. Drawing anyway wraps,
	// a wrapped frame is taller than Lines counts, and the wind-back then
	// erases the wrong rows on every keystroke. Say so instead, and draw the
	// list again when the window grows.
	if width < minWidth {
		return "\r\n" + ui.Fit("terminal too narrow", width) + "\r\n"
	}

	matches := p.Matches()

	rows := len(matches) // no height known: never scroll
	if height > 0 {
		rows = max(height-chromeLines, 1)
		// The "N de plus" line is itself a line. Forget to budget for it and
		// the frame is one taller than the terminal, which scrolls the whole
		// thing by one and leaves the wind-back erasing the wrong rows.
		if len(matches) > rows {
			rows = max(rows-1, 1)
		}
	}

	var b strings.Builder
	line(&b, "")
	line(&b, p.prompt(t, width, len(matches)))
	line(&b, "")

	switch {
	case len(matches) == 0:
		quoted := "\u201c" + p.Filter + "\u201d"
		if ui.Runes("    nothing matches "+quoted) <= width {
			line(&b, t.Wrap(ui.Dim, "    nothing matches ")+t.Wrap(ui.Bold, quoted))
		} else {
			line(&b, t.Wrap(ui.Dim, ui.Fit("    nothing matches", width)))
		}
	default:
		nameW, descW, portW, labels := p.columns(width)
		first, shown := window(p.Cursor, len(matches), rows)
		for i := first; i < first+shown; i++ {
			line(&b, p.row(t, matches[i], i == p.Cursor, nameW, descW, portW, labels))
		}
		if hidden := len(matches) - shown; hidden > 0 {
			line(&b, t.Wrap(ui.Dim, ui.Fit(fmt.Sprintf("    ⋯ %d more", hidden), width)))
		}
	}

	line(&b, "")
	line(&b, hint(t, width))
	return b.String()
}

// hint is the key legend, which drops its words before it drops its keys: on
// a narrow terminal the symbols alone still say what is possible, and a
// wrapped legend would break the wind-back just as surely as a wrapped row.
func hint(t ui.Theme, width int) string {
	for _, s := range []string{
		"    ↑↓ move    ⏎ open    ⎋ cancel",
		"    ↑↓ · ⏎ · ⎋",
	} {
		if ui.Runes(s) <= width {
			return t.Wrap(ui.Dim, s)
		}
	}
	return ""
}

// line writes one row of the frame. The carriage return is not decoration:
// the terminal is in raw mode, where a bare newline drops a line without
// returning to column one and every row would start further right than the
// last.
func line(b *strings.Builder, s string) {
	b.WriteString(strings.TrimRight(s, " "))
	b.WriteString("\r\n")
}

// Lines is how many terminal lines a frame occupies, which is exactly what
// the caller needs to wind it back. Counting the frame it already drew beats
// recomputing from the state: a keystroke changes how tall the *next* frame
// is, while the one to erase is the one already on screen.
func Lines(frame string) int {
	return strings.Count(frame, "\n")
}

// prompt is the filter line, with the match count pushed to the right edge.
func (p Picker) prompt(t ui.Theme, width, matched int) string {
	count := fmt.Sprintf("%d/%d", matched, len(p.All))

	// Everything gets a column budget, this line included: a prompt that
	// overflows wraps exactly like an overflowing row, and breaks the
	// wind-back the same way.
	budget := width - 2 // the two of indent; the right edge is the rows' right edge
	if room := budget - ui.Runes(count) - gap; room >= 12 {
		budget = room
	} else {
		count = ""
	}

	// The block is the only cursor there is in raw mode, so it sits right
	// after what has been typed. The ghost text trails it, the way every
	// search box anyone has used places its placeholder.
	filter, ghost := p.Filter, ""
	if filter == "" {
		ghost = "type to filter"
	}
	// 2 for "❯ ", 1 for the block. The floor at zero is what keeps a terminal
	// narrower than the chrome from slicing past the end of the filter: a
	// negative room made the comparison below true even for an empty filter,
	// and the index then ran off the slice. A 4-column tmux pane did it.
	switch room := max(budget-3, 0); {
	case ui.Runes(filter) > room:
		// Keep the tail: what was just typed is what someone is looking at.
		filter = ui.Fit("…"+string([]rune(filter)[ui.Runes(filter)-room:]), room)
		ghost = ""
	default:
		ghost = ui.Fit(ghost, min(ui.Runes(ghost), room-ui.Runes(filter)))
	}

	left := "  " + t.Wrap(ui.Accent, "❯") + " " + filter + t.Wrap(ui.Accent, "▏") + t.Wrap(ui.Dim, ghost)
	leftW := 5 + ui.Runes(filter) + ui.Runes(ghost)

	if pad := width - leftW - ui.Runes(count); count != "" && pad > 0 {
		return left + strings.Repeat(" ", pad) + t.Wrap(ui.Dim, count)
	}
	return left
}

// row is one destination: the selection bar, the name, the description, and
// the local ports it will answer on.
func (p Picker) row(t ui.Theme, d config.Destination, selected bool, nameW, descW, portW int, labels bool) string {
	bar, name, desc := "    ", ui.Fit(d.Name, nameW), ui.Fit(d.Desc, descW)
	name = highlight(name, p.Filter, t)
	desc = highlight(desc, p.Filter, t)

	if selected {
		// A solid bar rather than an arrow, and a description that stops
		// being dim: the eye should find the current row without reading.
		bar = "  " + t.Wrap(ui.Accent, "▌") + " "
		name = t.Wrap(ui.Bold, name)
	} else {
		desc = t.Wrap(ui.Dim, desc)
	}

	out := bar + name
	if descW > 0 {
		out += strings.Repeat(" ", gap) + desc
	}
	if portW > 0 {
		style := ui.Dim
		if len(p.Busy[d.Name]) > 0 {
			style = ui.Warn
		}
		out += strings.Repeat(" ", gap) + t.Wrap(style, ui.FitRight(p.ports(d, labels), portW))
	}
	return out
}

// columns sizes the row from the data and the terminal width, in three steps
// down: full labels, bare port numbers, then no ports at all. Sized from every
// destination rather than from the matches, so the layout does not shift
// sideways while somebody is typing.
func (p Picker) columns(width int) (nameW, descW, portW int, labels bool) {
	var labelled, bare int
	for _, d := range p.All {
		nameW = max(nameW, ui.Runes(d.Name))
		labelled = max(labelled, ui.Runes(p.ports(d, true)))
		bare = max(bare, ui.Runes(p.ports(d, false)))
	}
	nameW = min(max(nameW, minNameW), maxNameW)
	// The cap is not enough on its own: a very narrow terminal has less room
	// than the shortest name column, and the row would overflow before any
	// other column had a chance to give way.
	nameW = min(nameW, max(width-gutter, 1))

	room := func(w int) int { return width - gutter - nameW - gap - w - gap }

	switch {
	case room(labelled) >= roomyDescW:
		return nameW, room(labelled), labelled, true
	case room(bare) >= minDescW:
		return nameW, room(bare), bare, false
	}
	// No room for ports at all. And if the description would be down to a
	// stub, drop it as well and give the name everything: three letters and
	// an ellipsis say less than one name in full.
	if d := width - gutter - nameW - gap; d >= minDescW {
		return nameW, d, 0, false
	}
	return max(width-gutter, 1), 0, 0, false
}

// window returns which slice of the list to draw so the cursor stays visible.
func window(cursor, total, rows int) (first, shown int) {
	if total <= rows {
		return 0, total
	}
	first = min(max(cursor-rows/2, 0), total-rows)
	return first, rows
}

// highlight underlines what the filter matched, so the reason a row survived
// is visible rather than inferred.
func highlight(s, filter string, t ui.Theme) string {
	if filter == "" || !t.On() {
		return s
	}
	low, needle := strings.ToLower(s), strings.ToLower(filter)
	// ponytail: byte offsets taken from the folded copy, which holds for
	// every script whose lowercase is the same byte length — Latin, accents
	// included. Anything else goes unhighlighted rather than sliced mid-rune.
	if len(low) != len(s) {
		return s
	}
	var b strings.Builder
	for {
		i := strings.Index(low, needle)
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		b.WriteString(ui.Underline + s[i:i+len(needle)] + ui.NoUnderline)
		s, low = s[i+len(needle):], low[i+len(needle):]
	}
}

// ports is the right-hand column. With labels it says what answers where;
// without, the bare local ports, which is still the address you will type.
//
// A destination whose ports are already taken says so instead, because the
// one thing worth knowing before choosing is that choosing will fail.
func (p Picker) ports(d config.Destination, labels bool) string {
	if taken := p.Busy[d.Name]; len(taken) > 0 {
		out := make([]string, 0, len(taken))
		for _, n := range taken {
			out = append(out, fmt.Sprintf("%d", n))
		}
		return "● " + strings.Join(out, " ") + " in use"
	}
	out := make([]string, 0, len(d.Forward))
	for _, f := range d.Forward {
		if labels && f.Label != "" {
			out = append(out, fmt.Sprintf("%s %d", f.Label, f.Local))
			continue
		}
		out = append(out, fmt.Sprintf("%d", f.Local))
	}
	return strings.Join(out, "  ")
}
