// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

package recent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSurvivesARoundTripAndAMissingFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "subdir", "recent")

	// A first run has no state file, and that is not an error.
	if got := Load(p); len(got) != 0 {
		t.Fatalf("a missing file has to give an empty list, got %v", got)
	}
	// Save creates the directory: ~/.local/state/tunny does not exist on a
	// fresh machine.
	if err := Save(p, []string{"b", "a"}); err != nil {
		t.Fatal(err)
	}
	if got := Load(p); !equal(got, []string{"b", "a"}) {
		t.Fatalf("want [b a], got %v", got)
	}

	// A corrupt file is not worth an error either: the worst outcome is a
	// list in the wrong order, and refusing to start over it would be absurd.
	if err := os.WriteFile(p, []byte("\n\n  \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := Load(p); len(got) != 0 {
		t.Fatalf("blank lines have to be ignored, got %v", got)
	}
}

// The file is meant to be repaired by hand, so it has to survive having been
// repaired by hand: trailing spaces and a missing final newline included.
func TestLoadTrimsHandEdits(t *testing.T) {
	p := filepath.Join(t.TempDir(), "recent")
	if err := os.WriteFile(p, []byte("  b  \n\na"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := Load(p); !equal(got, []string{"b", "a"}) {
		t.Fatalf("want [b a], got %v", got)
	}
}

func TestPathFollowsXDG(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/elsewhere")
	if got, want := Path(), filepath.Join("/elsewhere", "tunny", "recent"); got != want {
		t.Fatalf("want %q, got %q", want, got)
	}

	t.Setenv("XDG_STATE_HOME", "")
	got := Path()
	if !strings.HasSuffix(got, filepath.Join(".local", "state", "tunny", "recent")) {
		t.Fatalf("without XDG the path has to fall back to ~/.local/state, got %q", got)
	}
}

// The order file records which machines someone administers, so it is theirs
// to read and nobody else's. Widening the mode used to leave the suite green.
func TestTheOrderFileIsNotWorldReadable(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "recent")
	if err := Save(p, []string{"a"}); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("want mode 0600, got %#o", perm)
	}
}
