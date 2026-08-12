package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/MorganKryze/tuna/src/internal/config"
	"github.com/MorganKryze/tuna/src/internal/tunnel"
)

// fakeSSH puts a shell script where tuna looks for ssh. The whole point of
// sshPath being a variable is this: the process handling, the signals and the
// stderr capture are the half of "Ctrl-C never relaunches" that no unit test
// could reach, and a fake binary reaches all of it with no server, no network
// and no dependency.
func fakeSSH(t *testing.T, body string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "ssh")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := sshPath
	sshPath = p
	t.Cleanup(func() { sshPath = old })
}

func dest() *config.Destination {
	return &config.Destination{
		Name:    "a",
		Host:    "nowhere.invalid",
		Forward: []config.Forward{{Local: 1, To: "127.0.0.1:1"}},
	}
}

// alive reports whether a pid still exists. Signal 0 asks the kernel without
// sending anything.
func alive(pid int) bool { return syscall.Kill(pid, 0) == nil }

// The defect this test exists for: a SIGTERM to tuna used to kill tuna alone.
// ssh was reparented to init and kept holding the forwarded ports, so the next
// run reported them busy and pointed the operator at lsof to find tuna's own
// child.
//
// The fake ignores SIGINT on purpose. A Ctrl-C typed at a terminal reaches the
// whole foreground group and would have killed it either way; what has to work
// here is tuna taking the child down itself.
func TestCancellingTheContextTakesTheChildWithIt(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	t.Setenv("FAKE_SSH_PIDFILE", pidFile)
	// exec, so the script *becomes* the sleep. Without it the shell leaves a
	// grandchild holding the stderr pipe, which is the very thing WaitDelay
	// exists to survive and which would make this test measure that instead.
	fakeSSH(t, `trap "" INT; echo $$ > "$FAKE_SSH_PIDFILE"; exec sleep 20`)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan tunnel.Result, 1)
	go func() { done <- sshRunner(ctx, dest(), func() {}) }()

	var pid int
	for range 100 {
		if b, err := os.ReadFile(pidFile); err == nil && len(b) > 1 {
			pid, _ = parsePID(strings.TrimSpace(string(b)))
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("the fake ssh never started")
	}

	cancel()
	select {
	case res := <-done:
		if res.Outcome != tunnel.OutcomeInterrupted {
			t.Errorf("a cancelled context is a deliberate close, got outcome %v", res.Outcome)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("sshRunner never returned after the context was cancelled")
	}

	for range 100 { // the kernel reaps on its own schedule
		if !alive(pid) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	t.Fatalf("ssh (pid %d) survived tuna: it is an orphan holding the local ports", pid)
}

// The other half: ssh dying on its own is a failure worth retrying, and its
// last words have to reach the classifier intact.
func TestAChildThatDiesOnItsOwnIsAFailureAndItsStderrSurvives(t *testing.T) {
	fakeSSH(t, `echo "ssh: connect to host nowhere.invalid port 22: Connection refused" >&2; exit 255`)

	res := sshRunner(t.Context(), dest(), func() {})
	if res.Outcome != tunnel.OutcomeFailed {
		t.Errorf("want OutcomeFailed, got %v", res.Outcome)
	}
	if !strings.Contains(res.Stderr, "Connection refused") {
		t.Errorf("ssh's own words have to reach Hopeless, got %q", res.Stderr)
	}
	if _, hopeless := tunnel.Hopeless(res.Stderr); hopeless {
		t.Error("a refused connection is retryable, not hopeless")
	}
}

// A child killed by SIGTERM is somebody asking tuna to stop, and must not
// start the loop again. A child killed by SIGKILL is the system taking it
// away, which is worth another try.
func TestHowTheChildDiedDecidesWhetherToRetry(t *testing.T) {
	for _, c := range []struct {
		name string
		body string
		want tunnel.Outcome
	}{
		{"SIGTERM to itself", `kill -TERM $$; exec sleep 5`, tunnel.OutcomeInterrupted},
		{"SIGKILL to itself", `kill -KILL $$; exec sleep 5`, tunnel.OutcomeFailed},
		{"a plain non-zero exit", `exit 3`, tunnel.OutcomeFailed},
	} {
		t.Run(c.name, func(t *testing.T) {
			fakeSSH(t, c.body)
			if got := sshRunner(t.Context(), dest(), func() {}).Outcome; got != c.want {
				t.Errorf("want %v, got %v", c.want, got)
			}
		})
	}
}

func parsePID(s string) (int, error) {
	var n int
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}
