package recent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSurvivesARoundTripAndAMissingFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sous-dossier", "recent")

	// A first run has no state file, and that is not an error.
	if got := Load(p); len(got) != 0 {
		t.Fatalf("fichier absent doit donner une liste vide, obtenu %v", got)
	}
	// Save creates the directory: ~/.local/state/tuna does not exist on a
	// fresh machine.
	if err := Save(p, []string{"b", "a"}); err != nil {
		t.Fatal(err)
	}
	if got := Load(p); !equal(got, []string{"b", "a"}) {
		t.Fatalf("attendu [b a], obtenu %v", got)
	}

	// A corrupt file is not worth an error either: the worst outcome is a
	// list in the wrong order, and refusing to start over it would be absurd.
	if err := os.WriteFile(p, []byte("\n\n  \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := Load(p); len(got) != 0 {
		t.Fatalf("lignes vides doivent être ignorées, obtenu %v", got)
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
		t.Fatalf("attendu [b a], obtenu %v", got)
	}
}

func TestPathFollowsXDG(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/ailleurs")
	if got, want := Path(), filepath.Join("/ailleurs", "tuna", "recent"); got != want {
		t.Fatalf("attendu %q, obtenu %q", want, got)
	}

	t.Setenv("XDG_STATE_HOME", "")
	got := Path()
	if !strings.HasSuffix(got, filepath.Join(".local", "state", "tuna", "recent")) {
		t.Fatalf("sans XDG le chemin doit retomber sur ~/.local/state, obtenu %q", got)
	}
}
