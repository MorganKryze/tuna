# Contributing

Not every contribution is code. Opening an issue is one, and there are two
templates waiting to guide you through it, [Bug report](https://github.com/MorganKryze/tuna/issues/new?template=bug_report.md) and [Feature idea](https://github.com/MorganKryze/tuna/issues/new?template=feature_idea.md): each asks the few things that turn a report into something actionable.

## Dev loop

```sh
go test ./...
go build -o tuna ./src/cmd/tuna && ./tuna
```

With [just](https://just.systems) installed, `just` lists the shortcuts:
`build`, `test`, `coverage`, `lint`, `hooks`. Linting is
[golangci-lint](https://golangci-lint.run) with a near-default config
(`.golangci.yml`); CI runs it on every push.

**Run `just hooks` once after cloning.** It points git at `githooks/`, and
without it the pre-commit hook does not exist — you find out from a red CI run
five minutes after opening a pull request instead of from a thirty-second
round trip locally. The hook runs gofmt, `go vet` and the tests.

## Layout

```text
src/cmd/tuna/
  main.go       wiring only: flags, the order things are called in, messages
  ssh.go        the real runner: exec, SIGINT, the stderr tee
src/internal/
  config/       read and validate destinations.toml; depends on nothing
  recent/       the recency order: read it, bump a name to the front, write it
  pick/         the picker
  tunnel/       ssh arguments, failure classification, the reconnection loop
  ui/           escape codes, when colour is allowed, column-aware padding
githooks/       pre-commit, installed by `just hooks`
```

The dependency graph runs one way: `config` and `ui` know nothing; `recent`,
`pick` and `tunnel` know `config`; `main` wires them. Keep it that way. A test
lives beside the package it exercises.

Inside a package, one file per concern, and the seam is always the same one —
what can be tested without the world, and what cannot:

```text
pick/pick.go     Picker, Matches, Update: pure, state + keystroke → state
pick/keys.go     readKey: bytes → keystroke, pure
pick/render.go   Frame: state + width + height + colour → a string, pure
pick/preview.go  the frames --preview shows, pure
pick/term.go     Pick: the only code that touches a terminal
tunnel/tunnel.go Connect: the state machine, driven by an injectable Runner
tunnel/args.go   SSHArgs
tunnel/hopeless.go  which stderr means retrying is pointless
config/config.go    types, paths, loading
config/validate.go  what a valid destinations.toml is
```

**Drawing is a pure function too.** `Frame(width, height, colour) string` takes
its terminal as arguments instead of reading one, which is what lets the layout
be asserted at every width from 20 to 200 columns — and looked at, with
`tuna --preview 60`. Two rules come with it, both enforced by tests: no line
may ever exceed the width, because a wrapped line makes the frame taller than
it is counted to be and the redraw then erases the wrong rows; and colour must
never change the geometry, so the same frame with and without escape codes has
to lay out identically.

`term.go` and `main.go` are the only files allowed to be thin and untested.
Everything they do beyond drawing or wiring lives in a package that is tested
without a TTY. If logic shows up in either, it is in the wrong place.

The rule that makes it work: `pick` does not sort and `recent` does not know
the picker exists. Recency is computed upstream and handed over as an
argument. Likewise `tunnel.Connect` never sees `os/exec`, a signal or a clock:
it calls a `Runner` that reports how long the tunnel lived and how it ended,
which is why the entire reconnection policy is tested in microseconds.

## Ground rules

- **Scope.** tuna opens one `ssh -N -L` at a time, in the foreground. No
  daemon, no multiple tunnels, no `stop`, no state beyond an ordered list of
  names. The [README](README.md#what-tuna-does-not-do) says where that line
  sits and when each piece would be worth reconsidering.
- **Dependencies: three, and a fourth needs an argument.** `BurntSushi/toml`
  because the standard library does not read TOML and TOML is the only
  hand-editable format that keeps comments — which the example config lives on.
  `golang.org/x/term` for raw mode, and `golang.org/x/sys` behind it; both
  official. A fourth needs the same case made in the pull request: what it
  buys, what it costs in `go.sum`, and why the standard library cannot. The
  picker is hand-written on `x/term` because `bubbletea` pulls thirty-five
  modules — including a spring physics engine — to draw four lines.
- **Deciding logic is a pure function, and it arrives with its test.** The
  reconnection loop, the picker's navigation and the recency order are all
  written as functions of their inputs, which is why the suite runs in under a
  second with no terminal, no clock and no network. `Connect` takes an
  injectable `Runner`; it does not know `os/exec`. Code that touches the
  terminal or a process stays thin and untested, by construction.
- **Config errors are product.** Anything you can get wrong in the TOML must
  fail at startup, naming the file and the offending key, never in the middle
  of an `ssh`. A test proves it.
- **Never guess at Ctrl-C.** The interrupt is caught with `signal.Notify` and
  never inferred from an exit code. A tunnel that relaunches itself when you
  try to close it is the worst bug this program can have, and the test suite
  guards it.
- **French to the user, English in the code.** The binary talks to a
  francophone; the code is read by everyone. No error message ever contains an
  absolute path from a development machine.

## Commits

One imperative line, scoped, short:

```text
run: reset the attempt counter after a stable tunnel
docs: say what happens on Ctrl-C
```

**On your first pull request the checks will sit there doing nothing**, and
that is normal rather than something you did wrong: this repository asks a
maintainer to approve workflow runs coming from a fork the first time, so
nobody can spend its CI by opening a pull request. One click here starts them,
and after that yours run on their own.
