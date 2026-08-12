// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

// Command tunny opens an admin SSH tunnel chosen from a list, and keeps it up
// across a network change.
//
// This file is wiring only: flags, the order the pieces are called in, and
// the messages. Every decision it looks like it makes belongs to a package
// under src/internal, where it is tested. If logic shows up here, it is in
// the wrong place.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/MorganKryze/tunny/internal/config"
	"github.com/MorganKryze/tunny/internal/pick"
	"github.com/MorganKryze/tunny/internal/port"
	"github.com/MorganKryze/tunny/internal/recent"
	"github.com/MorganKryze/tunny/internal/tunnel"
	"github.com/MorganKryze/tunny/internal/ui"
)

// version is what -ldflags "-X main.version=…" writes into, which is how every
// distribution stamps a Go binary.
//
// The initialiser has to stay a plain constant. The linker writes its value
// into the variable's static data, and a function call here would overwrite it
// again at package-init time: -X would apply and then be silently undone. That
// is what used to happen, and it made every packaged build call itself "dev".
var version = "dev"

// buildVersion is the fallback for a build nobody stamped: `go install …@v1.2.3`
// records the module version in the binary, and the toolchain stamps a version
// derived from VCS for a build inside a git checkout. A source tarball has
// neither, which is exactly the case -ldflags exists for.
func buildVersion() string {
	if version != "dev" {
		return version // the linker spoke, and it has the last word
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return version
}

func main() {
	noRetry := flag.Bool("no-retry", false, "one attempt, no reconnection")
	preview := flag.Bool("preview", false, "print the list without opening anything, then exit")
	showVersion := flag.Bool("version", false, "print the version and exit")
	list := flag.Bool("list", false, "print destination names, one per line, for shell completion")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `tunny %s — open an admin SSH tunnel.

  tunny              pick from the list
  tunny <name>       go straight to it
  tunny --no-retry   do not relaunch if it drops
  tunny --preview    see what the list looks like, without opening anything
  tunny --version    print the version and exit

Config: %s
`, buildVersion(), config.Path())
	}
	flag.Parse()

	// Bare, on stdout, exit 0. Every packaging ecosystem's smoke test runs
	// `<binary> --version` and reads what comes back: Homebrew's `test do`,
	// Debian's autopkgtest, the AUR check(). Decorating it costs them a regex.
	if *showVersion {
		fmt.Println(buildVersion())
		return
	}

	// Names only, one per line, nothing else. There is deliberately no
	// human-facing `tunny list` — the picker is the list — but a shell
	// completing an argument is not a human, and the alternative is a second
	// TOML parser living in three shell scripts.
	if *list {
		cfg, err := config.Load(config.Path())
		if err != nil {
			// On stderr and not swallowed: the completions redirect it away
			// themselves, and somebody running --list by hand to find out why
			// their completion is empty deserves the reason.
			fmt.Fprint(os.Stderr, failed(err, ui.ColorOK(os.Stderr)))
			os.Exit(1)
		}
		for _, n := range cfg.Names() {
			fmt.Println(n)
		}
		return
	}

	if *preview {
		if err := showPreview(); err != nil {
			fmt.Fprintf(os.Stderr, "tunny: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// One place owns "the operator wants out", and it covers every way of
	// saying it. SIGINT is Ctrl-C; SIGTERM is a service manager or a pkill;
	// SIGHUP is the terminal going away. Before this, only SIGINT was heard,
	// and only while ssh was actually running.
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

	if err := launch(ctx, flag.Arg(0), *noRetry); err != nil {
		if errors.Is(err, pick.ErrNoChoice) {
			return // Escape is not a failure
		}
		fmt.Fprint(os.Stderr, failed(err, ui.ColorOK(os.Stderr)))
		os.Exit(1)
	}
}

// showPreview draws the picker without opening it, at the real terminal's
// width when there is one and at 80 columns when there is not, so the output
// is the same whether a human is looking or a test is capturing it.
func showPreview() error {
	cfg, err := config.Load(config.Path())
	if err != nil {
		return err
	}
	width, height := 80, 24
	if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		width, height = w, h
	}
	// An explicit width is how the narrow-terminal fallbacks get looked at
	// without owning a narrow terminal: `tunny --preview 60`.
	if arg := flag.Arg(0); arg != "" {
		w, err := strconv.Atoi(arg)
		if err != nil || w < 20 {
			return fmt.Errorf("width %q: expected a number of at least 20", arg)
		}
		width = w
	}
	ordered := recent.Order(cfg.Destination, recent.Load(recent.Path()))
	fmt.Print(pick.Preview(ordered, port.BusyIn(ordered), width, height, ui.ColorOK(os.Stdout)))
	return nil
}

func launch(ctx context.Context, name string, noRetry bool) error {
	cfg, err := config.Load(config.Path())
	if err != nil {
		return err
	}

	statePath := recent.Path()
	if name == "" {
		// The picker receives a list already in order: it does not sort, and
		// recent does not know the picker exists.
		ordered := recent.Order(cfg.Destination, recent.Load(statePath))
		if name, err = pick.Pick(ctx, ordered, port.BusyIn(ordered)); err != nil {
			return err
		}
	}

	dest, ok := cfg.Find(name)
	if !ok {
		return fmt.Errorf("unknown destination %q; known: %s", name, strings.Join(cfg.Names(), ", "))
	}

	// After the request has been understood and before anything is printed.
	// Without it, a missing ssh spends seven seconds on three retries and
	// blames the network, while the real error is thrown away with the attempt
	// that produced it. Later than the name check on purpose: a typo in the
	// destination is the more immediate mistake, and it deserves its own
	// message even on a machine with no ssh at all.
	if _, err := exec.LookPath(sshPath); err != nil {
		return fmt.Errorf("%s is not on your PATH; install an OpenSSH client", sshPath)
	}

	// Checked before the banner and before ssh, because ssh finding out is
	// three lines of diagnostics after a banner that promised a tunnel. The
	// picker showed this already; `tunny <nom>` skipped the picker, and a port
	// can be taken while the list is on screen either way.
	if taken := port.BusyIn([]config.Destination{*dest}); len(taken[dest.Name]) > 0 {
		return busyError(dest.Name, taken[dest.Name])
	}

	// Written before connecting, not after: a tunnel held open for hours
	// would otherwise leave the order stale for the whole session, and a
	// crash would lose it entirely. A failure here is worth a warning and
	// nothing more — it costs a list in the wrong order, not a tunnel.
	if err := recent.Save(statePath, recent.Bump(recent.Load(statePath), dest.Name)); err != nil {
		fmt.Fprint(os.Stderr, failed(fmt.Errorf("could not save the recency order: %w", err), ui.ColorOK(os.Stderr)))
	}

	// The banner goes to stdout: the URLs are the one thing worth piping
	// somewhere. Everything else tunny says is on stderr.
	fmt.Print(banner(dest, ui.ColorOK(os.Stdout)))

	retry := tunnel.DefaultRetry()
	if noRetry {
		retry.Max = 0
	}
	// From here on the terminal belongs to the tunnel, and the only thing on
	// it should be what tunny and ssh say. The "^C" that used to land in the
	// middle of that is the terminal driver echoing the keystroke, not a
	// message from anyone.
	defer ui.HideControlEcho(os.Stdin)()

	color := ui.ColorOK(os.Stderr)
	retry.Notify = func(attempt, max int, wait time.Duration) {
		fmt.Fprint(os.Stderr, retrying(attempt, max, wait, color))
	}
	retry.OnUp = func(reconnected bool) {
		if reconnected {
			fmt.Fprint(os.Stderr, restored(color))
			return
		}
		fmt.Fprint(os.Stderr, established(color))
	}

	if err := tunnel.Connect(ctx, dest, sshRunner, retry); err != nil {
		return err
	}
	// Silence used to be the only sign that tunny had stopped rather than
	// gone away to try again. This is the line that tells them apart.
	fmt.Fprint(os.Stderr, closed(color))
	return nil
}
