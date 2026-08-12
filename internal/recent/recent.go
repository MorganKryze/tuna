// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

// Package recent keeps the order in which destinations were last used. The
// order is the data: no timestamps, no JSON, one name per line, most recent
// first. A file you can read with cat is a file you can repair with vim.
//
// It knows nothing about the picker. Recency is computed here and handed to
// the picker as an already-sorted list, which is what makes both testable
// without a terminal.
package recent

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/MorganKryze/tunny/internal/config"
)

// Path honours XDG_STATE_HOME and falls back to ~/.local/state.
func Path() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "tunny", "recent")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "state", "tunny", "recent")
	}
	return filepath.Join(home, ".local", "state", "tunny", "recent")
}

// Load never fails. A missing or damaged order file costs a list in the wrong
// order, which is not worth refusing to start over.
func Load(path string) []string {
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

// Save creates the directory on the way: ~/.local/state/tunny does not exist
// on a fresh machine.
func Save(path string, names []string) error {
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
// config are dropped here, which is the whole cleanup mechanism: a deleted
// destination disappears on its own, with no migration and no housekeeping.
func Order(dests []config.Destination, recent []string) []config.Destination {
	byName := make(map[string]config.Destination, len(dests))
	for _, d := range dests {
		byName[d.Name] = d
	}
	out := make([]config.Destination, 0, len(dests))
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
