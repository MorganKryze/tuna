// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The command-line surface, exercised by running the real binary. main.go,
// launch() and showPreview() are wiring, and wiring is the one thing a unit
// test cannot check: what a packager and a script see is the process, not the
// functions.
//
// Exit codes especially. They are documented in the README and in tunny.1, and
// nothing else holds them to it.

// build compiles tunny once and hands back its path. `go test` already has a
// toolchain, so this needs nothing the suite did not already require.
func build(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "tunny")
	out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("building tunny: %v\n%s", err, out)
	}
	return bin
}

// sandbox gives the binary an XDG root of its own, holding the example config
// so the test never reads or writes the developer's real one.
func sandbox(t *testing.T, config string) []string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "config", "tunny")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if config != "" {
		if err := os.WriteFile(filepath.Join(dir, "destinations.toml"), []byte(config), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return []string{
		"XDG_CONFIG_HOME=" + filepath.Join(root, "config"),
		"XDG_STATE_HOME=" + filepath.Join(root, "state"),
		"NO_COLOR=1",
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + root,
	}
}

const twoDestinations = `
[[destination]]
name = "alpha"
desc = "the first one"
host = "nowhere.invalid"
forward = [{ local = 19911, to = "127.0.0.1:1", label = "One" }]

[[destination]]
name = "beta"
desc = "the second one"
host = "nowhere.invalid"
forward = [{ local = 19912, to = "127.0.0.1:2" }]
`

func TestTheCommandLineSurface(t *testing.T) {
	bin := build(t)

	for _, c := range []struct {
		name     string
		args     []string
		config   string
		wantExit int
		wantOut  string // on stdout
		wantErr  string // on stderr
		bare     bool   // stdout is exactly one whitespace-free token
	}{
		{
			// Homebrew's test do, Debian's autopkgtest and the AUR check()
			// all call this and read what comes back, so it owes them one
			// token and nothing else. What the token *is* depends on how the
			// binary was built, which TestTheLinkerHasTheLastWordOnTheVersion
			// covers.
			name: "version is bare, on stdout, and succeeds",
			args: []string{"--version"}, config: twoDestinations, bare: true,
		},
		{
			name: "list is one name per line, for the completions",
			args: []string{"--list"}, config: twoDestinations, wantOut: "alpha\nbeta\n",
		},
		{
			name: "preview draws without opening anything",
			args: []string{"--preview", "--width", "80"}, config: twoDestinations, wantOut: "the first one",
		},
		{
			name: "a width below the minimum is refused, not drawn badly",
			args: []string{"--preview", "--width", "5"}, config: twoDestinations,
			wantExit: 1, wantErr: "at least 20",
		},
		{
			// The width used to be a positional argument, in the same slot
			// the destination name lives in. Somebody who typed it that way
			// gets told where it went rather than a guess at which was meant.
			name: "the old positional width says where the width went",
			args: []string{"--preview", "80"}, config: twoDestinations,
			wantExit: 1, wantErr: "--width 80",
		},
		{
			name: "a width without a preview is a typo, not a no-op",
			args: []string{"--width", "60", "alpha"}, config: twoDestinations,
			wantExit: 2, wantErr: "only means something with --preview",
		},
		{
			name: "an unknown destination lists the known ones",
			args: []string{"gamma"}, config: twoDestinations,
			wantExit: 1, wantErr: `unknown destination "gamma"; known: alpha, beta`,
		},
		{
			name: "no config says what to do next, once",
			args: []string{"--list"}, config: "",
			wantExit: 1, wantErr: "destinations.example.toml",
		},
		{
			name: "an invalid config names the offending key",
			args: []string{"--list"}, config: "[[destination]]\nname = \"a\"\nhost = \"h\"\nforwards = []\n",
			wantExit: 1, wantErr: "forwards",
		},
		{
			name: "an unrecognised flag exits 2, from the flag package",
			args: []string{"--nope"}, config: twoDestinations,
			wantExit: 2, wantErr: "not defined",
		},
		{
			name: "without a terminal and without a name, it says so",
			args: nil, config: twoDestinations,
			wantExit: 1, wantErr: "not a terminal",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			cmd := exec.Command(bin, c.args...)
			cmd.Env = sandbox(t, c.config)
			var stdout, stderr strings.Builder
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()

			code := 0
			var ee *exec.ExitError
			switch {
			case errors.As(err, &ee):
				code = ee.ExitCode()
			case err != nil:
				t.Fatal(err)
			}
			if code != c.wantExit {
				t.Errorf("exit %d, want %d\nstdout: %s\nstderr: %s", code, c.wantExit, &stdout, &stderr)
			}
			if c.wantOut != "" && !strings.Contains(stdout.String(), c.wantOut) {
				t.Errorf("stdout: want %q in %q", c.wantOut, stdout.String())
			}
			if c.wantErr != "" && !strings.Contains(stderr.String(), c.wantErr) {
				t.Errorf("stderr: want %q in %q", c.wantErr, stderr.String())
			}
			if c.bare {
				if fields := strings.Fields(stdout.String()); len(fields) != 1 {
					t.Errorf("want one bare token on stdout, got %q", stdout.String())
				}
				if stderr.Len() != 0 {
					t.Errorf("want nothing on stderr, got %q", stderr.String())
				}
			}
		})
	}
}

// --preview is the smoke test PACKAGING.md hands to maintainers, so it owes
// them what it promises: no tunnel, and not a byte written outside the sandbox.
func TestPreviewWritesNothing(t *testing.T) {
	bin := build(t)
	env := sandbox(t, twoDestinations)

	var state string
	for _, kv := range env {
		if after, ok := strings.CutPrefix(kv, "XDG_STATE_HOME="); ok {
			state = after
		}
	}

	cmd := exec.Command(bin, "--preview", "--width", "80")
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("preview failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Errorf("--preview created %s; it must not touch state", state)
	}
}
