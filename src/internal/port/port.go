// Package port answers one question: is this local port already taken?
//
// It exists because the answer is knowable before ssh is launched, and
// finding out afterwards costs a banner, three lines of ssh diagnostics and
// an abandoned tunnel — for a condition the picker could have shown from the
// start.
package port

import (
	"fmt"
	"net"

	"github.com/MorganKryze/tuna/src/internal/config"
)

// Busy reports whether something is already listening on 127.0.0.1:n.
//
// The probe is a bind, which is the same syscall ssh -L is about to make:
// anything cheaper would be a guess about somebody else's socket. Go sets
// SO_REUSEADDR on its listeners, so a port left in TIME_WAIT does not read as
// busy while a port genuinely being listened on does.
func Busy(n int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", n))
	if err != nil {
		return true
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
