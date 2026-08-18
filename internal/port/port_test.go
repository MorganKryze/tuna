// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

package port

import (
	"net"
	"testing"

	"github.com/MorganKryze/tunny/internal/config"
)

// held opens a real listener and hands back its port. Faking this would test
// the fake: the whole point is that the probe makes the same syscall ssh is
// about to make.
func held(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l.Addr().(*net.TCPAddr).Port
}

// free asks the kernel for a port and gives it straight back, which is the
// closest thing to "a port nobody is using" that does not race with the rest
// of the machine.
func free(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	n := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestBusySeesARealListener(t *testing.T) {
	if n := held(t); !Busy(n) {
		t.Errorf("port %d is being listened on, Busy has to say true", n)
	}
	if n := free(t); Busy(n) {
		t.Errorf("port %d is free, Busy has to say false", n)
	}
}

// The probe must not become the problem: asking twice in a row has to give
// the same answer, which it only does if the first probe released the port.
func TestProbingDoesNotHoldThePort(t *testing.T) {
	n := free(t)
	for i := range 3 {
		if Busy(n) {
			t.Fatalf("probe %d: port %d turned busy, most likely from the probe itself", i+1, n)
		}
	}
}

func TestBusyInReportsOnlyWhatIsTaken(t *testing.T) {
	busy, idle := held(t), free(t)
	dests := []config.Destination{
		{Name: "taken", Forward: []config.Forward{
			{Local: busy, To: "127.0.0.1:1"},
			{Local: idle, To: "127.0.0.1:2"},
		}},
		{Name: "free", Forward: []config.Forward{{Local: idle, To: "127.0.0.1:3"}}},
	}

	got := BusyIn(dests)
	// A destination with nothing in the way is absent, not present-and-empty:
	// the callers read a missing key as "go ahead".
	if _, ok := got["free"]; ok {
		t.Errorf("a destination with nothing in the way must not show up: %v", got)
	}
	if want := []int{busy}; len(got["taken"]) != 1 || got["taken"][0] != want[0] {
		t.Errorf("want %v for the taken destination, got %v", want, got["taken"])
	}
}

// A port held on ::1 alone used to read as free, because the probe only ever
// asked 127.0.0.1. ssh -L binds "localhost", which is both addresses on any
// machine with IPv6, so the picker said go ahead to a forward that could not
// bind and ssh found out afterwards.
func TestBusySeesAListenerOnTheIPv6Loopback(t *testing.T) {
	l, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skipf("no IPv6 loopback here: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })
	if n := l.Addr().(*net.TCPAddr).Port; !Busy(n) {
		t.Errorf("port %d is being listened on over ::1, Busy has to say true", n)
	}
}

// A bind is refused for reasons that have nothing to do with the port being
// taken, and the difference is the whole value of the answer: "busy" names a
// culprit and sends somebody to lsof to find it. 192.0.2.1 is TEST-NET-1 and
// is not an address of this machine, so the kernel refuses every bind to it —
// the same refusal a machine with IPv6 switched off gives for ::1.
func TestAnUnbindableAddressIsNotAPortInUse(t *testing.T) {
	if taken("192.0.2.1", free(t)) {
		t.Error("an address this machine does not have is not a port in use")
	}
}
