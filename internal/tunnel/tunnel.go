// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

// Package tunnel keeps an ssh tunnel up. It is the core of the program and
// the only part with a state machine, so it is the most tested.
//
// Connect knows nothing about os/exec, signals or the clock: it calls a
// Runner, which reports how long the tunnel lived and how it ended. That is
// what lets the whole reconnection policy be tested in microseconds instead
// of seven seconds, and it is why the process handling lives in cmd/tunny
// rather than here.
package tunnel

import (
	"context"
	"fmt"
	"time"

	"github.com/MorganKryze/tunny/internal/config"
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
// by Connect: that keeps Connect free of a clock.
type Result struct {
	Lived   time.Duration
	Stderr  string
	Outcome Outcome
}

// Runner launches the tunnel and blocks until it dies. It calls up once the
// tunnel looks established, which is the only moment Connect cannot observe
// for itself: it is inside run() for the whole life of the process.
//
// The context is how the operator gets out. A cancelled context means stop,
// whether that came from Ctrl-C, a SIGTERM from a service manager, or a
// terminal that went away; the Runner is expected to take its child down with
// it rather than leave one behind.
type Runner func(ctx context.Context, d *config.Destination, up func()) Result

type Retry struct {
	Max         int           // reconnection attempts per episode
	StableAfter time.Duration // held this long = the episode is over
	// Wait sits out the backoff and returns non-nil if it was cut short. It
	// takes a context because most of a failing episode is spent in here —
	// seven of the ten seconds of three attempts — and "it is retrying and I
	// want it to stop" is exactly when somebody reaches for Ctrl-C.
	Wait func(context.Context, time.Duration) error
	// Notify is told about each retry before it is waited out, and OnUp when
	// the tunnel comes up — with reconnected set when it is coming back
	// rather than starting. ssh says nothing either way, so without these a
	// dropped tunnel, a hung one and a recovered one all look alike from the
	// terminal. Both optional: nil simply stays quiet.
	Notify func(attempt, max int, wait time.Duration)
	OnUp   func(reconnected bool)
}

func DefaultRetry() Retry {
	return Retry{Max: 3, StableAfter: 30 * time.Second, Wait: wait}
}

func wait(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// maxBackoff is where the doubling stops. Retry.Max is exported and nothing
// bounds it, and shifting a second left often enough overflows to a negative
// duration — which time.Timer fires immediately, turning the backoff into a
// spin against a host that is already down.
const maxBackoff = 30 * time.Second

func backoff(attempt int) time.Duration {
	switch {
	case attempt < 1:
		return time.Second
	case attempt >= 6: // 1s, 2s, 4s, 8s, 16s, then flat
		return maxBackoff
	}
	return time.Second << (attempt - 1)
}

// Connect runs the tunnel and keeps it up. It returns nil when the operator
// closed it, and an error when it gave up.
func Connect(ctx context.Context, d *config.Destination, run Runner, r Retry) error {
	// Retry is exported and built by hand — by the command, by --no-retry and
	// by every test — so the two fields that can make it unusable are settled
	// here rather than left to a nil call and a comparison against a negative.
	if r.Wait == nil {
		r.Wait = wait
	}
	r.Max = max(r.Max, 0)

	attempts, launches := 0, 0
	for {
		reconnected := launches > 0
		res := run(ctx, d, func() {
			if r.OnUp != nil {
				r.OnUp(reconnected)
			}
		})
		launches++

		// The context is checked first and on its own. It is the one signal
		// that covers every way of asking tunny to stop, including the ones no
		// exit code can express: a SIGTERM while ssh was healthy, or a Ctrl-C
		// that landed during the backoff rather than during an attempt.
		if ctx.Err() != nil || res.Outcome == OutcomeInterrupted {
			return nil
		}
		// Held long enough to count as a working tunnel: whatever just
		// happened starts a fresh episode. The counter is therefore per
		// outage, not per session — a tunnel left open all day survives five
		// network changes, while a destination that is really down gives up
		// in three quick tries.
		if res.Lived >= r.StableAfter {
			attempts = 0
		}
		if line, ok := Hopeless(res.Stderr); ok {
			return fmt.Errorf("%s: %s", d.Name, line)
		}

		attempts++
		if attempts > r.Max {
			if r.Max == 0 {
				return fmt.Errorf("%s: connection failed", d.Name)
			}
			return fmt.Errorf("%s: gave up after %d reconnection attempts", d.Name, r.Max)
		}
		pause := backoff(attempts)
		if r.Notify != nil {
			r.Notify(attempts, r.Max, pause)
		}
		if err := r.Wait(ctx, pause); err != nil {
			return nil // cut short on purpose, which is not a failure
		}
	}
}
