package config

import (
	"os"
	"path/filepath"
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
