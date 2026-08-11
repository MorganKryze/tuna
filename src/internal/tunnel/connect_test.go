package tunnel

import (
	"strings"
	"testing"
	"time"

	"github.com/MorganKryze/tuna/src/internal/config"
)

// fake records how many times it was called and answers from a script. The
// last entry repeats, so a one-entry script means "always this".
type fake struct {
	script []Result
	calls  int
}

func (f *fake) run(*config.Destination, func()) Result {
	r := f.script[min(f.calls, len(f.script)-1)]
	f.calls++
	return r
}

func testRetry() Retry {
	return Retry{Max: 3, StableAfter: 30 * time.Second, Sleep: func(time.Duration) {}}
}

func TestGivesUpAfterThreeQuickFailures(t *testing.T) {
	f := &fake{script: []Result{{Lived: 2 * time.Second, Outcome: OutcomeFailed}}}
	if err := Connect(&config.Destination{Name: "a"}, f.run, testRetry()); err == nil {
		t.Fatal("trois échecs rapprochés doivent finir par un abandon")
	}
	// One initial attempt plus three retries.
	if f.calls != 4 {
		t.Fatalf("attendu 4 lancements, obtenu %d", f.calls)
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
	if err := Connect(&config.Destination{Name: "a"}, f.run, testRetry()); err == nil {
		t.Fatal("trois échecs rapprochés après les reprises doivent abandonner")
	}
	// Five held attempts, then three quick ones. Not four: the fifth reset
	// left the counter at 1, because relaunching after a stable tunnel is
	// itself the first attempt of the new episode.
	if f.calls != 8 {
		t.Fatalf("attendu 8 lancements (5 stables + 3 rapprochés), obtenu %d", f.calls)
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
	_ = Connect(&config.Destination{Name: "a"}, f.run, testRetry())
	if f.calls != 6 {
		t.Fatalf("30s pile doit remettre le compteur à zéro : attendu 6 lancements, obtenu %d", f.calls)
	}
}

// And a hair under does not. Without this, StableAfter could be read as
// "roughly thirty seconds" and drift on the next edit.
func TestJustUnderStableAfterDoesNotCount(t *testing.T) {
	f := &fake{script: []Result{{Lived: 30*time.Second - time.Millisecond, Outcome: OutcomeFailed}}}
	_ = Connect(&config.Destination{Name: "a"}, f.run, testRetry())
	if f.calls != 4 {
		t.Fatalf("29,999s ne doit rien remettre à zéro : attendu 4 lancements, obtenu %d", f.calls)
	}
}

func TestCtrlCNeverRelaunches(t *testing.T) {
	f := &fake{script: []Result{{Lived: time.Second, Outcome: OutcomeInterrupted}}}
	if err := Connect(&config.Destination{Name: "a"}, f.run, testRetry()); err != nil {
		t.Fatalf("une fermeture volontaire n'est pas une erreur : %v", err)
	}
	if f.calls != 1 {
		t.Fatalf("Ctrl-C doit rendre la main immédiatement, obtenu %d lancements", f.calls)
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
	if err := Connect(&config.Destination{Name: "a"}, f.run, testRetry()); err != nil {
		t.Fatalf("Ctrl-C doit primer et sortir sans erreur, obtenu : %v", err)
	}
	if f.calls != 1 {
		t.Fatalf("attendu 1 lancement, obtenu %d", f.calls)
	}
}

func TestHopelessFailuresGiveUpAtOnce(t *testing.T) {
	cases := map[string]string{
		"bind [127.0.0.1]:8200: Address already in use":  "8200",
		"debian@10.0.0.5: Permission denied (publickey)": "Permission denied",
		"ssh: Could not resolve hostname nope":           "Could not resolve hostname",
	}
	for stderr, attendu := range cases {
		t.Run(attendu, func(t *testing.T) {
			f := &fake{script: []Result{{Lived: time.Second, Stderr: stderr, Outcome: OutcomeFailed}}}
			err := Connect(&config.Destination{Name: "a"}, f.run, testRetry())
			if err == nil {
				t.Fatal("un échec sans espoir doit être une erreur")
			}
			if f.calls != 1 {
				t.Fatalf("attendu 1 lancement, obtenu %d", f.calls)
			}
			if !strings.Contains(err.Error(), attendu) {
				t.Fatalf("l'erreur doit citer ssh (%q), obtenu : %v", attendu, err)
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
	if err := Connect(&config.Destination{Name: "a"}, f.run, testRetry()); err == nil {
		t.Fatal("l'échec doit finir par remonter")
	}
	if f.calls != 4 {
		t.Fatalf("attendu 4 lancements, obtenu %d", f.calls)
	}
}

// --no-retry is Max: 0, and it must still be one attempt, not zero.
func TestNoRetryStillConnectsOnce(t *testing.T) {
	f := &fake{script: []Result{{Lived: time.Second, Outcome: OutcomeFailed}}}
	r := testRetry()
	r.Max = 0
	err := Connect(&config.Destination{Name: "a"}, f.run, r)
	if err == nil {
		t.Fatal("l'échec doit remonter")
	}
	if f.calls != 1 {
		t.Fatalf("attendu 1 lancement, obtenu %d", f.calls)
	}
	// With no retries there was no "abandon after N attempts" to speak of,
	// and saying so would read as though tuna had tried three times.
	if strings.Contains(err.Error(), "tentatives") {
		t.Fatalf("--no-retry ne doit pas parler de tentatives : %v", err)
	}
}

// --no-retry still returns immediately when the tunnel was closed on purpose.
func TestNoRetryHonoursCtrlC(t *testing.T) {
	f := &fake{script: []Result{{Lived: time.Second, Outcome: OutcomeInterrupted}}}
	r := testRetry()
	r.Max = 0
	if err := Connect(&config.Destination{Name: "a"}, f.run, r); err != nil {
		t.Fatalf("Ctrl-C n'est pas une erreur, même sans reconnexion : %v", err)
	}
}

func TestBackoffGrows(t *testing.T) {
	var slept []time.Duration
	f := &fake{script: []Result{{Lived: time.Second, Outcome: OutcomeFailed}}}
	r := testRetry()
	r.Sleep = func(d time.Duration) { slept = append(slept, d) }
	_ = Connect(&config.Destination{Name: "a"}, f.run, r)

	attendu := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}
	if len(slept) != len(attendu) {
		t.Fatalf("attendu %d pauses, obtenu %v", len(attendu), slept)
	}
	for i := range attendu {
		if slept[i] != attendu[i] {
			t.Fatalf("pause n°%d : attendu %v, obtenu %v", i+1, attendu[i], slept[i])
		}
	}
}

// The shipped policy is what the README documents, so it gets a check of its
// own rather than living only inside DefaultRetry's body.
func TestDefaultRetryIsWhatTheReadmeSays(t *testing.T) {
	r := DefaultRetry()
	if r.Max != 3 {
		t.Errorf("attendu 3 tentatives, obtenu %d", r.Max)
	}
	if r.StableAfter != 30*time.Second {
		t.Errorf("attendu 30s de stabilité, obtenu %v", r.StableAfter)
	}
	if r.Sleep == nil {
		t.Error("Sleep par défaut ne doit pas être nil : Connect l'appelle sans le vérifier")
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
	_ = Connect(&config.Destination{Name: "a"}, func(d *config.Destination, up func()) Result {
		up()
		return f.run(d, up)
	}, r)

	if want := []bool{false, true, true, true}; len(seen) != len(want) {
		t.Fatalf("attendu %d annonces, obtenu %v", len(want), seen)
	} else {
		for i := range want {
			if seen[i] != want[i] {
				t.Fatalf("annonce n°%d : attendu reconnected=%v, obtenu %v", i+1, want[i], seen[i])
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
	_ = Connect(&config.Destination{Name: "a"}, f.run, r)
	if called {
		t.Error("un runner qui n'appelle jamais up ne doit rien annoncer")
	}

	r.OnUp = nil
	_ = Connect(&config.Destination{Name: "a"}, func(d *config.Destination, up func()) Result {
		up() // nil OnUp: the closure Connect passes has to absorb this
		return f.run(d, up)
	}, r)
}
