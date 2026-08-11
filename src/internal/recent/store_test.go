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
	// Save creates the directory: ~/.local/state/tuna does not exist on a
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
	if got, want := Path(), filepath.Join("/elsewhere", "tuna", "recent"); got != want {
		t.Fatalf("want %q, got %q", want, got)
	}

	t.Setenv("XDG_STATE_HOME", "")
	got := Path()
	if !strings.HasSuffix(got, filepath.Join(".local", "state", "tuna", "recent")) {
		t.Fatalf("without XDG the path has to fall back to ~/.local/state, got %q", got)
	}
}
