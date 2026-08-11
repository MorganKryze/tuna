package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	noRetry := flag.Bool("no-retry", false, "une seule tentative, pas de reconnexion")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `tuna — ouvre un tunnel SSH d'admin.

  tuna              choisir dans la liste
  tuna <nom>        lancer directement
  tuna --no-retry   ne pas relancer si ça coupe

Configuration : %s
`, ConfigPath())
	}
	flag.Parse()

	if err := run(flag.Arg(0), *noRetry); err != nil {
		if errors.Is(err, ErrNoChoice) {
			return // Escape is not a failure
		}
		fmt.Fprintf(os.Stderr, "tuna: %v\n", err)
		os.Exit(1)
	}
}

func run(name string, noRetry bool) error {
	cfg, err := LoadConfig(ConfigPath())
	if err != nil {
		return err
	}

	statePath := StatePath()
	if name == "" {
		ordered := Order(cfg.Destination, LoadRecent(statePath))
		if name, err = Pick(ordered); err != nil {
			return err
		}
	}

	dest, ok := cfg.Find(name)
	if !ok {
		known := make([]string, 0, len(cfg.Destination))
		for _, d := range cfg.Destination {
			known = append(known, d.Name)
		}
		return fmt.Errorf("destination %q inconnue ; connues : %s", name, strings.Join(known, ", "))
	}

	// Written before connecting, not after: a tunnel held open for hours
	// would otherwise leave the order stale for the whole session, and a
	// crash would lose it entirely.
	if err := SaveRecent(statePath, Bump(LoadRecent(statePath), dest.Name)); err != nil {
		fmt.Fprintf(os.Stderr, "tuna: ordre de récence non enregistré : %v\n", err)
	}

	for _, f := range dest.Forward {
		label := f.Label
		if label == "" {
			label = dest.Name
		}
		fmt.Printf("%-12s → http://localhost:%d\n", label, f.Local)
	}
	fmt.Fprintln(os.Stderr, "Ctrl-C pour fermer.")

	retry := DefaultRetry()
	if noRetry {
		retry.Max = 0
	}
	return Connect(dest, sshRunner, retry)
}

// sshRunner is the only place that touches processes and signals. It reports
// how long ssh lived and how it ended; Connect decides what that means.
func sshRunner(d *Destination) Result {
	cmd := exec.Command("ssh", SSHArgs(d)...)
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

	outcome := OutcomeFailed
	if signaled(err) {
		outcome = OutcomeInterrupted
	} else {
		select {
		case <-sig:
			// ssh exited on its own account, but a SIGINT is waiting: the
			// operator asked to stop between the two, and relaunching now
			// would be the exact bug this program must not have.
			outcome = OutcomeInterrupted
		default:
		}
	}
	return Result{Lived: lived, Stderr: captured.String(), Outcome: outcome}
}

// signaled reports whether the process was killed by SIGINT or SIGTERM
// rather than exiting on its own.
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
	to   *os.File
	into *strings.Builder
}

func (t *teeWriter) Write(p []byte) (int, error) {
	t.into.Write(p)
	return t.to.Write(p)
}
