package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
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
func sshRunner(d *config.Destination, up func()) tunnel.Result {
	cmd := exec.Command(sshPath, tunnel.SSHArgs(d)...)
	cmd.Stdin = os.Stdin // the host-key prompt and any passphrase need the TTY
	cmd.Stdout = os.Stdout

	// stderr is both shown and read: shown because ssh's own words are the
	// best diagnostic there is, read because three of them mean retrying is
	// pointless.
	var captured strings.Builder
	cmd.Stderr = &teeWriter{to: os.Stderr, into: &captured}

	// Notify keeps Go from killing us on Ctrl-C before we can report it.
	// ssh gets the same SIGINT — it is in the same foreground group — so it
	// dies on its own; we only need to know it was deliberate.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)

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

	outcome := tunnel.OutcomeFailed
	if signaled(err) {
		outcome = tunnel.OutcomeInterrupted
	} else {
		select {
		case <-sig:
			// ssh exited on its own account, but a SIGINT is waiting: the
			// operator asked to stop between the two, and relaunching now
			// would be the exact bug this program must not have.
			outcome = tunnel.OutcomeInterrupted
		default:
		}
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
