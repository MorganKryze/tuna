// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"os/exec"
	"strings"
	"syscall"
	"testing"
)

// killed starts a process that will not exit on its own, sends it sig, and
// hands back whatever Wait reported. Spawning a real process is the point:
// signaled reads a syscall.WaitStatus, and a hand-built error would only
// test the fake.
func killed(t *testing.T, sig syscall.Signal) error {
	t.Helper()
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Process.Signal(sig); err != nil {
		t.Fatal(err)
	}
	return cmd.Wait()
}

// This is the single most important behaviour in the program: it is what
// tells "the operator closed the tunnel" from "the tunnel died", and getting
// it wrong gives a tunnel that relaunches itself every time you try to close
// it.
func TestSignaledTellsADeliberateCloseFromAFailure(t *testing.T) {
	if got := signaled(killed(t, syscall.SIGINT)); !got {
		t.Error("SIGINT has to read as a deliberate close")
	}
	if got := signaled(killed(t, syscall.SIGTERM)); !got {
		t.Error("SIGTERM has to read as a deliberate close")
	}

	// A process that chose to exit, however badly, is a failure worth
	// retrying — which is every case where ssh itself gives up.
	if got := signaled(exec.Command("false").Run()); got {
		t.Error("a non-zero exit code is not a deliberate close")
	}
	// No error at all: ssh -N returned on its own.
	if got := signaled(nil); got {
		t.Error("nil is not a deliberate close")
	}
	// Not an ExitError: the binary is missing, and nothing ever ran.
	if got := signaled(exec.Command("tunny-that-does-not-exist").Run()); got {
		t.Error("a missing binary is not a deliberate close")
	}

	// SIGKILL is deliberately not on the list: it is what a system sends
	// when it takes a process away, not what an operator types, so the
	// tunnel is worth bringing back.
	if got := signaled(killed(t, syscall.SIGKILL)); got {
		t.Error("SIGKILL has to stay retryable")
	}
}

// stderr is shown and kept at once: shown because ssh's own words are the
// best diagnostic there is, kept because three of them decide whether to
// retry. Dropping either half breaks one of the two.
func TestTeeWriterShowsAndKeeps(t *testing.T) {
	var shown, kept strings.Builder
	w := &teeWriter{to: &shown, into: &kept}

	for _, s := range []string{"bind [127.0.0.1]:8200: ", "Address already in use\n"} {
		n, err := w.Write([]byte(s))
		if err != nil {
			t.Fatal(err)
		}
		// A short count makes io.Copy retry the tail and duplicate it.
		if n != len(s) {
			t.Fatalf("wrote %d bytes out of %d", n, len(s))
		}
	}

	want := "bind [127.0.0.1]:8200: Address already in use\n"
	if shown.String() != want {
		t.Fatalf("shown: want %q, got %q", want, shown.String())
	}
	if kept.String() != want {
		t.Fatalf("kept: want %q, got %q", want, kept.String())
	}
}
