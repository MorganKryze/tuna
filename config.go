package main

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

// ConfigPath honours XDG_CONFIG_HOME and falls back to ~/.config.
func ConfigPath() string {
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

func LoadConfig(path string) (*Config, error) {
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

func (c *Config) validate() error {
	if len(c.Destination) == 0 {
		return fmt.Errorf("aucune destination définie")
	}
	seen := make(map[string]bool, len(c.Destination))
	for i := range c.Destination {
		d := &c.Destination[i]
		switch {
		case d.Name == "":
			return fmt.Errorf("destination n°%d : name est obligatoire", i+1)
		case seen[d.Name]:
			return fmt.Errorf("destination %q : nom en double", d.Name)
		case d.Host == "":
			return fmt.Errorf("destination %q : host est obligatoire", d.Name)
		case len(d.Forward) == 0:
			return fmt.Errorf("destination %q : au moins un forward est nécessaire", d.Name)
		case d.Port < 0 || d.Port > 65535:
			return fmt.Errorf("destination %q : port %d hors bornes", d.Name, d.Port)
		}
		seen[d.Name] = true
		for _, f := range d.Forward {
			if f.Local < 1 || f.Local > 65535 {
				return fmt.Errorf("destination %q : port local %d hors bornes", d.Name, f.Local)
			}
			// ssh wants host:port on the far side. Catching it here beats
			// catching it as an opaque ssh usage error three seconds later.
			if !strings.Contains(f.To, ":") {
				return fmt.Errorf("destination %q : cible %q doit s'écrire hôte:port", d.Name, f.To)
			}
		}
	}
	return nil
}

func (c *Config) Find(name string) (*Destination, bool) {
	for i := range c.Destination {
		if c.Destination[i].Name == name {
			return &c.Destination[i], true
		}
	}
	return nil, false
}
