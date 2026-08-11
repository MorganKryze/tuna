// Command tuna opens an admin SSH tunnel chosen from a list, and keeps it up
// across a network change.
//
// This file is wiring only: flags, the order the pieces are called in, and
// the messages. Every decision it looks like it makes belongs to a package
// under src/internal, where it is tested. If logic shows up here, it is in
// the wrong place.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/MorganKryze/tuna/src/internal/config"
	"github.com/MorganKryze/tuna/src/internal/pick"
	"github.com/MorganKryze/tuna/src/internal/port"
	"github.com/MorganKryze/tuna/src/internal/recent"
	"github.com/MorganKryze/tuna/src/internal/tunnel"
	"github.com/MorganKryze/tuna/src/internal/ui"
)

// version is stamped by the release workflow; a local build says so.
var version = "dev"

func main() {
	noRetry := flag.Bool("no-retry", false, "une seule tentative, pas de reconnexion")
	preview := flag.Bool("preview", false, "afficher la liste sans l'ouvrir, et sortir")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `tuna %s — ouvre un tunnel SSH d'admin.

  tuna              choisir dans la liste
  tuna <nom>        lancer directement
  tuna --no-retry   ne pas relancer si ça coupe
  tuna --preview    voir à quoi ressemble la liste, sans rien ouvrir

Configuration : %s
`, version, config.Path())
	}
	flag.Parse()

	if *preview {
		if err := showPreview(); err != nil {
			fmt.Fprintf(os.Stderr, "tuna: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := launch(flag.Arg(0), *noRetry); err != nil {
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
	// without owning a narrow terminal: `tuna --preview 60`.
	if arg := flag.Arg(0); arg != "" {
		w, err := strconv.Atoi(arg)
		if err != nil || w < 20 {
			return fmt.Errorf("largeur %q : attendu un nombre d'au moins 20", arg)
		}
		width = w
	}
	ordered := recent.Order(cfg.Destination, recent.Load(recent.Path()))
	fmt.Print(pick.Preview(ordered, port.BusyIn(ordered), width, height, ui.ColorOK(os.Stdout)))
	return nil
}

func launch(name string, noRetry bool) error {
	cfg, err := config.Load(config.Path())
	if err != nil {
		return err
	}

	statePath := recent.Path()
	if name == "" {
		// The picker receives a list already in order: it does not sort, and
		// recent does not know the picker exists.
		ordered := recent.Order(cfg.Destination, recent.Load(statePath))
		if name, err = pick.Pick(ordered, port.BusyIn(ordered)); err != nil {
			return err
		}
	}

	dest, ok := cfg.Find(name)
	if !ok {
		return fmt.Errorf("destination %q inconnue ; connues : %s", name, strings.Join(cfg.Names(), ", "))
	}

	// Checked before the banner and before ssh, because ssh finding out is
	// three lines of diagnostics after a banner that promised a tunnel. The
	// picker showed this already; `tuna <nom>` skipped the picker, and a port
	// can be taken while the list is on screen either way.
	if taken := port.BusyIn([]config.Destination{*dest}); len(taken[dest.Name]) > 0 {
		return busyError(dest.Name, taken[dest.Name])
	}

	// Written before connecting, not after: a tunnel held open for hours
	// would otherwise leave the order stale for the whole session, and a
	// crash would lose it entirely. A failure here is worth a warning and
	// nothing more — it costs a list in the wrong order, not a tunnel.
	if err := recent.Save(statePath, recent.Bump(recent.Load(statePath), dest.Name)); err != nil {
		fmt.Fprint(os.Stderr, failed(fmt.Errorf("ordre de récence non enregistré : %w", err), ui.ColorOK(os.Stderr)))
	}

	// The banner goes to stdout: the URLs are the one thing worth piping
	// somewhere. Everything else tuna says is on stderr.
	fmt.Print(banner(dest, ui.ColorOK(os.Stdout)))

	retry := tunnel.DefaultRetry()
	if noRetry {
		retry.Max = 0
	}
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

	if err := tunnel.Connect(dest, sshRunner, retry); err != nil {
		return err
	}
	// Silence used to be the only sign that tuna had stopped rather than
	// gone away to try again. This is the line that tells them apart.
	fmt.Fprint(os.Stderr, closed(color))
	return nil
}
