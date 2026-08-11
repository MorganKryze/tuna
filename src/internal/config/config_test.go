package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadsADestination(t *testing.T) {
	cfg, err := Load(write(t, `
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
	if _, ok := cfg.Find("absente"); ok {
		t.Fatal("Find invente une destination qui n'existe pas")
	}
}

// A typo in a key is the failure this whole check exists for: TOML decoders
// ignore what they do not know, so `forwards` would silently produce a
// destination with no tunnel at all.
func TestAnUnknownKeyIsRefused(t *testing.T) {
	_, err := Load(write(t, `
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

func TestMissingFileSaysWhere(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "absent.toml"))
	if err == nil {
		t.Fatal("un fichier absent doit être une erreur")
	}
	if !strings.Contains(err.Error(), "absent.toml") {
		t.Fatalf("l'erreur doit donner le chemin cherché, obtenu : %v", err)
	}
}

// Path is what the error message above prints, so the fallback matters as
// much as the happy path: an empty XDG variable must not produce "/tuna/…"
// hanging off the filesystem root.
func TestPathFollowsXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/ailleurs")
	if got, want := Path(), filepath.Join("/ailleurs", "tuna", "destinations.toml"); got != want {
		t.Fatalf("attendu %q, obtenu %q", want, got)
	}

	t.Setenv("XDG_CONFIG_HOME", "")
	got := Path()
	if !strings.HasSuffix(got, filepath.Join(".config", "tuna", "destinations.toml")) {
		t.Fatalf("sans XDG le chemin doit retomber sur ~/.config, obtenu %q", got)
	}
	if strings.HasPrefix(got, string(filepath.Separator)+"tuna") {
		t.Fatalf("XDG vide ne doit pas donner un chemin à la racine : %q", got)
	}
}

func TestNamesListsInConfigOrder(t *testing.T) {
	cfg, err := Load(write(t, `
[[destination]]
name = "z"
host = "h"
forward = [{ local = 1, to = "127.0.0.1:1" }]
[[destination]]
name = "a"
host = "h"
forward = [{ local = 2, to = "127.0.0.1:2" }]
`))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(cfg.Names(), ","); got != "z,a" {
		t.Fatalf("Names doit suivre l'ordre du fichier, obtenu %q", got)
	}
}
