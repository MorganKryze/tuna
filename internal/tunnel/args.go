// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

package tunnel

import (
	"fmt"
	"strconv"

	"github.com/MorganKryze/tunny/internal/config"
)

// SSHArgs builds the argument list for the real ssh binary. tunny never
// reimplements ssh through crypto/ssh: that would mean redoing ~/.ssh/config,
// aliases, ProxyJump, the agent and known_hosts — hundreds of lines to do it
// worse, with the unknown-host prompt still left to solve.
func SSHArgs(d *config.Destination) []string {
	// ExitOnForwardFailure is not optional: without it ssh stays connected
	// with a dead forward, and the first sign of trouble is a browser tab
	// that never loads, minutes later.
	args := []string{"-N", "-o", "ExitOnForwardFailure=yes"}
	if d.Jump != "" {
		args = append(args, "-J", d.Jump)
	}
	if d.Port != 0 {
		args = append(args, "-p", strconv.Itoa(d.Port))
	}
	for _, f := range d.Forward {
		args = append(args, "-L", fmt.Sprintf("%d:%s", f.Local, f.To))
	}
	// Last, always: ssh reads anything after the host as a remote command,
	// which -N forbids.
	return append(args, d.Host)
}
