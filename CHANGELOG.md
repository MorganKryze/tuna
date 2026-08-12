<!--
SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
SPDX-License-Identifier: GPL-3.0-only
-->

# Changelog

Newest first. Dates are release dates.

## Unreleased

### ⚠️ Breaking

- **Renamed to `tunny`.** The name `tuna` belongs to kernel.org's thread
  affinity tool, which owns `/usr/bin/tuna` in Arch and Debian and holds the
  attribute in nixpkgs. `tunny` is a word for the same fish and is free
  everywhere. Move your config across: `mv ~/.config/tuna ~/.config/tunny`.
  tunny prints that exact command if it finds the old directory.
- **The binary, the module path and the layout all moved**:
  `go install github.com/MorganKryze/tunny/cmd/tunny@latest`. `src/cmd/tunny`
  became `cmd/tunny` and `src/internal` became `internal`, which is where Go
  programs keep those and where packagers expect them.
- **State moved** to `~/.local/state/tunny/recent`. Nothing carries over; the
  list starts empty and refills as you use it.

### ✨ Features

- **`--version`**, printing a bare version on stdout, exit 0. Every packaging
  ecosystem's smoke test calls it.
- **`--list`**, printing destination names one per line, for the shell
  completions. There is still no human-facing list command: the picker is the
  list.
- **Shell completions** for bash, zsh and fish, completing destination names
  from tunny itself rather than from a second TOML parser.
- **A man page**, `tunny.1`.
- **A missing config now says what to do next**, and says it once. It used to
  print the path twice and stop there.
- **`ssh` missing from `PATH` is caught before anything is printed.** It used
  to cost seven seconds of retries and a message blaming the network.

### 🐛 Fixes

- **A SIGTERM no longer orphans the ssh child.** tunny caught SIGINT only, and
  never signalled its child at all: `pkill tunny`, a service manager or an IDE
  closing its terminal left ssh reparented to init, still holding the forwarded
  ports. The next run then reported those ports busy and pointed the operator
  at `lsof` to find tunny's own child. tunny now listens for SIGINT, SIGTERM
  and SIGHUP, and takes the child down with it.
- **Ctrl-C during the backoff is heard.** The signal handler was installed per
  attempt, leaving seven of the ten seconds of a failing episode unhandled, and
  tunny died there by default disposition without restoring the terminal.
- **A SIGTERM while the picker is open no longer hangs**, and restores the
  terminal on the way out.
- **`-ldflags "-X main.version=…"` works again.** The variable's initialiser
  was a function call, and Go's linker only honours `-X` on a constant one, so
  the stamp was applied and then silently overwritten at package-init time.
  Every distribution build reported `dev`.
- **The picker no longer panics between 1 and 5 columns**, where the prompt's
  column budget went negative and ran off the end of a slice. Below 20 columns
  it now says the terminal is too narrow rather than drawing something that
  wraps.
- **Accented keystrokes work.** The filter read one byte and cast it to a rune,
  so `é` became `Ã` and its second byte was dropped; backspace then cut the
  encoding in half.
- **A `-J` destination cannot hang tunny** on a grandchild holding the stderr
  pipe.
- Release binaries no longer report `+dirty`.

### 🧹 Internal

- Licence stated as **GPL-3.0-only**, with a copyright holder and an
  `SPDX-License-Identifier` in every source file. A `debian/copyright` can now
  be written.
- **`PACKAGING.md`**, with the build line, the runtime dependency, the file
  list, the platform list and the verification commands.
- The README gained requirements, a files-and-environment table, documented
  exit codes, uninstall and troubleshooting.
- `go test -race` runs in CI, in `just race` and in the pre-commit hook.
- The process handling is testable: a fake `ssh` on a controlled `PATH` covers
  the signals, the exit codes and the stderr capture without a server.

## v0.1.1 (2026-08-12)

tunny spoke French until this release. Every line it prints changed; nothing
about `destinations.toml` or about how tunnels behave did.

- `CLICOLOR_FORCE` keeps colour through a pipe. `NO_COLOR` still wins over it.
- The README carries a picture of the picker and a social card, both drawn by
  the binary itself.
- Test-count and coverage badges, self-hosted on an orphan branch.

## v0.1.0 (2026-08-11)

First release. A filterable picker with the most recent destination on top,
three reconnection attempts per outage with the counter reset by thirty seconds
of stable tunnel, and a Ctrl-C that never relaunches.

- A local port already taken is shown in the list before you choose, and
  refused at launch with the `lsof` command that names what holds it.
- `Permission denied` and `Could not resolve hostname` give up at once and
  quote ssh.
- `-o ExitOnForwardFailure=yes` on every invocation.
- TOML config validated before anything runs, with unknown keys refused.
- Recency is a file of names, one per line, most recent first.
