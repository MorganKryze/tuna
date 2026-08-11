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
		t.Fatalf("valid config refused: %v", err)
	}
	if len(cfg.Destination) != 1 {
		t.Fatalf("want 1 destination, got %d", len(cfg.Destination))
	}
	d, ok := cfg.Find("hyperviseur")
	if !ok {
		t.Fatal("Find misses a destination that exists")
	}
	if d.Forward[0].Local != 9090 || d.Forward[0].Label != "Cockpit" {
		t.Fatalf("forward read wrong: %+v", d.Forward[0])
	}
	if _, ok := cfg.Find("absente"); ok {
		t.Fatal("Find invents a destination that does not exist")
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
		t.Fatal("an unknown key has to be refused")
	}
	if !strings.Contains(err.Error(), "forwards") {
		t.Fatalf("the error has to name the offending key, got: %v", err)
	}
}

func TestMissingFileSaysWhere(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "absent.toml"))
	if err == nil {
		t.Fatal("a missing file has to be an error")
	}
	if !strings.Contains(err.Error(), "absent.toml") {
		t.Fatalf("the error has to give the path it looked for, got: %v", err)
	}
}

// Path is what the error message above prints, so the fallback matters as
// much as the happy path: an empty XDG variable must not produce "/tuna/…"
// hanging off the filesystem root.
func TestPathFollowsXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/elsewhere")
	if got, want := Path(), filepath.Join("/elsewhere", "tuna", "destinations.toml"); got != want {
		t.Fatalf("want %q, got %q", want, got)
	}

	t.Setenv("XDG_CONFIG_HOME", "")
	got := Path()
	if !strings.HasSuffix(got, filepath.Join(".config", "tuna", "destinations.toml")) {
		t.Fatalf("without XDG the path has to fall back to ~/.config, got %q", got)
	}
	if strings.HasPrefix(got, string(filepath.Separator)+"tuna") {
		t.Fatalf("an empty XDG must not produce a path at the filesystem root: %q", got)
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
		t.Fatalf("Names has to follow the order of the file, got %q", got)
	}
}
