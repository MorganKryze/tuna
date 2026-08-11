package port

import (
	"net"
	"testing"

	"github.com/MorganKryze/tuna/src/internal/config"
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
