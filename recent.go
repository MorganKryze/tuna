package main

import (
	"os"
	"path/filepath"
	"strings"
)

// StatePath honours XDG_STATE_HOME and falls back to ~/.local/state.
func StatePath() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "tuna", "recent")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "state", "tuna", "recent")
	}
	return filepath.Join(home, ".local", "state", "tuna", "recent")
}

// LoadRecent never fails. A missing or damaged order file costs a list in the
// wrong order, which is not worth refusing to start over.
func LoadRecent(path string) []string {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(body), "\n") {
		if name := strings.TrimSpace(line); name != "" {
			out = append(out, name)
		}
	}
	return out
}

func SaveRecent(path string, names []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(names, "\n")+"\n"), 0o600)
}

// Bump puts chosen at the front, removing any earlier mention of it.
func Bump(names []string, chosen string) []string {
	out := make([]string, 0, len(names)+1)
	out = append(out, chosen)
	for _, n := range names {
		if n != chosen {
			out = append(out, n)
		}
	}
	return out
}

// Order sorts dests by the recency list, then appends whatever the list does
// not mention, in config order. Names in recent that no longer exist in the
// config are dropped here, which is the whole cleanup mechanism.
func Order(dests []Destination, recent []string) []Destination {
	byName := make(map[string]Destination, len(dests))
	for _, d := range dests {
		byName[d.Name] = d
	}
	out := make([]Destination, 0, len(dests))
	placed := make(map[string]bool, len(dests))
	for _, name := range recent {
		if d, ok := byName[name]; ok && !placed[name] {
			out = append(out, d)
			placed[name] = true
		}
	}
	for _, d := range dests {
		if !placed[d.Name] {
			out = append(out, d)
		}
	}
	return out
}
