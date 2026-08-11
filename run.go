package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Outcome separates "the tunnel died" from "the operator closed it". Getting
// this wrong is the worst bug this program can have: a tunnel that relaunches
// itself every time you try to close it.
type Outcome int

const (
	OutcomeFailed Outcome = iota
	OutcomeInterrupted
)

// Result is what a Runner reports back. Lived is measured by the Runner, not
// by Connect: that keeps Connect free of a clock and makes it testable in
// microseconds instead of seconds.
type Result struct {
	Lived   time.Duration
	Stderr  string
	Outcome Outcome
}

type Runner func(d *Destination) Result

type Retry struct {
	Max         int           // reconnection attempts per episode
	StableAfter time.Duration // held this long = the episode is over
	Sleep       func(time.Duration)
}

func DefaultRetry() Retry {
	return Retry{Max: 3, StableAfter: 30 * time.Second, Sleep: time.Sleep}
}

func SSHArgs(d *Destination) []string {
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
	// Last, always: ssh reads anything after the host as a remote command.
	return append(args, d.Host)
}

// hopelessMarkers are the failures that would repeat identically three times.
// Everything else — host down, network gone, wifi switched, laptop woken —
// is worth another try.
var hopelessMarkers = []string{
	"Address already in use",
	"Permission denied",
	"Could not resolve hostname",
}

// Hopeless reports whether stderr shows a failure retrying cannot fix, and
// returns the offending line so the operator reads ssh's own words rather
// than a paraphrase.
func Hopeless(stderr string) (string, bool) {
	for _, line := range strings.Split(stderr, "\n") {
		for _, marker := range hopelessMarkers {
			if strings.Contains(line, marker) {
				return strings.TrimSpace(line), true
			}
		}
	}
	return "", false
}

func backoff(attempt int) time.Duration {
	return time.Second << (attempt - 1) // 1s, 2s, 4s
}

// Connect runs the tunnel and keeps it up. It returns nil when the operator
// closed it, and an error when it gave up.
func Connect(d *Destination, run Runner, r Retry) error {
	attempts := 0
	for {
		res := run(d)

		if res.Outcome == OutcomeInterrupted {
			return nil
		}
		// Held long enough to count as a working tunnel: whatever just
		// happened starts a fresh episode.
		if res.Lived >= r.StableAfter {
			attempts = 0
		}
		if line, ok := Hopeless(res.Stderr); ok {
			return fmt.Errorf("%s : %s", d.Name, line)
		}

		attempts++
		if attempts > r.Max {
			if r.Max == 0 {
				return fmt.Errorf("%s : connexion échouée", d.Name)
			}
			return fmt.Errorf("%s : abandon après %d tentatives de reconnexion", d.Name, r.Max)
		}
		r.Sleep(backoff(attempts))
	}
}
