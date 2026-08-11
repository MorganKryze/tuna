package main

import (
	"strings"
	"testing"
	"time"
)

func TestSSHArgsCarryEveryField(t *testing.T) {
	got := strings.Join(SSHArgs(&Destination{
		Name: "vm",
		Host: "debian@10.0.0.5",
		Port: 22022,
		Jump: "mon-hote",
		Forward: []Forward{
			{Local: 8201, To: "127.0.0.1:8200"},
			{Local: 9120, To: "10.0.0.5:9120"},
		},
	}), " ")

	for _, attendu := range []string{
		"-N",
		"-o ExitOnForwardFailure=yes",
		"-J mon-hote",
		"-p 22022",
		"-L 8201:127.0.0.1:8200",
		"-L 9120:10.0.0.5:9120",
	} {
		if !strings.Contains(got, attendu) {
			t.Errorf("args ssh sans %q : %s", attendu, got)
		}
	}
	// The host is what ssh reads as its target: anything after it is taken
	// as a remote command, which -N forbids.
	if !strings.HasSuffix(got, "debian@10.0.0.5") {
		t.Errorf("l'hôte doit être le dernier argument : %s", got)
	}
}

func TestSSHArgsOmitWhatIsNotSet(t *testing.T) {
	got := strings.Join(SSHArgs(&Destination{
		Host:    "mon-hote",
		Forward: []Forward{{Local: 9090, To: "127.0.0.1:9090"}},
	}), " ")
	if strings.Contains(got, "-J") || strings.Contains(got, "-p") {
		t.Errorf("un champ vide ne doit produire aucun drapeau : %s", got)
	}
}

// fake records how many times it was called and answers from a script.
type fake struct {
	script []Result
	calls  int
}

func (f *fake) run(*Destination) Result {
	r := f.script[min(f.calls, len(f.script)-1)]
	f.calls++
	return r
}

func testRetry() Retry {
	return Retry{Max: 3, StableAfter: 30 * time.Second, Sleep: func(time.Duration) {}}
}

func TestGivesUpAfterThreeQuickFailures(t *testing.T) {
	f := &fake{script: []Result{{Lived: 2 * time.Second, Outcome: OutcomeFailed}}}
	if err := Connect(&Destination{Name: "a"}, f.run, testRetry()); err == nil {
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
	if err := Connect(&Destination{Name: "a"}, f.run, testRetry()); err == nil {
		t.Fatal("trois échecs rapprochés après les reprises doivent abandonner")
	}
	// Five held attempts, then three quick ones. Not four: the fifth reset
	// left the counter at 1, because relaunching after a stable tunnel is
	// itself the first attempt of the new episode.
	if f.calls != 8 {
		t.Fatalf("attendu 8 lancements (5 stables + 3 rapprochés), obtenu %d", f.calls)
	}
}

func TestCtrlCNeverRelaunches(t *testing.T) {
	f := &fake{script: []Result{{Lived: time.Second, Outcome: OutcomeInterrupted}}}
	if err := Connect(&Destination{Name: "a"}, f.run, testRetry()); err != nil {
		t.Fatalf("une fermeture volontaire n'est pas une erreur : %v", err)
	}
	if f.calls != 1 {
		t.Fatalf("Ctrl-C doit rendre la main immédiatement, obtenu %d lancements", f.calls)
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
			err := Connect(&Destination{Name: "a"}, f.run, testRetry())
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

// --no-retry is Max: 0, and it must still be one attempt, not zero.
func TestNoRetryStillConnectsOnce(t *testing.T) {
	f := &fake{script: []Result{{Lived: time.Second, Outcome: OutcomeFailed}}}
	r := testRetry()
	r.Max = 0
	if err := Connect(&Destination{Name: "a"}, f.run, r); err == nil {
		t.Fatal("l'échec doit remonter")
	}
	if f.calls != 1 {
		t.Fatalf("attendu 1 lancement, obtenu %d", f.calls)
	}
}

func TestBackoffGrows(t *testing.T) {
	var slept []time.Duration
	f := &fake{script: []Result{{Lived: time.Second, Outcome: OutcomeFailed}}}
	r := testRetry()
	r.Sleep = func(d time.Duration) { slept = append(slept, d) }
	_ = Connect(&Destination{Name: "a"}, f.run, r)

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
