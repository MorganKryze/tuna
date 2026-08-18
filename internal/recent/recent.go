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
		// No home means nowhere to look, and a relative path here would
		// mean "wherever tunny happened to be started". That is not a
		// fallback, it is reading the order file out of whatever directory
		// somebody cd'd into. Rooting it keeps a miss a miss.
		home = "/"
	}
	return filepath.Join(home, ".local", "state", "tunny", "recent")
}

// Load never fails. A missing or damaged order file costs a list in the wrong
// order, which is not worth refusing to start over.
func Load(path string) []string {
	// The path is tunny's own, built by Path from XDG_STATE_HOME or from the
	// home directory. Somebody who can set either can already run anything.
	body, err := os.ReadFile(path) //nolint:gosec // the path is ours, not input
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

// Save writes the order, creating the directory on the way: ~/.local/state/tunny
// does not exist on a fresh machine.
//
// Written to a temporary file and renamed, because rename replaces the file in
// one step and a write does not. This runs immediately before a tunnel that
// may stay open for hours, so the window between "truncated" and "written
// again" is the window a laptop lid closes in. Load tolerates a torn file, but
// it tolerates it by losing the order — which is the only thing this package
// has.
//
// ponytail: no fsync. The order of a menu is not worth a disk flush, and the
// failure it would protect against loses a list, not a tunnel.
func Save(path string, names []string) error {
	dir := filepath.Dir(path)
	// 0700, not 0755. The file inside is 0600 and lists which machines
	// somebody administers; leaving the directory readable publishes the fact
	// that it exists and how often it changes, for no one's benefit.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// In the target's own directory: rename is only atomic within a
	// filesystem, and the one /tmp is on is rarely the one $XDG_STATE_HOME is
	// on. CreateTemp opens at 0600, which is the mode this file has to keep.
	f, err := os.CreateTemp(dir, ".recent-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }() // a no-op once the rename lands
	if _, err := f.WriteString(strings.Join(names, "\n") + "\n"); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
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
