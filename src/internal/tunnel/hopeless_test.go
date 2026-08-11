package tunnel

import "testing"

func TestHopelessRecognisesTheThreeDeadEnds(t *testing.T) {
	cases := []struct {
		name, stderr string
		want         bool
	}{
		{"port taken", "bind [127.0.0.1]:8200: Address already in use", true},
		{"key refused", "debian@10.0.0.5: Permission denied (publickey)", true},
		{"unknown name", "ssh: Could not resolve hostname nope", true},
		// Everything below is a network problem, and a network problem is
		// exactly what the retry loop exists for. Classify one of these as
		// hopeless and tuna stops surviving a wifi change.
		{"host down", "ssh: connect to host mon-hote port 22: Connection refused", false},
		{"network gone", "ssh: connect to host mon-hote port 22: Network is unreachable", false},
		{"timed out", "ssh: connect to host mon-hote port 22: Operation timed out", false},
		{"back from sleep", "client_loop: send disconnect: Broken pipe", false},
		{"nothing at all", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			line, ok := Hopeless(c.stderr)
			if ok != c.want {
				t.Fatalf("want hopeless=%v for %q, got %v", c.want, c.stderr, ok)
			}
			if ok && line == "" {
				t.Fatal("a hopeless failure has to return ssh's line, not an empty string")
			}
		})
	}
}

// ssh prints a paragraph, and the marker is rarely on the first line. The
// returned line has to be the offending one, not the whole block and not the
// first thing ssh happened to say.
func TestHopelessReturnsTheOffendingLineOnly(t *testing.T) {
	stderr := "OpenSSH_9.8p1, LibreSSL 3.3.6\n" +
		"debug1: Reading configuration data /etc/ssh/ssh_config\n" +
		"bind [127.0.0.1]:8200: Address already in use\n" +
		"channel_setup_fwd_listener_tcpip: cannot listen to port: 8200\n"

	line, ok := Hopeless(stderr)
	if !ok {
		t.Fatal("the marker is there, it has to be recognised")
	}
	if line != "bind [127.0.0.1]:8200: Address already in use" {
		t.Fatalf("want the offending line alone, got %q", line)
	}
}
