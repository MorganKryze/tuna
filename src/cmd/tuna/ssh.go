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

// sshRunner is the only code in the program that touches processes and
// signals. It reports how long ssh lived and how it ended; tunnel.Connect
// decides what that means. Keeping the two apart is what lets the whole
// reconnection policy be tested without spawning anything.
func sshRunner(d *config.Destination) tunnel.Result {
	cmd := exec.Command("ssh", tunnel.SSHArgs(d)...)
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
	err := cmd.Run()
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
