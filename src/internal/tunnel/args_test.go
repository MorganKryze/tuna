package tunnel

import (
	"strings"
	"testing"

	"github.com/MorganKryze/tuna/src/internal/config"
)

func TestSSHArgsCarryEveryField(t *testing.T) {
	got := strings.Join(SSHArgs(&config.Destination{
		Name: "vm",
		Host: "debian@10.0.0.5",
		Port: 22022,
		Jump: "mon-hote",
		Forward: []config.Forward{
			{Local: 8201, To: "127.0.0.1:8200"},
			{Local: 9120, To: "10.0.0.5:9120"},
		},
	}), " ")

	for _, attendu := range []string{
		"-N",
		"-o ExitOnForwardFailure=yes",
		"-J mon-hote",
		"-p 22022",
		"-L 8201:127.0.0.1:8200",
		"-L 9120:10.0.0.5:9120",
	} {
		if !strings.Contains(got, attendu) {
			t.Errorf("args ssh sans %q : %s", attendu, got)
		}
	}
	// The host is what ssh reads as its target: anything after it is taken
	// as a remote command, which -N forbids.
	if !strings.HasSuffix(got, "debian@10.0.0.5") {
		t.Errorf("l'hôte doit être le dernier argument : %s", got)
	}
}

func TestSSHArgsOmitWhatIsNotSet(t *testing.T) {
	got := strings.Join(SSHArgs(&config.Destination{
		Host:    "mon-hote",
		Forward: []config.Forward{{Local: 9090, To: "127.0.0.1:9090"}},
	}), " ")
	if strings.Contains(got, "-J") || strings.Contains(got, "-p") {
		t.Errorf("un champ vide ne doit produire aucun drapeau : %s", got)
	}
}

// Each argument has to be its own element: hand ssh "-L 8201:…" as a single
// string and it reads the space as part of the forward specification.
func TestSSHArgsAreSeparateElements(t *testing.T) {
	args := SSHArgs(&config.Destination{
		Host:    "h",
		Forward: []config.Forward{{Local: 1, To: "127.0.0.1:1"}},
	})
	for _, a := range args {
		if strings.Contains(a, " ") {
			t.Fatalf("un argument contient une espace : %q dans %v", a, args)
		}
	}
}
