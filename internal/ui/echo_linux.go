// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

//go:build linux

package ui

import "golang.org/x/sys/unix"

const (
	ioctlReadTermios  = unix.TCGETS
	ioctlWriteTermios = unix.TCSETS
)
