package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// write drops a config in a temp dir and hands back its path.
func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "destinations.toml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadsADestination(t *testing.T) {
	cfg, err := LoadConfig(write(t, `
[[destination]]
name = "hyperviseur"
desc = "Cockpit"
host = "mon-hote"
forward = [{ local = 9090, to = "127.0.0.1:9090", label = "Cockpit" }]
`))
	if err != nil {
		t.Fatalf("configuration valide refusée : %v", err)
	}
	if len(cfg.Destination) != 1 {
		t.Fatalf("attendu 1 destination, obtenu %d", len(cfg.Destination))
	}
	d, ok := cfg.Find("hyperviseur")
	if !ok {
		t.Fatal("Find ne retrouve pas une destination qui existe")
	}
	if d.Forward[0].Local != 9090 || d.Forward[0].Label != "Cockpit" {
		t.Fatalf("forward mal lu : %+v", d.Forward[0])
	}
}

// A typo in a key is the failure this whole check exists for: TOML decoders
// ignore what they do not know, so `forwards` would silently produce a
// destination with no tunnel at all.
func TestAnUnknownKeyIsRefused(t *testing.T) {
	_, err := LoadConfig(write(t, `
[[destination]]
name = "a"
host = "h"
forwards = [{ local = 1, to = "127.0.0.1:1" }]
`))
	if err == nil {
		t.Fatal("une clé inconnue doit être refusée")
	}
	if !strings.Contains(err.Error(), "forwards") {
		t.Fatalf("l'erreur doit nommer la clé fautive, obtenu : %v", err)
	}
}

func TestRefusesInvalidDestinations(t *testing.T) {
	cases := []struct {
		nom, body, attenduDansErreur string
	}{
		{
			"nom en double",
			`[[destination]]
name = "a"
host = "h"
forward = [{ local = 1, to = "127.0.0.1:1" }]
[[destination]]
name = "a"
host = "h2"
forward = [{ local = 2, to = "127.0.0.1:2" }]`,
			"a",
		},
		{
			"sans forward",
			`[[destination]]
name = "a"
host = "h"`,
			"forward",
		},
		{
			"sans host",
			`[[destination]]
name = "a"
forward = [{ local = 1, to = "127.0.0.1:1" }]`,
			"host",
		},
		{
			"port local hors bornes",
			`[[destination]]
name = "a"
host = "h"
forward = [{ local = 70000, to = "127.0.0.1:1" }]`,
			"70000",
		},
		{
			"cible sans port",
			`[[destination]]
name = "a"
host = "h"
forward = [{ local = 1, to = "127.0.0.1" }]`,
			"127.0.0.1",
		},
	}
	for _, c := range cases {
		t.Run(c.nom, func(t *testing.T) {
			_, err := LoadConfig(write(t, c.body))
			if err == nil {
				t.Fatal("configuration invalide acceptée")
			}
			if !strings.Contains(err.Error(), c.attenduDansErreur) {
				t.Fatalf("l'erreur doit nommer %q, obtenu : %v", c.attenduDansErreur, err)
			}
		})
	}
}

func TestMissingFileSaysWhere(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), "absent.toml"))
	if err == nil {
		t.Fatal("un fichier absent doit être une erreur")
	}
	if !strings.Contains(err.Error(), "absent.toml") {
		t.Fatalf("l'erreur doit donner le chemin cherché, obtenu : %v", err)
	}
}
