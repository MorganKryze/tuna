// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

// Package config reads and validates destinations.toml. It depends on
// nothing else in the program: every other package takes what it needs from
// here, and none of them hand anything back.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Forward is one -L: a local port, where it lands on the far side, and the
// name shown to a human. Label is decoration; the tunnel works without it.
type Forward struct {
	Local int    `toml:"local"`
	To    string `toml:"to"`
	Label string `toml:"label"`
}

// Destination is one ssh invocation. Host, Port and Jump deliberately mirror
// ssh's own vocabulary rather than inventing a second one: an alias in
// ~/.ssh/config usually carries the last two already.
type Destination struct {
	Name    string    `toml:"name"`
	Desc    string    `toml:"desc"`
	Host    string    `toml:"host"`
	Port    int       `toml:"port"`
	Jump    string    `toml:"jump"`
	Forward []Forward `toml:"forward"`
}

// Config is the whole file: a list of destinations and nothing else. There
// are no global settings, because every one of them would be a second place
// to look for an answer that ssh_config already has.
type Config struct {
	Destination []Destination `toml:"destination"`
}

// Path honours XDG_CONFIG_HOME and falls back to ~/.config.
func Path() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "tunny", "destinations.toml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		// No home means nowhere to look, and a relative path here would
		// mean "wherever tunny happened to be started". That is not a
		// fallback, it is reading destinations.toml out of whatever directory
		// somebody cd'd into. Rooting it keeps a miss a miss.
		home = "/"
	}
	return filepath.Join(home, ".config", "tunny", "destinations.toml")
}

// Load reads and validates the file, refusing anything it does not
// understand. A config that half-parses is a tunnel that half-opens.
func Load(path string) (*Config, error) {
	var cfg Config
	md, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, missing(path)
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	// A decoder ignores keys it does not know, which turns `forwards` into a
	// destination with no tunnel and no complaint. Undecoded() is what makes
	// the typo loud.
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, 0, len(undecoded))
		for _, k := range undecoded {
			keys = append(keys, k.String())
		}
		return nil, fmt.Errorf("unknown key in %s: %s", path, strings.Join(keys, ", "))
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &cfg, nil
}

// missing is the one error every new user meets, so it owes them a next step.
// The wrapped fs.PathError already names the path, and repeating it read as a
// stutter; "no such file or directory" on its own told nobody what to do.
func missing(path string) error {
	// tunny was called tuna until v0.2.0. Anyone upgrading has a config one
	// directory over, and the fix is a move rather than a rewrite.
	if old := filepath.Join(filepath.Dir(filepath.Dir(path)), "tuna", "destinations.toml"); exists(old) {
		return fmt.Errorf("no config at %s\n      tunny was called tuna until v0.2.0: mv %s %s",
			path, filepath.Dir(old), filepath.Dir(path))
	}
	return fmt.Errorf("no config at %s\n      start from destinations.example.toml: https://github.com/MorganKryze/tunny#getting-started",
		path)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Find returns the destination with this exact name. No prefix matching and
// no fuzz: `tunny prod` opening a tunnel to "production-backup" is not a
// convenience.
func (c *Config) Find(name string) (*Destination, bool) {
	for i := range c.Destination {
		if c.Destination[i].Name == name {
			return &c.Destination[i], true
		}
	}
	return nil, false
}

// Names lists every destination in config order, for the "unknown
// destination" message.
func (c *Config) Names() []string {
	out := make([]string, 0, len(c.Destination))
	for _, d := range c.Destination {
		out = append(out, d.Name)
	}
	return out
}
