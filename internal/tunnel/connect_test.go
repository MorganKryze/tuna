// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

package tunnel

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/MorganKryze/tunny/internal/config"
)

// fake records how many times it was called and answers from a script. The
// last entry repeats, so a one-entry script means "always this".
type fake struct {
	script []Result
	calls  int
}

func (f *fake) run(context.Context, *config.Destination, func()) Result {
	r := f.script[min(f.calls, len(f.script)-1)]
	f.calls++
	return r
}

func testRetry() Retry {
	return Retry{Max: 3, StableAfter: 30 * time.Second, Wait: func(context.Context, time.Duration) error { return nil }}
}

func TestGivesUpAfterThreeQuickFailures(t *testing.T) {
	f := &fake{script: []Result{{Lived: 2 * time.Second, Outcome: OutcomeFailed}}}
	if err := Connect(t.Context(), &config.Destination{Name: "a"}, f.run, testRetry()); err == nil {
		t.Fatal("three quick failures have to end in giving up")
	}
	// One initial attempt plus three retries.
	if f.calls != 4 {
		t.Fatalf("want 4 launches, got %d", f.calls)
	}
}

// The counter is per episode, not per session: a tunnel held open all day
// through five network changes must never hit the ceiling.
func TestAStableTunnelResetsTheCounter(t *testing.T) {
	f := &fake{script: []Result{
		{Lived: 40 * time.Second, Outcome: OutcomeFailed}, // held, then dropped
		{Lived: 40 * time.Second, Outcome: OutcomeFailed}, // held again
		{Lived: 40 * time.Second, Outcome: OutcomeFailed},
		{Lived: 40 * time.Second, Outcome: OutcomeFailed},
		{Lived: 40 * time.Second, Outcome: OutcomeFailed},
		{Lived: 2 * time.Second, Outcome: OutcomeFailed}, // now it is really down
		{Lived: 2 * time.Second, Outcome: OutcomeFailed},
		{Lived: 2 * time.Second, Outcome: OutcomeFailed},
	}}
	if err := Connect(t.Context(), &config.Destination{Name: "a"}, f.run, testRetry()); err == nil {
		t.Fatal("three quick failures after the recoveries have to give up")
	}
	// Five held attempts, then three quick ones. Not four: the fifth reset
	// left the counter at 1, because relaunching after a stable tunnel is
	// itself the first attempt of the new episode.
	if f.calls != 8 {
		t.Fatalf("want 8 launches (5 stable, 3 quick), got %d", f.calls)
	}
}

// The threshold is inclusive: exactly StableAfter counts as having held.
//
// Three stable runs then three quick ones, because one of each proves
// nothing — with a single stable run both an inclusive and an exclusive
// threshold end on four launches. Three resets separate them: inclusive
// gives six launches, exclusive gives four.
func TestExactlyStableAfterCounts(t *testing.T) {
	f := &fake{script: []Result{
		{Lived: 30 * time.Second, Outcome: OutcomeFailed},
		{Lived: 30 * time.Second, Outcome: OutcomeFailed},
		{Lived: 30 * time.Second, Outcome: OutcomeFailed},
		{Lived: 2 * time.Second, Outcome: OutcomeFailed},
	}}
	_ = Connect(t.Context(), &config.Destination{Name: "a"}, f.run, testRetry())
	if f.calls != 6 {
		t.Fatalf("exactly 30s has to reset the counter: want 6 launches, got %d", f.calls)
	}
}

// And a hair under does not. Without this, StableAfter could be read as
// "roughly thirty seconds" and drift on the next edit.
func TestJustUnderStableAfterDoesNotCount(t *testing.T) {
	f := &fake{script: []Result{{Lived: 30*time.Second - time.Millisecond, Outcome: OutcomeFailed}}}
	_ = Connect(t.Context(), &config.Destination{Name: "a"}, f.run, testRetry())
	if f.calls != 4 {
		t.Fatalf("29.999s must reset nothing: want 4 launches, got %d", f.calls)
	}
}

func TestCtrlCNeverRelaunches(t *testing.T) {
	f := &fake{script: []Result{{Lived: time.Second, Outcome: OutcomeInterrupted}}}
	if err := Connect(t.Context(), &config.Destination{Name: "a"}, f.run, testRetry()); err != nil {
		t.Fatalf("a deliberate close is not an error: %v", err)
	}
	if f.calls != 1 {
		t.Fatalf("Ctrl-C has to return at once, got %d launches", f.calls)
	}
}

// Ctrl-C wins over everything else, including a stderr that would otherwise
// be classified: a tunnel you deliberately closed is closed.
func TestCtrlCWinsOverAHopelessStderr(t *testing.T) {
	f := &fake{script: []Result{{
		Lived:   time.Second,
		Stderr:  "bind [127.0.0.1]:8200: Address already in use",
		Outcome: OutcomeInterrupted,
	}}}
	if err := Connect(t.Context(), &config.Destination{Name: "a"}, f.run, testRetry()); err != nil {
		t.Fatalf("Ctrl-C has to win and return without an error, got: %v", err)
	}
	if f.calls != 1 {
		t.Fatalf("want 1 launch, got %d", f.calls)
	}
}

func TestHopelessFailuresGiveUpAtOnce(t *testing.T) {
	cases := map[string]string{
		"bind [127.0.0.1]:8200: Address already in use":  "8200",
		"debian@10.0.0.5: Permission denied (publickey)": "Permission denied",
		"ssh: Could not resolve hostname nope":           "Could not resolve hostname",
	}
	for stderr, want := range cases {
		t.Run(want, func(t *testing.T) {
			f := &fake{script: []Result{{Lived: time.Second, Stderr: stderr, Outcome: OutcomeFailed}}}
			err := Connect(t.Context(), &config.Destination{Name: "a"}, f.run, testRetry())
			if err == nil {
				t.Fatal("a hopeless failure has to be an error")
			}
			if f.calls != 1 {
				t.Fatalf("want 1 launch, got %d", f.calls)
			}
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("the error has to quote ssh (%q), got: %v", want, err)
			}
		})
	}
}

// A retryable failure is the whole point of the loop: the host being down
// must burn attempts rather than abandon on the first try.
func TestARetryableFailureUsesEveryAttempt(t *testing.T) {
	f := &fake{script: []Result{{
		Lived:   time.Second,
		Stderr:  "ssh: connect to host mon-hote port 22: Connection refused",
		Outcome: OutcomeFailed,
	}}}
	if err := Connect(t.Context(), &config.Destination{Name: "a"}, f.run, testRetry()); err == nil {
		t.Fatal("the failure has to surface in the end")
	}
	if f.calls != 4 {
		t.Fatalf("want 4 launches, got %d", f.calls)
	}
}

// --no-retry is Max: 0, and it must still be one attempt, not zero.
func TestNoRetryStillConnectsOnce(t *testing.T) {
	f := &fake{script: []Result{{Lived: time.Second, Outcome: OutcomeFailed}}}
	r := testRetry()
	r.Max = 0
	err := Connect(t.Context(), &config.Destination{Name: "a"}, f.run, r)
	if err == nil {
		t.Fatal("the failure has to surface")
	}
	if f.calls != 1 {
		t.Fatalf("want 1 launch, got %d", f.calls)
	}
	// With no retries there was no "abandon after N attempts" to speak of,
	// and saying so would read as though tunny had tried three times.
	if strings.Contains(err.Error(), "attempts") {
		t.Fatalf("--no-retry must not talk about attempts: %v", err)
	}
}

// --no-retry still returns immediately when the tunnel was closed on purpose.
func TestNoRetryHonoursCtrlC(t *testing.T) {
	f := &fake{script: []Result{{Lived: time.Second, Outcome: OutcomeInterrupted}}}
	r := testRetry()
	r.Max = 0
	if err := Connect(t.Context(), &config.Destination{Name: "a"}, f.run, r); err != nil {
		t.Fatalf("Ctrl-C is not an error, reconnection or not: %v", err)
	}
}

func TestBackoffGrows(t *testing.T) {
	var slept []time.Duration
	f := &fake{script: []Result{{Lived: time.Second, Outcome: OutcomeFailed}}}
	r := testRetry()
	r.Wait = func(_ context.Context, d time.Duration) error { slept = append(slept, d); return nil }
	_ = Connect(t.Context(), &config.Destination{Name: "a"}, f.run, r)

	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}
	if len(slept) != len(want) {
		t.Fatalf("want %d pauses, got %v", len(want), slept)
	}
	for i := range want {
		if slept[i] != want[i] {
			t.Fatalf("pause %d: want %v, got %v", i+1, want[i], slept[i])
		}
	}
}

// The shipped policy is what the README documents, so it gets a check of its
// own rather than living only inside DefaultRetry's body.
func TestDefaultRetryIsWhatTheReadmeSays(t *testing.T) {
	r := DefaultRetry()
	if r.Max != 3 {
		t.Errorf("want 3 attempts, got %d", r.Max)
	}
	if r.StableAfter != 30*time.Second {
		t.Errorf("want 30s of stability, got %v", r.StableAfter)
	}
	if r.Wait == nil {
		t.Error("the default Wait must not be nil: Connect calls it without checking")
	}
}

// OnUp separates "it came up" from "it came back", because those two need
// different words on screen and Connect is the only place that knows which
// one just happened.
func TestOnUpTellsAFirstStartFromAComeback(t *testing.T) {
	var seen []bool
	f := &fake{script: []Result{{Lived: 2 * time.Second, Outcome: OutcomeFailed}}}
	r := testRetry()
	r.OnUp = func(reconnected bool) { seen = append(seen, reconnected) }

	// Every launch reports; only the first is not a reconnection.
	_ = Connect(t.Context(), &config.Destination{Name: "a"}, func(ctx context.Context, d *config.Destination, up func()) Result {
		up()
		return f.run(ctx, d, up)
	}, r)

	if want := []bool{false, true, true, true}; len(seen) != len(want) {
		t.Fatalf("want %d announcements, got %v", len(want), seen)
	} else {
		for i := range want {
			if seen[i] != want[i] {
				t.Fatalf("announcement %d: want reconnected=%v, got %v", i+1, want[i], seen[i])
			}
		}
	}
}

// A tunnel that never comes up must not claim it did, and a nil OnUp must not
// crash the loop.
func TestOnUpIsOnlyCalledByTheRunner(t *testing.T) {
	called := false
	f := &fake{script: []Result{{Lived: time.Second, Outcome: OutcomeFailed}}}
	r := testRetry()
	r.OnUp = func(bool) { called = true }
	_ = Connect(t.Context(), &config.Destination{Name: "a"}, f.run, r)
	if called {
		t.Error("a runner that never calls up must announce nothing")
	}

	r.OnUp = nil
	_ = Connect(t.Context(), &config.Destination{Name: "a"}, func(ctx context.Context, d *config.Destination, up func()) Result {
		up() // nil OnUp: the closure Connect passes has to absorb this
		return f.run(ctx, d, up)
	}, r)
}

// Notify's arguments used to be untested: swapping them read as "retrying 3/1
// in 1s" on screen and no test noticed.
func TestNotifyIsToldWhichAttemptAndHowLong(t *testing.T) {
	type notice struct {
		attempt, max int
		wait         time.Duration
	}
	var seen []notice

	f := &fake{script: []Result{{Lived: time.Second, Outcome: OutcomeFailed}}}
	r := testRetry()
	r.Notify = func(attempt, total int, wait time.Duration) {
		seen = append(seen, notice{attempt, total, wait})
	}
	_ = Connect(t.Context(), &config.Destination{Name: "a"}, f.run, r)

	want := []notice{{1, 3, time.Second}, {2, 3, 2 * time.Second}, {3, 3, 4 * time.Second}}
	if len(seen) != len(want) {
		t.Fatalf("want %d notices, got %v", len(want), seen)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Errorf("notice %d: want %+v, got %+v", i+1, want[i], seen[i])
		}
	}
}

// A cancelled context stops the loop wherever it is, including in the middle
// of a backoff. Most of a failing episode is spent waiting, and that is when
// somebody reaches for Ctrl-C.
func TestCancellingDuringTheBackoffStopsTheLoop(t *testing.T) {
	f := &fake{script: []Result{{Lived: time.Second, Outcome: OutcomeFailed}}}
	r := testRetry()
	r.Wait = func(context.Context, time.Duration) error { return context.Canceled }

	if err := Connect(t.Context(), &config.Destination{Name: "a"}, f.run, r); err != nil {
		t.Fatalf("being asked to stop is not a failure: %v", err)
	}
	if f.calls != 1 {
		t.Fatalf("want 1 launch before the interrupted wait, got %d", f.calls)
	}
}

// Retry.Max is exported and nothing bounds it. Doubling a second past the
// width of an int64 gives a negative duration, and a timer set to a negative
// duration fires at once: the backoff would become a spin against a host that
// is already down, which is the one thing a backoff exists to prevent.
func TestTheBackoffStopsDoubling(t *testing.T) {
	for attempt := 1; attempt <= 200; attempt++ {
		d := backoff(attempt)
		if d <= 0 {
			t.Fatalf("attempt %d: backoff %v, which a timer fires immediately", attempt, d)
		}
		if d > maxBackoff {
			t.Fatalf("attempt %d: backoff %v is over the %v ceiling", attempt, d, maxBackoff)
		}
	}
	if got := backoff(0); got <= 0 {
		t.Errorf("backoff(0) = %v; a shift by a negative amount panics", got)
	}
}

// A hand-built Retry with no Wait used to be a nil call, and a negative Max a
// comparison that never came true. Neither is reachable from the command
// today, which is exactly why they are worth pinning: the struct is exported.
func TestAHandBuiltRetryIsUsable(t *testing.T) {
	f := &fake{script: []Result{{Lived: time.Second, Outcome: OutcomeFailed}}}
	err := Connect(t.Context(), &config.Destination{Name: "a"}, f.run, Retry{Max: -1})
	if err == nil {
		t.Fatal("a negative Max is no retries at all, so this has to give up")
	}
	// Unclamped it reads "gave up after -1 reconnection attempts", which is
	// the count it did not make of the retries it never tried.
	if got := err.Error(); !strings.Contains(got, "connection failed") {
		t.Errorf("a negative Max has to read like no retries: %q", got)
	}
	if f.calls != 1 {
		t.Errorf("want the one attempt, got %d", f.calls)
	}

	// And with a retry to make, so the nil Wait is actually reached. The
	// deadline is what keeps this from being a real one-second backoff: the
	// default Wait returns as soon as the context is done, which is the whole
	// reason it takes one.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	f = &fake{script: []Result{{Lived: time.Second, Outcome: OutcomeFailed}}}
	if err := Connect(ctx, &config.Destination{Name: "a"}, f.run, Retry{Max: 1}); err != nil {
		t.Errorf("a backoff cut short is not a failure, got %v", err)
	}
}
