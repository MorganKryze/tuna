package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/MorganKryze/tuna/src/internal/config"
	"github.com/MorganKryze/tuna/src/internal/tunnel"
)

// upAfter is how long ssh has to survive before the tunnel counts as open.
const upAfter = 3 * time.Second

// sshPath is a variable rather than a literal so a test can point tuna at a
// fake ssh on a controlled PATH. Nothing sets it in production; it is the seam
// that makes the process and signal handling testable without a real server.
var sshPath = "ssh"

// sshRunner is the only code in the program that touches processes and
// signals. It reports how long ssh lived and how it ended; tunnel.Connect
// decides what that means. Keeping the two apart is what lets the whole
// reconnection policy be tested without spawning anything.
func sshRunner(ctx context.Context, d *config.Destination, up func()) tunnel.Result {
	cmd := exec.CommandContext(ctx, sshPath, tunnel.SSHArgs(d)...)
	cmd.Stdin = os.Stdin // the host-key prompt and any passphrase need the TTY
	cmd.Stdout = os.Stdout

	// What a cancelled context does to the child. Without this, a SIGTERM to
	// tuna killed tuna alone: ssh was reparented to init and kept holding the
	// forwarded ports, so the next run reported them busy and told the
	// operator to go find the culprit with lsof. The culprit was tuna's own
	// child.
	//
	// SIGTERM to the process rather than to the group, on purpose. Putting ssh
	// in its own group would stop the terminal's own Ctrl-C from reaching it,
	// and would make it a background group that cannot read the TTY — which is
	// where the host-key prompt and any passphrase have to arrive.
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }

	// A -J destination runs a ProxyCommand child that inherits this stderr
	// pipe, and Wait does not return until every writer has closed it. A
	// grandchild outliving its parent would hang tuna with no output and no
	// way out. WaitDelay gives up on the pipe instead of on the program.
	cmd.WaitDelay = 3 * time.Second

	// stderr is both shown and read: shown because ssh's own words are the
	// best diagnostic there is, read because three of them mean retrying is
	// pointless.
	var captured strings.Builder
	cmd.Stderr = &teeWriter{to: os.Stderr, into: &captured}

	start := time.Now()

	// "Still alive after a few seconds" is the signal that the tunnel came
	// up. It is a proxy, and an honest one: -o ExitOnForwardFailure=yes makes
	// ssh exit within a moment when a forward cannot be bound, so surviving
	// this long means the forwards were accepted.
	//
	// ponytail: the exact answer would be to check that something is
	// listening on the local ports, and both ways of asking are worse than
	// the guess. Binding them races with ssh's own bind — and losing that
	// race causes the very "address already in use" this would be reporting
	// on. Dialling them opens a real connection through the tunnel to the
	// service on the far side, which is a side effect a status message has
	// no business having.
	if err := cmd.Start(); err != nil {
		return tunnel.Result{Stderr: err.Error(), Outcome: tunnel.OutcomeFailed}
	}
	upTimer := time.AfterFunc(upAfter, up)
	defer upTimer.Stop()

	err := cmd.Wait()
	lived := time.Since(start)

	// Two ways to learn it was deliberate, and both are needed. The context
	// covers a signal tuna received; signaled() covers the terminal delivering
	// Ctrl-C straight to the foreground group, where ssh dies before tuna has
	// looked at anything.
	outcome := tunnel.OutcomeFailed
	if ctx.Err() != nil || signaled(err) {
		outcome = tunnel.OutcomeInterrupted
	}
	return tunnel.Result{Lived: lived, Stderr: captured.String(), Outcome: outcome}
}

// signaled reports whether the process was killed by SIGINT or SIGTERM
// rather than exiting on its own. This is never inferred from an exit code:
// getting it wrong gives a tunnel that relaunches itself every time you try
// to close it, which is the worst thing this program could do.
func signaled(err error) bool {
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		return false
	}
	ws, ok := ee.Sys().(syscall.WaitStatus)
	if !ok || !ws.Signaled() {
		return false
	}
	return ws.Signal() == syscall.SIGINT || ws.Signal() == syscall.SIGTERM
}

// teeWriter shows output and keeps a copy.
type teeWriter struct {
	to   io.Writer
	into *strings.Builder
}

func (t *teeWriter) Write(p []byte) (int, error) {
	t.into.Write(p)
	return t.to.Write(p)
}
