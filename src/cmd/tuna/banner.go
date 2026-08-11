package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/MorganKryze/tuna/src/internal/config"
	"github.com/MorganKryze/tuna/src/internal/ui"
)

// banner is what replaces the picker once a destination is chosen: the same
// selection bar and the same indentation, so the eye lands where it already
// was, then the URLs to actually open.
func banner(d *config.Destination, color bool) string {
	t := ui.Theme(color)

	labelW := 0
	for _, f := range d.Forward {
		labelW = max(labelW, ui.Runes(label(d, f)))
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
	fmt.Fprintf(&b, "\n    %s\n\n", t.Wrap(ui.Dim, "Ctrl-C pour fermer"))
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

// retrying is the line printed between two attempts. Without it a dropped
// tunnel looks exactly like a hung one: ssh says nothing on the way down, and
// a terminal that has gone quiet for four seconds reads as a crash.
func retrying(attempt, max int, wait time.Duration, color bool) string {
	t := ui.Theme(color)
	return fmt.Sprintf("    %s %s\n",
		t.Wrap(ui.Warn, "⟳"),
		t.Wrap(ui.Dim, fmt.Sprintf("connexion perdue — tentative %d/%d dans %s",
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
	quoi, qui := "le port local %s est déjà pris", "dit qui l'occupe"
	if len(taken) > 1 {
		quoi, qui = "les ports locaux %s sont déjà pris", "dit qui les occupe"
	}
	return fmt.Errorf("%s : "+fmt.Sprintf(quoi, strings.Join(ports, ", "))+
		"\n      lsof -nP -iTCP:%s -sTCP:LISTEN   "+qui,
		name, strings.Join(ports, ","))
}
