<!--
SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
SPDX-License-Identifier: GPL-3.0-only
-->

# Packaging tunny

Everything a distribution maintainer needs, in one file, so nothing has to be
inferred from a workflow.

Patches are welcome, and so are questions: open an issue rather than guess.

## Build

```sh
go build -trimpath -ldflags "-s -w -X main.version=v0.2.0" -o tunny ./cmd/tunny
```

`-X main.version` is the only stamp tunny reads. Without it a build from a
source tarball reports `dev`, because the module version the toolchain records
comes from VCS metadata a tarball does not carry. `main.version` is a plain
string constant for exactly this reason; anything else and the linker's value
is overwritten at package-init time.

`-trimpath` matters beyond neatness: without it the binary embeds the absolute
build directory, which breaks reproducibility.

## Test

```sh
go test ./...
```

No network, no TTY, no ssh server, no clock. The suite spawns a fake `ssh`
shell script on a temporary `PATH` and binds ephemeral loopback ports; it is
safe in a build chroot and suitable for a `check()` or an autopkgtest.

`go test -race ./...` also passes, and CI runs it on linux and darwin.

## Runtime dependency

**An OpenSSH client on `PATH`.** tunny builds an argument list and executes
`ssh`; it implements no SSH itself. It checks for the binary before printing
anything and refuses with a clear message if it is absent.

- Debian: `openssh-client`
- Arch: `openssh`
- nixpkgs: `wrapProgram $out/bin/tunny --prefix PATH : ${lib.makeBinPath [ openssh ]}`
- Homebrew: nothing to declare, macOS ships one

`lsof` is a useful suggestion, never a requirement: one error message names it
as the way to find what holds a busy port. Arch `optdepends`, Debian
`Suggests`.

## Files

| Path | What |
| --- | --- |
| `/usr/bin/tunny` | the binary |
| `/usr/share/doc/tunny/examples/destinations.example.toml` | the starting config |
| `/usr/share/licenses/tunny/LICENSE` (Arch) | GPL-3.0-only |
| `/usr/share/man/man1/tunny.1` | from `docs/tunny.1` |
| `/usr/share/bash-completion/completions/tunny` | from `completions/tunny.bash` |
| `/usr/share/zsh/site-functions/_tunny` | from `completions/tunny.zsh` |
| `/usr/share/fish/vendor_completions.d/tunny.fish` | from `completions/tunny.fish` |
| `~/.config/tunny/destinations.toml` | user config, honours `XDG_CONFIG_HOME` |
| `~/.local/state/tunny/recent` | user state, honours `XDG_STATE_HOME`, mode 0600 |

tunny creates the state directory itself and writes nothing outside the two
XDG paths. It listens on nothing and makes no network connection of its own.

## Toolchain and platforms

`go.mod` declares `go 1.25.0`, inherited from `golang.org/x/term` and
`golang.org/x/sys`. Debian `testing` and `unstable` both carry Go 1.26, as does
`stable-backports`; `stable` at 1.24 does not, which matters only for a
backport.

Builds and passes its tests on **linux, darwin, freebsd, netbsd and openbsd**,
on every architecture Go supports for them. It does **not** build on windows,
solaris, illumos or aix: the terminal handling is `//go:build unix` and no
fallback exists. Declare the honest list rather than "unix".

## Dependencies

Three modules, all packaged in Debian at the versions `go.mod` pins:

| Module | Licence | Debian |
| --- | --- | --- |
| `github.com/BurntSushi/toml` | MIT | `golang-github-burntsushi-toml-dev` |
| `golang.org/x/term` | BSD-3-Clause | `golang-golang-x-term-dev` |
| `golang.org/x/sys` | BSD-3-Clause | `golang-golang-x-sys-dev` |

All compatible with GPL-3.0-only. `go.sum` is complete: `go mod verify` passes
and a build is fully offline once the module cache is populated.

## Licence

**GPL-3.0-only.** This project grants no "or any later version" permission: the
`LICENSE` text is the licence, and no per-file notice offers the option. Every
source file carries an `SPDX-License-Identifier`.

Copyright (C) 2026 Morgan Kryze `<contact@libresoftware.cloud>`

## Verifying a release

Release binaries carry a SHA256SUMS file and a Sigstore build attestation tying
them to the workflow that produced them:

```sh
sha256sum -c SHA256SUMS --ignore-missing
gh attestation verify tunny-linux-amd64 --repo MorganKryze/tunny
```

Source tarballs are GitHub's own for the tag. Two builds of the same tarball
are byte-identical.

## Starting points

`packaging/` holds a working recipe for each ecosystem, kept next to the code
so they move with it:

| File | State |
| --- | --- |
| `packaging/homebrew/tunny.rb` | formula for a tap, and the template the release workflow renders. It rewrites `url` and `sha256` for the new tag and pushes the result to `MorganKryze/homebrew-tap`, so a release bumps the formula on its own. |
| `packaging/aur/PKGBUILD` | its `build()` and `check()` were run; `makepkg` itself was not, for want of an Arch machine. |
| `packaging/nix/tunny.nix` | built with `nix-build` end to end: binary, man page, three completions, example config, and the openssh wrapper. |

Both hashes in the Nix file are real and were taken from a build. Bump them
with the version.

## Smoke test

```sh
tunny --version          # bare version on stdout, exit 0
tunny --preview 80       # draws the list, exits 0, touches nothing
```

`--preview` reads the config and prints what the picker would show, without
opening a tunnel or writing any state. Point `XDG_CONFIG_HOME` at a temporary
directory holding `destinations.example.toml` and it stays entirely inside the
sandbox.
