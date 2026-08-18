// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

// Package port answers one question: is this local port already taken?
//
// It exists because the answer is knowable before ssh is launched, and
// finding out afterwards costs a banner, three lines of ssh diagnostics and
// an abandoned tunnel — for a condition the picker could have shown from the
// start.
package port

import (
	"errors"
	"net"
	"strconv"
	"syscall"

	"github.com/MorganKryze/tunny/internal/config"
)

// Busy reports whether something is already listening on the local port.
//
// Both loopback addresses, because ssh -L binds "localhost" and localhost is
// two addresses on any machine with IPv6: a port held on ::1 alone reads as
// free on 127.0.0.1 and the picker then says go ahead to a forward that will
// not bind.
func Busy(n int) bool {
	return taken("127.0.0.1", n) || taken("[::1]", n)
}

// taken reports whether addr refuses a bind because something is already
// there. Any other refusal is not this function's business and is not "busy":
// a machine with IPv6 switched off refuses every bind to ::1, and a port under
// 1024 refuses one from a user who is not root. Reporting either as busy would
// name a culprit that does not exist and send somebody to lsof to look for it.
//
// The probe is a bind, which is the same syscall ssh is about to make:
// anything cheaper would be a guess about somebody else's socket. Go sets
// SO_REUSEADDR on its listeners, so a port left in TIME_WAIT does not read as
// busy while a port genuinely being listened on does.
func taken(addr string, n int) bool {
	l, err := net.Listen("tcp", addr+":"+strconv.Itoa(n))
	if err != nil {
		return errors.Is(err, syscall.EADDRINUSE)
	}
	// ponytail: held for the microsecond between Listen and Close, which is
	// a race nobody can lose in practice — and losing it costs a retry, not
	// a wrong answer, because Connect still classifies ssh's own complaint.
	_ = l.Close()
	return false
}

// BusyIn maps each destination's name to the local ports already taken,
// leaving out the destinations that are entirely free. A missing key means
// "nothing in the way", which is what the picker and the launcher both read.
func BusyIn(dests []config.Destination) map[string][]int {
	out := make(map[string][]int)
	for _, d := range dests {
		var taken []int
		for _, f := range d.Forward {
			if Busy(f.Local) {
				taken = append(taken, f.Local)
			}
		}
		if len(taken) > 0 {
			out[d.Name] = taken
		}
	}
	return out
}
