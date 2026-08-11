package config

import (
	"errors"
	"fmt"
	"strings"
)

// validate runs at load time, never later. A mistake in the TOML has to fail
// with the offending key in hand, not three seconds into an ssh whose usage
// error says nothing about which destination caused it.
func (c *Config) validate() error {
	if len(c.Destination) == 0 {
		return errors.New("no destination defined")
	}
	seen := make(map[string]bool, len(c.Destination))
	for i := range c.Destination {
		d := &c.Destination[i]
		switch {
		case d.Name == "":
			return fmt.Errorf("destination %d: name is required", i+1)
		case seen[d.Name]:
			return fmt.Errorf("destination %q: duplicate name", d.Name)
		case d.Host == "":
			return fmt.Errorf("destination %q: host is required", d.Name)
		case len(d.Forward) == 0:
			return fmt.Errorf("destination %q: needs at least one forward", d.Name)
		case d.Port < 0 || d.Port > 65535:
			return fmt.Errorf("destination %q: port %d out of range", d.Name, d.Port)
		}
		seen[d.Name] = true
		for _, f := range d.Forward {
			if f.Local < 1 || f.Local > 65535 {
				return fmt.Errorf("destination %q: local port %d out of range", d.Name, f.Local)
			}
			// ssh wants host:port on the far side. Catching it here beats
			// catching it as an opaque ssh usage error three seconds later.
			if !strings.Contains(f.To, ":") {
				return fmt.Errorf("destination %q: target %q must be written host:port", d.Name, f.To)
			}
		}
	}
	return nil
}
