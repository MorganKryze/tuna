// Package config reads and validates destinations.toml. It depends on
// nothing else in the program: every other package takes what it needs from
// here, and none of them hand anything back.
package config

import (
	"fmt"
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

type Config struct {
	Destination []Destination `toml:"destination"`
}

// Path honours XDG_CONFIG_HOME and falls back to ~/.config.
func Path() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "tuna", "destinations.toml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		// No home means no config to find; the caller reports the miss.
		return filepath.Join(".config", "tuna", "destinations.toml")
	}
	return filepath.Join(home, ".config", "tuna", "destinations.toml")
}

func Load(path string) (*Config, error) {
	var cfg Config
	md, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return nil, fmt.Errorf("lecture de %s : %w", path, err)
	}
	// A decoder ignores keys it does not know, which turns `forwards` into a
	// destination with no tunnel and no complaint. Undecoded() is what makes
	// the typo loud.
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, 0, len(undecoded))
		for _, k := range undecoded {
			keys = append(keys, k.String())
		}
		return nil, fmt.Errorf("clé inconnue dans %s : %s", path, strings.Join(keys, ", "))
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("%s : %w", path, err)
	}
	return &cfg, nil
}

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
