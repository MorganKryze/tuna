// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

package config

import (
	"os"
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
// much as the happy path: an empty XDG variable must not produce "/tunny/…"
// hanging off the filesystem root.
func TestPathFollowsXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/elsewhere")
	if got, want := Path(), filepath.Join("/elsewhere", "tunny", "destinations.toml"); got != want {
		t.Fatalf("want %q, got %q", want, got)
	}

	t.Setenv("XDG_CONFIG_HOME", "")
	got := Path()
	if !strings.HasSuffix(got, filepath.Join(".config", "tunny", "destinations.toml")) {
		t.Fatalf("without XDG the path has to fall back to ~/.config, got %q", got)
	}
	if strings.HasPrefix(got, string(filepath.Separator)+"tunny") {
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

// The error a brand-new user meets, and the one an upgrading user meets. Both
// used to be the same sentence with the path in it twice and no next step.
func TestAMissingConfigSaysWhatToDoNext(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)

	_, err := Load(Path())
	if err == nil {
		t.Fatal("a missing config has to be an error")
	}
	if n := strings.Count(err.Error(), Path()); n != 1 {
		t.Errorf("the path belongs in the message once, found it %d times: %v", n, err)
	}
	if !strings.Contains(err.Error(), "destinations.example.toml") {
		t.Errorf("a new user needs somewhere to start, got: %v", err)
	}

	// The same user one release earlier, with a config under the old name.
	if err := os.MkdirAll(filepath.Join(root, "tuna"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tuna", "destinations.toml"), []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Load(Path())
	if err == nil || !strings.Contains(err.Error(), "mv ") {
		t.Errorf("an upgrading user needs the move command, got: %v", err)
	}
}

// Neither path is ever allowed to be relative. Without a home directory the
// fallback used to be ".config/tunny/destinations.toml", which is not a
// fallback: it reads the config out of whatever directory tunny was started
// in, so cd'ing into a directory somebody else can write to changes which
// machines tunny will open a tunnel to.
func TestThePathIsNeverRelative(t *testing.T) {
	unset(t, "XDG_CONFIG_HOME")
	unset(t, "HOME")
	if got := Path(); !filepath.IsAbs(got) {
		t.Errorf("without HOME the path has to stay absolute, got %q", got)
	}
}

func unset(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "") // registers the cleanup that puts the real value back
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
}
