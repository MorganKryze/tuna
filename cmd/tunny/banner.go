// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/MorganKryze/tunny/internal/config"
	"github.com/MorganKryze/tunny/internal/ui"
)

// banner is what replaces the picker once a destination is chosen: the same
// selection bar and the same indentation, so the eye lands where it already
// was, then the URLs to actually open.
func banner(d *config.Destination, color bool) string {
	t := ui.Theme(color)

	labelW := 0
	for _, f := range d.Forward {
		labelW = max(labelW, ui.Width(label(d, f)))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "\n  %s %s", t.Wrap(ui.Accent, "▌"), t.Wrap(ui.Bold, d.Name))
	if d.Desc != "" {
		fmt.Fprintf(&b, "   %s", t.Wrap(ui.Dim, d.Desc))
	}
	b.WriteString("\n\n")

	for _, f := range d.Forward {
		// The URL is left whole and unstyled: terminals turn a bare URL into
		// something clickable, and a colour code in the middle of one is how
		// that stops working.
		fmt.Fprintf(&b, "    %s   http://localhost:%d\n",
			t.Wrap(ui.Dim, ui.Fit(label(d, f), labelW)), f.Local)
	}
	return b.String()
}

// label falls back to the destination name, so a forward with no label still
// gets a word rather than a bare port number.
func label(d *config.Destination, f config.Forward) string {
	if f.Label != "" {
		return f.Label
	}
	return d.Name
}

// The four lines a running tunnel can print. They share a marker column so
// they read as one conversation rather than four unrelated notices, and every
// one of them exists because ssh -N says nothing at all: without them an open
// tunnel, a dropped one, a recovered one and a closed one look identical from
// the terminal.
func established(color bool) string {
	t := ui.Theme(color)
	return fmt.Sprintf("\n    %s %s\n", t.Wrap(ui.Ok, "✓"),
		t.Wrap(ui.Dim, "tunnel open · Ctrl-C to close"))
}

func restored(color bool) string {
	t := ui.Theme(color)
	return fmt.Sprintf("    %s %s\n", t.Wrap(ui.Ok, "✓"), t.Wrap(ui.Dim, "connection restored"))
}

// closed is what settles the question the silence used to leave open: whether
// tunny stopped or is about to try again.
func closed(color bool) string {
	t := ui.Theme(color)
	return fmt.Sprintf("\n    %s %s\n\n", t.Wrap(ui.Ok, "✓"), t.Wrap(ui.Dim, "tunnel closed"))
}

// failed is the last line, and the only one in red. ssh's own words are kept
// verbatim on the first line; anything tunny adds is indented under it.
func failed(err error, color bool) string {
	t := ui.Theme(color)
	lines := strings.Split(err.Error(), "\n")
	out := fmt.Sprintf("\n    %s %s\n", t.Wrap(ui.Err, "✗"), t.Wrap(ui.Bold, lines[0]))
	for _, l := range lines[1:] {
		out += t.Wrap(ui.Dim, l) + "\n"
	}
	return out + "\n"
}

// retrying is the line printed between two attempts. Without it a dropped
// tunnel looks exactly like a hung one: ssh says nothing on the way down, and
// a terminal that has gone quiet for four seconds reads as a crash.
func retrying(attempt, max int, wait time.Duration, color bool) string {
	t := ui.Theme(color)
	return fmt.Sprintf("\n    %s %s\n",
		t.Wrap(ui.Warn, "⟳"),
		t.Wrap(ui.Dim, fmt.Sprintf("connection lost, retrying %d/%d in %s",
			attempt, max, wait.Round(time.Second))))
}

// busyError is what replaces ssh's three lines of complaint when the local
// port is already taken. It names the port and the one command that says who
// has it, because "already in use" without "by what" is a question, not an
// answer.
func busyError(name string, taken []int) error {
	ports := make([]string, len(taken))
	for i, n := range taken {
		ports[i] = strconv.Itoa(n)
	}
	what, which := "local port %s is already taken", "shows what has it"
	if len(taken) > 1 {
		what, which = "local ports %s are already taken", "shows what has them"
	}
	return fmt.Errorf("%s: "+fmt.Sprintf(what, strings.Join(ports, ", "))+
		"\n      lsof -nP -iTCP:%s -sTCP:LISTEN   "+which,
		name, strings.Join(ports, ","))
}
