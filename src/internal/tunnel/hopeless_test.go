package tunnel

import "testing"

func TestHopelessRecognisesTheThreeDeadEnds(t *testing.T) {
	cases := []struct {
		nom, stderr string
		attendu     bool
	}{
		{"port occupé", "bind [127.0.0.1]:8200: Address already in use", true},
		{"clé refusée", "debian@10.0.0.5: Permission denied (publickey)", true},
		{"nom inconnu", "ssh: Could not resolve hostname nope", true},
		// Everything below is a network problem, and a network problem is
		// exactly what the retry loop exists for. Classify one of these as
		// hopeless and tuna stops surviving a wifi change.
		{"hôte éteint", "ssh: connect to host mon-hote port 22: Connection refused", false},
		{"réseau coupé", "ssh: connect to host mon-hote port 22: Network is unreachable", false},
		{"délai dépassé", "ssh: connect to host mon-hote port 22: Operation timed out", false},
		{"retour de veille", "client_loop: send disconnect: Broken pipe", false},
		{"rien du tout", "", false},
	}
	for _, c := range cases {
		t.Run(c.nom, func(t *testing.T) {
			line, ok := Hopeless(c.stderr)
			if ok != c.attendu {
				t.Fatalf("attendu hopeless=%v pour %q, obtenu %v", c.attendu, c.stderr, ok)
			}
			if ok && line == "" {
				t.Fatal("un échec sans espoir doit rendre la ligne de ssh, pas une chaîne vide")
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
		t.Fatal("le marqueur est présent, il doit être reconnu")
	}
	if line != "bind [127.0.0.1]:8200: Address already in use" {
		t.Fatalf("ligne fautive attendue seule, obtenu %q", line)
	}
}
