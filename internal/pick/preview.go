// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

package pick

import (
	"fmt"
	"strings"

	"github.com/MorganKryze/tunny/internal/config"
	"github.com/MorganKryze/tunny/internal/ui"
)

// Preview renders the picker in the states worth looking at, without a
// terminal: what it shows on arrival, with the cursor moved, while filtering,
// and when the filter matches nothing.
//
// It exists because the drawing is the one part of tunny a test cannot judge —
// a golden test proves the columns line up, not that the thing reads well.
// `tunny --preview` puts the frames on screen so a human can say.
func Preview(dests []config.Destination, busy map[string][]int, width, height int, color bool) string {
	base := Picker{All: dests, Busy: busy}

	states := []struct {
		titre string
		p     Picker
	}{
		{"on opening", base},
	}
	if len(dests) > 1 {
		down := base
		down.Cursor = 1
		states = append(states, struct {
			titre string
			p     Picker
		}{"cursor on the second row", down})
	}
	// A filter taken from the data itself, so the preview shows a real match
	// rather than an invented one.
	if needle := commonPrefix(dests); needle != "" {
		f := base
		f.Filter = needle
		states = append(states, struct {
			titre string
			p     Picker
		}{fmt.Sprintf("filter %q", needle), f})
	}
	empty := base
	empty.Filter = "zzz"
	states = append(states, struct {
		titre string
		p     Picker
	}{"filter matching nothing", empty})

	var b strings.Builder
	for _, s := range states {
		fmt.Fprintf(&b, "\n%s\n", ui.Theme(color).Wrap(ui.Dim, "── "+s.titre+" "+strings.Repeat("─", max(width-ui.Width(s.titre)-5, 0))))
		// The frame speaks raw mode; on a preview nothing has put the
		// terminal there, so the carriage returns come back out.
		b.WriteString(strings.ReplaceAll(s.p.Frame(width, height, color), "\r\n", "\n"))
	}
	return b.String()
}

// commonPrefix finds a short string present in more than one destination, so
// the filtering state of the preview actually narrows something.
func commonPrefix(dests []config.Destination) string {
	for _, d := range dests {
		for _, word := range strings.Fields(strings.ToLower(d.Desc + " " + d.Name)) {
			if len(word) < 4 {
				continue
			}
			needle := word[:4]
			hits := 0
			for _, other := range dests {
				if strings.Contains(strings.ToLower(other.Name+" "+other.Desc), needle) {
					hits++
				}
			}
			if hits > 1 && hits < len(dests) {
				return needle
			}
		}
	}
	return ""
}
