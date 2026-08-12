// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

package tunnel

import (
	"strings"
	"testing"

	"github.com/MorganKryze/tunny/internal/config"
)

func TestSSHArgsCarryEveryField(t *testing.T) {
	got := strings.Join(SSHArgs(&config.Destination{
		Name: "vm",
		Host: "debian@10.0.0.5",
		Port: 22022,
		Jump: "mon-hote",
		Forward: []config.Forward{
			{Local: 8201, To: "127.0.0.1:8200"},
			{Local: 9120, To: "10.0.0.5:9120"},
		},
	}), " ")

	for _, want := range []string{
		"-N",
		"-o ExitOnForwardFailure=yes",
		"-J mon-hote",
		"-p 22022",
		"-L 8201:127.0.0.1:8200",
		"-L 9120:10.0.0.5:9120",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("ssh args without %q: %s", want, got)
		}
	}
	// The host is what ssh reads as its target: anything after it is taken
	// as a remote command, which -N forbids.
	if !strings.HasSuffix(got, "debian@10.0.0.5") {
		t.Errorf("the host has to be the last argument: %s", got)
	}
}

func TestSSHArgsOmitWhatIsNotSet(t *testing.T) {
	got := strings.Join(SSHArgs(&config.Destination{
		Host:    "mon-hote",
		Forward: []config.Forward{{Local: 9090, To: "127.0.0.1:9090"}},
	}), " ")
	if strings.Contains(got, "-J") || strings.Contains(got, "-p") {
		t.Errorf("an empty field must produce no flag: %s", got)
	}
}

// Each argument has to be its own element: hand ssh "-L 8201:…" as a single
// string and it reads the space as part of the forward specification.
func TestSSHArgsAreSeparateElements(t *testing.T) {
	args := SSHArgs(&config.Destination{
		Host:    "h",
		Forward: []config.Forward{{Local: 1, To: "127.0.0.1:1"}},
	})
	for _, a := range args {
		if strings.Contains(a, " ") {
			t.Fatalf("an argument contains a space: %q in %v", a, args)
		}
	}
}
