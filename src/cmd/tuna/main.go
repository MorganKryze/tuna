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
	"strings"

	"github.com/MorganKryze/tuna/src/internal/config"
	"github.com/MorganKryze/tuna/src/internal/pick"
	"github.com/MorganKryze/tuna/src/internal/recent"
	"github.com/MorganKryze/tuna/src/internal/tunnel"
)

// version is stamped by the release workflow; a local build says so.
var version = "dev"

func main() {
	noRetry := flag.Bool("no-retry", false, "une seule tentative, pas de reconnexion")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `tuna %s — ouvre un tunnel SSH d'admin.

  tuna              choisir dans la liste
  tuna <nom>        lancer directement
  tuna --no-retry   ne pas relancer si ça coupe

Configuration : %s
`, version, config.Path())
	}
	flag.Parse()

	if err := launch(flag.Arg(0), *noRetry); err != nil {
		if errors.Is(err, pick.ErrNoChoice) {
			return // Escape is not a failure
		}
		fmt.Fprintf(os.Stderr, "tuna: %v\n", err)
		os.Exit(1)
	}
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
		if name, err = pick.Pick(recent.Order(cfg.Destination, recent.Load(statePath))); err != nil {
			return err
		}
	}

	dest, ok := cfg.Find(name)
	if !ok {
		return fmt.Errorf("destination %q inconnue ; connues : %s", name, strings.Join(cfg.Names(), ", "))
	}

	// Written before connecting, not after: a tunnel held open for hours
	// would otherwise leave the order stale for the whole session, and a
	// crash would lose it entirely. A failure here is worth a warning and
	// nothing more — it costs a list in the wrong order, not a tunnel.
	if err := recent.Save(statePath, recent.Bump(recent.Load(statePath), dest.Name)); err != nil {
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

	retry := tunnel.DefaultRetry()
	if noRetry {
		retry.Max = 0
	}
	return tunnel.Connect(dest, sshRunner, retry)
}
