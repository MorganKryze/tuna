// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package ui

import "golang.org/x/sys/unix"

// The ioctl that reads and writes termios has a different name on each
// family. Two constants in two files is what x/term does for the same reason.
const (
	ioctlReadTermios  = unix.TIOCGETA
	ioctlWriteTermios = unix.TIOCSETA
)
