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
		t.Fatalf("bannière inattendue.\n--- obtenu ---\n%s\n--- attendu ---\n%s", got, want)
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
			t.Fatalf("une séquence ANSI touche l'URL : %q", line)
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
		t.Errorf("sans label, le nom doit servir d'étiquette :\n%s", got)
	}
	// No description: no dangling separator where it would have been.
	if strings.Contains(got, "brut   \n") || strings.Contains(got, "▌ brut  ") {
		t.Errorf("une description vide ne doit rien laisser traîner :\n%q", got)
	}
}

// Labels of different lengths have to line the URLs up, or the column reads
// as three unrelated lines.
func TestLabelsArePaddedToACommonWidth(t *testing.T) {
	got := plain(banner(&config.Destination{
		Name: "x",
		Forward: []config.Forward{
			{Local: 1, To: "h:1", Label: "court"},
			{Local: 2, To: "h:2", Label: "une-étiquette-longue"},
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
		t.Fatalf("les URLs doivent commencer à la même colonne, obtenu %v :\n%s", cols, got)
	}
}

func TestRetryingSaysWhichAttemptAndHowLong(t *testing.T) {
	got := plain(retrying(2, 3, 2*time.Second, false))
	for _, attendu := range []string{"2/3", "2s", "connexion perdue"} {
		if !strings.Contains(got, attendu) {
			t.Errorf("%q attendu dans %q", attendu, got)
		}
	}
	if !strings.HasSuffix(got, "\n") {
		t.Error("la ligne doit se terminer, sinon ssh écrit à sa suite")
	}
}

func TestNoColourMeansNoEscapeCodes(t *testing.T) {
	if ansi.MatchString(banner(hyperviseur(), false)) {
		t.Error("une séquence ANSI a survécu à color=false dans la bannière")
	}
	if ansi.MatchString(retrying(1, 3, time.Second, false)) {
		t.Error("une séquence ANSI a survécu à color=false dans l'avis de reconnexion")
	}
}

// The message this replaces was three lines of ssh diagnostics arriving after
// a banner that had already promised a tunnel. What it owes the reader is the
// port and a way to find out what has it.
func TestBusyErrorNamesThePortAndHowToFindIt(t *testing.T) {
	one := busyError("control-plane", []int{8201}).Error()
	for _, attendu := range []string{"control-plane", "8201", "lsof -nP -iTCP:8201 -sTCP:LISTEN"} {
		if !strings.Contains(one, attendu) {
			t.Errorf("%q attendu dans :\n%s", attendu, one)
		}
	}
	if !strings.Contains(one, "le port local") || !strings.Contains(one, "l'occupe") {
		t.Errorf("un seul port doit se dire au singulier :\n%s", one)
	}

	many := busyError("hyperviseur", []int{9090, 9120}).Error()
	if !strings.Contains(many, "les ports locaux") || !strings.Contains(many, "les occupe") {
		t.Errorf("plusieurs ports doivent se dire au pluriel :\n%s", many)
	}
	// lsof takes its ports comma-separated with no space; a space there makes
	// the command in the message one that does not run.
	if !strings.Contains(many, "-iTCP:9090,9120 ") {
		t.Errorf("la commande lsof doit être collable telle quelle :\n%s", many)
	}
}

// Four states, four lines, and the point of all of them is that ssh -N says
// nothing: without them an open tunnel, a dropped one, a recovered one and a
// closed one look identical from the terminal.
func TestTheStatusLines(t *testing.T) {
	cases := []struct {
		nom, got, marqueur, texte string
	}{
		{"ouvert", established(false), "✓", "tunnel ouvert · Ctrl-C pour fermer"},
		{"rétabli", restored(false), "✓", "connexion rétablie"},
		{"fermé", closed(false), "✓", "tunnel fermé"},
		{"perdu", retrying(1, 3, time.Second, false), "⟳", "tentative 1/3"},
	}
	for _, c := range cases {
		t.Run(c.nom, func(t *testing.T) {
			if !strings.Contains(c.got, c.marqueur) || !strings.Contains(c.got, c.texte) {
				t.Fatalf("attendu %q et %q, obtenu %q", c.marqueur, c.texte, c.got)
			}
			// The same marker column as every other line, or they read as
			// four unrelated notices instead of one conversation.
			if !strings.HasPrefix(strings.TrimLeft(c.got, "\n"), "    ") {
				t.Errorf("indentation attendue de 4 espaces : %q", c.got)
			}
			if !strings.HasSuffix(c.got, "\n") {
				t.Errorf("la ligne doit se terminer, sinon ssh écrit à sa suite : %q", c.got)
			}
		})
	}
}

// The banner no longer promises a way to close something that has not opened:
// that sentence moved onto the line printed once the tunnel is actually up.
func TestTheBannerDoesNotPromiseAnOpenTunnel(t *testing.T) {
	if strings.Contains(banner(hyperviseur(), false), "Ctrl-C") {
		t.Error("la bannière est imprimée avant que le tunnel existe : elle ne doit pas parler de le fermer")
	}
}

func TestFailedKeepsSshsOwnWordsFirst(t *testing.T) {
	got := plain(failed(busyError("control-plane", []int{8201}), false))
	lines := strings.Split(strings.Trim(got, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("attendu 2 lignes, obtenu %d : %q", len(lines), got)
	}
	if !strings.HasPrefix(lines[0], "    ✗ ") {
		t.Errorf("la première ligne porte le marqueur : %q", lines[0])
	}
	if !strings.Contains(lines[1], "lsof") {
		t.Errorf("le reste est indenté dessous : %q", lines[1])
	}
}
