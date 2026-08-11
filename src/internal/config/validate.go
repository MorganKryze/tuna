package config

import (
	"fmt"
	"strings"
)

// validate runs at load time, never later. A mistake in the TOML has to fail
// with the offending key in hand, not three seconds into an ssh whose usage
// error says nothing about which destination caused it.
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
