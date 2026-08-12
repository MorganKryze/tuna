// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

package config

import (
	"strings"
	"testing"
)

func TestRefusesInvalidDestinations(t *testing.T) {
	cases := []struct {
		name, body, wantInError string
	}{
		{
			"empty file",
			``,
			"no destination",
		},
		{
			"no name",
			`[[destination]]
host = "h"
forward = [{ local = 1, to = "127.0.0.1:1" }]`,
			"name",
		},
		{
			"duplicate name",
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
			"no forward",
			`[[destination]]
name = "a"
host = "h"`,
			"forward",
		},
		{
			"no host",
			`[[destination]]
name = "a"
forward = [{ local = 1, to = "127.0.0.1:1" }]`,
			"host",
		},
		{
			"ssh port out of range",
			`[[destination]]
name = "a"
host = "h"
port = 70000
forward = [{ local = 1, to = "127.0.0.1:1" }]`,
			"70000",
		},
		{
			"local port out of range",
			`[[destination]]
name = "a"
host = "h"
forward = [{ local = 70000, to = "127.0.0.1:1" }]`,
			"70000",
		},
		{
			"local port at zero",
			`[[destination]]
name = "a"
host = "h"
forward = [{ to = "127.0.0.1:1" }]`,
			"0",
		},
		{
			"target with no port",
			`[[destination]]
name = "a"
host = "h"
forward = [{ local = 1, to = "127.0.0.1" }]`,
			"127.0.0.1",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Load(write(t, c.body))
			if err == nil {
				t.Fatal("invalid config accepted")
			}
			if !strings.Contains(err.Error(), c.wantInError) {
				t.Fatalf("the error has to name %q, got: %v", c.wantInError, err)
			}
		})
	}
}
