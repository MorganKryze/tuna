package main

import (
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/MorganKryze/tuna/src/internal/config"
)

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plain(s string) string { return ansi.ReplaceAllString(s, "") }

func hyperviseur() *config.Destination {
	return &config.Destination{
		Name: "hyperviseur",
		Desc: "Cockpit + Komodo — racine d'admin",
		Forward: []config.Forward{
			{Local: 9090, To: "127.0.0.1:9090", Label: "Cockpit"},
			{Local: 9120, To: "10.10.10.10:9120", Label: "Komodo"},
		},
	}
}

func TestBannerLooksLikeThis(t *testing.T) {
	want := "\n" +
		"  ▌ hyperviseur   Cockpit + Komodo — racine d'admin\n" +
		"\n" +
		"    Cockpit   http://localhost:9090\n" +
		"    Komodo    http://localhost:9120\n"
	if got := banner(hyperviseur(), false); got != want {
		t.Fatalf("unexpected banner.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// The URL is what somebody clicks or copies. A style code anywhere inside it
// is how a terminal stops recognising it as a link, and how a copy-paste
// gains invisible characters.
func TestTheURLIsNeverStyled(t *testing.T) {
	for _, line := range strings.Split(banner(hyperviseur(), true), "\n") {
		i := strings.Index(line, "http://")
		if i < 0 {
			continue
		}
		if ansi.MatchString(line[i:]) {
			t.Fatalf("an ANSI sequence touches the URL: %q", line)
		}
	}
}

func TestBannerFallsBackToTheDestinationName(t *testing.T) {
	got := plain(banner(&config.Destination{
		Name:    "brut",
		Forward: []config.Forward{{Local: 8080, To: "127.0.0.1:80"}},
	}, false))

	// No label: the name stands in, because a bare port number is the one
	// thing the reader already has in the URL next to it.
	if !strings.Contains(got, "    brut   http://localhost:8080") {
		t.Errorf("with no label, the name has to stand in:\n%s", got)
	}
	// No description: no dangling separator where it would have been.
	if strings.Contains(got, "brut   \n") || strings.Contains(got, "▌ brut  ") {
		t.Errorf("an empty description must leave nothing dangling:\n%q", got)
	}
}

// Labels of different lengths have to line the URLs up, or the column reads
// as three unrelated lines.
func TestLabelsArePaddedToACommonWidth(t *testing.T) {
	got := plain(banner(&config.Destination{
		Name: "x",
		Forward: []config.Forward{
			{Local: 1, To: "h:1", Label: "short"},
			{Local: 2, To: "h:2", Label: "a-rather-long-label"},
		},
	}, false))

	var cols []int
	for _, line := range strings.Split(got, "\n") {
		// Columns, not bytes: "é" is one column and two bytes, and comparing
		// byte offsets would accuse the padding of the drift it prevents.
		if i := strings.Index(line, "http://"); i >= 0 {
			cols = append(cols, utf8.RuneCountInString(line[:i]))
		}
	}
	if len(cols) != 2 || cols[0] != cols[1] {
		t.Fatalf("the URLs have to start at the same column, got %v:\n%s", cols, got)
	}
}

func TestRetryingSaysWhichAttemptAndHowLong(t *testing.T) {
	got := plain(retrying(2, 3, 2*time.Second, false))
	for _, want := range []string{"2/3", "2s", "connection lost"} {
		if !strings.Contains(got, want) {
			t.Errorf("want %q in %q", want, got)
		}
	}
	if !strings.HasSuffix(got, "\n") {
		t.Error("the line has to end, or ssh writes right after it")
	}
}

func TestNoColourMeansNoEscapeCodes(t *testing.T) {
	if ansi.MatchString(banner(hyperviseur(), false)) {
		t.Error("an ANSI sequence survived color=false in the banner")
	}
	if ansi.MatchString(retrying(1, 3, time.Second, false)) {
		t.Error("an ANSI sequence survived color=false in the retry notice")
	}
}

// The message this replaces was three lines of ssh diagnostics arriving after
// a banner that had already promised a tunnel. What it owes the reader is the
// port and a way to find out what has it.
func TestBusyErrorNamesThePortAndHowToFindIt(t *testing.T) {
	one := busyError("control-plane", []int{8201}).Error()
	for _, want := range []string{"control-plane", "8201", "lsof -nP -iTCP:8201 -sTCP:LISTEN"} {
		if !strings.Contains(one, want) {
			t.Errorf("want %q in:\n%s", want, one)
		}
	}
	if !strings.Contains(one, "local port") || !strings.Contains(one, "has it") {
		t.Errorf("one port has to read as singular:\n%s", one)
	}

	many := busyError("hyperviseur", []int{9090, 9120}).Error()
	if !strings.Contains(many, "local ports") || !strings.Contains(many, "has them") {
		t.Errorf("several ports have to read as plural:\n%s", many)
	}
	// lsof takes its ports comma-separated with no space; a space there makes
	// the command in the message one that does not run.
	if !strings.Contains(many, "-iTCP:9090,9120 ") {
		t.Errorf("the lsof command has to be pasteable as it stands:\n%s", many)
	}
}

// Four states, four lines, and the point of all of them is that ssh -N says
// nothing: without them an open tunnel, a dropped one, a recovered one and a
// closed one look identical from the terminal.
func TestTheStatusLines(t *testing.T) {
	cases := []struct {
		name, got, marker, text string
	}{
		{"open", established(false), "✓", "tunnel open · Ctrl-C to close"},
		{"restored", restored(false), "✓", "connection restored"},
		{"closed", closed(false), "✓", "tunnel closed"},
		{"lost", retrying(1, 3, time.Second, false), "⟳", "retrying 1/3"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(c.got, c.marker) || !strings.Contains(c.got, c.text) {
				t.Fatalf("want %q and %q, got %q", c.marker, c.text, c.got)
			}
			// The same marker column as every other line, or they read as
			// four unrelated notices instead of one conversation.
			if !strings.HasPrefix(strings.TrimLeft(c.got, "\n"), "    ") {
				t.Errorf("want an indent of 4 spaces: %q", c.got)
			}
			if !strings.HasSuffix(c.got, "\n") {
				t.Errorf("the line has to end, or ssh writes right after it: %q", c.got)
			}
		})
	}
}

// The banner no longer promises a way to close something that has not opened:
// that sentence moved onto the line printed once the tunnel is actually up.
func TestTheBannerDoesNotPromiseAnOpenTunnel(t *testing.T) {
	if strings.Contains(banner(hyperviseur(), false), "Ctrl-C") {
		t.Error("the banner prints before the tunnel exists: it must not talk about closing it")
	}
}

func TestFailedKeepsSshsOwnWordsFirst(t *testing.T) {
	got := plain(failed(busyError("control-plane", []int{8201}), false))
	lines := strings.Split(strings.Trim(got, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d: %q", len(lines), got)
	}
	if !strings.HasPrefix(lines[0], "    ✗ ") {
		t.Errorf("the first line carries the marker: %q", lines[0])
	}
	if !strings.Contains(lines[1], "lsof") {
		t.Errorf("the rest is indented underneath: %q", lines[1])
	}
}

// The version is the first thing a bug report asks for, and the release
// workflow's -ldflags only reaches the binaries it builds itself: an install
// straight from a tag has to find its version somewhere else.
func TestVersionFallsBackToTheBuildInfo(t *testing.T) {
	// Under `go test` the build info exists but carries no module version,
	// which is the same shape as a local `go build`.
	if got := versionOr("dev"); got != "dev" {
		t.Fatalf("a local build has to keep the fallback, got %q", got)
	}
	if version == "" {
		t.Error("the version shown must never be empty")
	}
}
