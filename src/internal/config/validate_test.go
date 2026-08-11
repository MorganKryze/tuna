package config

import (
	"strings"
	"testing"
)

func TestRefusesInvalidDestinations(t *testing.T) {
	cases := []struct {
		nom, body, attenduDansErreur string
	}{
		{
			"fichier vide",
			``,
			"aucune destination",
		},
		{
			"sans nom",
			`[[destination]]
host = "h"
forward = [{ local = 1, to = "127.0.0.1:1" }]`,
			"name",
		},
		{
			"nom en double",
			`[[destination]]
name = "a"
host = "h"
forward = [{ local = 1, to = "127.0.0.1:1" }]
[[destination]]
name = "a"
host = "h2"
forward = [{ local = 2, to = "127.0.0.1:2" }]`,
			"a",
		},
		{
			"sans forward",
			`[[destination]]
name = "a"
host = "h"`,
			"forward",
		},
		{
			"sans host",
			`[[destination]]
name = "a"
forward = [{ local = 1, to = "127.0.0.1:1" }]`,
			"host",
		},
		{
			"port ssh hors bornes",
			`[[destination]]
name = "a"
host = "h"
port = 70000
forward = [{ local = 1, to = "127.0.0.1:1" }]`,
			"70000",
		},
		{
			"port local hors bornes",
			`[[destination]]
name = "a"
host = "h"
forward = [{ local = 70000, to = "127.0.0.1:1" }]`,
			"70000",
		},
		{
			"port local à zéro",
			`[[destination]]
name = "a"
host = "h"
forward = [{ to = "127.0.0.1:1" }]`,
			"0",
		},
		{
			"cible sans port",
			`[[destination]]
name = "a"
host = "h"
forward = [{ local = 1, to = "127.0.0.1" }]`,
			"127.0.0.1",
		},
	}
	for _, c := range cases {
		t.Run(c.nom, func(t *testing.T) {
			_, err := Load(write(t, c.body))
			if err == nil {
				t.Fatal("configuration invalide acceptée")
			}
			if !strings.Contains(err.Error(), c.attenduDansErreur) {
				t.Fatalf("l'erreur doit nommer %q, obtenu : %v", c.attenduDansErreur, err)
			}
		})
	}
}
