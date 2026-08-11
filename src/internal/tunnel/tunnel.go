// Package tunnel keeps an ssh tunnel up. It is the core of the program and
// the only part with a state machine, so it is the most tested.
//
// Connect knows nothing about os/exec, signals or the clock: it calls a
// Runner, which reports how long the tunnel lived and how it ended. That is
// what lets the whole reconnection policy be tested in microseconds instead
// of seven seconds, and it is why the process handling lives in cmd/tuna
// rather than here.
package tunnel

import (
	"fmt"
	"time"

	"github.com/MorganKryze/tuna/src/internal/config"
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
type Runner func(d *config.Destination, up func()) Result

type Retry struct {
	Max         int           // reconnection attempts per episode
	StableAfter time.Duration // held this long = the episode is over
	Sleep       func(time.Duration)
	// Notify is told about each retry before it is waited out, and OnUp when
	// the tunnel comes up — with reconnected set when it is coming back
	// rather than starting. ssh says nothing either way, so without these a
	// dropped tunnel, a hung one and a recovered one all look alike from the
	// terminal. Both optional: nil simply stays quiet.
	Notify func(attempt, max int, wait time.Duration)
	OnUp   func(reconnected bool)
}

func DefaultRetry() Retry {
	return Retry{Max: 3, StableAfter: 30 * time.Second, Sleep: time.Sleep}
}

func backoff(attempt int) time.Duration {
	return time.Second << (attempt - 1) // 1s, 2s, 4s
}

// Connect runs the tunnel and keeps it up. It returns nil when the operator
// closed it, and an error when it gave up.
func Connect(d *config.Destination, run Runner, r Retry) error {
	attempts, launches := 0, 0
	for {
		reconnected := launches > 0
		res := run(d, func() {
			if r.OnUp != nil {
				r.OnUp(reconnected)
			}
		})
		launches++

		if res.Outcome == OutcomeInterrupted {
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
			return fmt.Errorf("%s : %s", d.Name, line)
		}

		attempts++
		if attempts > r.Max {
			if r.Max == 0 {
				return fmt.Errorf("%s : connexion échouée", d.Name)
			}
			return fmt.Errorf("%s : abandon après %d tentatives de reconnexion", d.Name, r.Max)
		}
		wait := backoff(attempts)
		if r.Notify != nil {
			r.Notify(attempts, r.Max, wait)
		}
		r.Sleep(wait)
	}
}
