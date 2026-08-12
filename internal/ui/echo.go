// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

package ui

import (
	"os"

	"golang.org/x/sys/unix"
)

// HideControlEcho stops the terminal from echoing control characters, and
// returns the function that puts it back.
//
// The "^C" that appears when you close a tunnel is not printed by tunny: the
// terminal driver echoes control characters itself, under the ECHOCTL flag.
// Turning it off for the life of the tunnel leaves the screen showing only
// what tunny and ssh actually said.
//
// Not raw mode, deliberately: ssh still holds this terminal, and the host-key
// prompt and any passphrase need ordinary echo and line editing to work. This
// clears one flag and nothing else.
//
// A caller that is not on a terminal gets a restore function that does
// nothing, which is the same shape and saves every call site a check.
func HideControlEcho(f *os.File) func() {
	fd := int(f.Fd())
	t, err := unix.IoctlGetTermios(fd, ioctlReadTermios)
	if err != nil {
		return func() {}
	}
	restore := *t
	t.Lflag &^= unix.ECHOCTL
	if err := unix.IoctlSetTermios(fd, ioctlWriteTermios, t); err != nil {
		return func() {}
	}
	// ponytail: a tunny killed with SIGKILL never runs this, and leaves the
	// terminal not echoing "^C" until `stty sane`. That is the mildest thing
	// a half-restored terminal can be — nothing is invisible, nothing is
	// unusable — which is why one flag is worth clearing and raw mode is not.
	return func() { _ = unix.IoctlSetTermios(fd, ioctlWriteTermios, &restore) }
}
