<div align="center">

# tuna

**Pick your admin SSH tunnel from a list instead of remembering its name.**

[![Build](https://github.com/MorganKryze/tuna/actions/workflows/build.yml/badge.svg)](https://github.com/MorganKryze/tuna/actions/workflows/build.yml)
[![Security](https://github.com/MorganKryze/tuna/actions/workflows/security.yml/badge.svg)](https://github.com/MorganKryze/tuna/actions/workflows/security.yml)
[![Tests](https://raw.githubusercontent.com/MorganKryze/tuna/badges/tests.svg)](https://github.com/MorganKryze/tuna/actions/workflows/build.yml)
[![Coverage](https://raw.githubusercontent.com/MorganKryze/tuna/badges/coverage.svg)](https://github.com/MorganKryze/tuna/actions/workflows/build.yml)
[![Release](https://img.shields.io/github/v/release/MorganKryze/tuna?label=release&color=247b7b)](https://github.com/MorganKryze/tuna/releases)
[![License: GPL-3.0](https://img.shields.io/badge/license-GPL--3.0-blue.svg)](LICENSE)

[![Go](https://img.shields.io/badge/Go-single%20static%20binary-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Dependencies](https://img.shields.io/badge/dependencies-3-247b7b)](CONTRIBUTING.md#ground-rules)
[![Platforms](https://img.shields.io/badge/platforms-macOS%20%C2%B7%20Linux-lightgrey)](https://github.com/MorganKryze/tuna/releases)

<img src="docs/assets/picker.svg" alt="The tuna picker: a filter line with a match count, then one row per destination showing its name, its description and the local ports it will answer on" width="700">

</div>

> A tuna swims a long way and finds the same water again. So does this.

You run three or four machines with no mesh agent on them, on purpose. The
hypervisor, the control plane, the gateway. Their web UIs sit on a loopback
address you reach with `ssh -N -L`, the command runs long enough that you wrote
it into a `justfile`, and by now you have forgotten which recipe was which.
Change network and the tunnel dies without a word.

tuna is that `justfile`, with the names on screen and a tunnel that comes back.

## Install

```sh
go install github.com/MorganKryze/tuna/src/cmd/tuna@latest
```

Or take a binary from the [Releases](https://github.com/MorganKryze/tuna/releases)
page: darwin and linux, amd64 and arm64, with a `SHA256SUMS` and a build
attestation next to them.

## Getting started

```sh
mkdir -p ~/.config/tuna
curl -o ~/.config/tuna/destinations.toml \
  https://raw.githubusercontent.com/MorganKryze/tuna/main/destinations.example.toml
$EDITOR ~/.config/tuna/destinations.toml
tuna
```

Type to filter. tuna underlines the matched text as you go, and the count on
the right says how much is left:

```text
  ❯ dupl▏                                                                    2/3

  ▌ control-plane   Duplicati on the control plane                          8201
    gateway         Duplicati on the gateway                                8200
```

Arrows move, Enter opens, Escape gives up.

```sh
tuna              # the picker, most recent destination on top
tuna gateway      # straight to it, no picker
tuna --no-retry   # one attempt, no reconnection
tuna --preview    # what the list looks like, without opening anything
```

There is no `tuna add`, because the config is a file and a file belongs in an
editor. There is no `tuna list` either: the picker is the list.

## What a session looks like

```text
  ▌ hypervisor   Cockpit and Komodo, the admin root

    Cockpit   http://localhost:9090
    Komodo    http://localhost:9120

    ✓ tunnel open · Ctrl-C to close

    ⟳ connection lost, retrying 1/3 in 1s
    ✓ connection restored

    ✓ tunnel closed
```

`ssh -N` says nothing at all while it works, and nothing when it stops. Those
four lines are how you tell an open tunnel from a hung one, and a recovered one
from a closed one. The URLs go to stdout so you can pipe them somewhere;
everything else tuna says goes to stderr.

## Configuration

`~/.config/tuna/destinations.toml`, honouring `XDG_CONFIG_HOME`. One
destination is one ssh invocation, with as many `forward` entries as there are
UIs behind it.

```toml
[[destination]]
name = "hypervisor"
desc = "Cockpit and the orchestrator"
host = "my-host"
forward = [
  { local = 9090, to = "127.0.0.1:9090", label = "Cockpit" },
  { local = 9120, to = "10.0.0.5:9120", label = "Komodo" },
]

[[destination]]
name = "vm-backup"
desc = "Duplicati on a VM, reachable only from its host"
host = "debian@10.0.0.5"
port = 22022
jump = "my-host"
forward = [{ local = 8201, to = "127.0.0.1:8200", label = "Duplicati" }]
```

| Key             | Required | What it does                                                                                                 |
| --------------- | -------- | ------------------------------------------------------------------------------------------------------------ |
| `name`          | yes      | what you type after `tuna`, and what the picker shows. Unique.                                               |
| `desc`          | no       | the picker's second column, and part of what the filter searches.                                            |
| `host`          | yes      | ssh's target. An alias from `~/.ssh/config` is the point: it already carries the user, the port and the key. |
| `port`          | no       | becomes `-p`, for what the alias does not cover.                                                             |
| `jump`          | no       | becomes `-J`, for a VM reachable only from its host.                                                         |
| `forward`       | yes      | at least one. Each entry becomes one `-L`.                                                                   |
| `forward.local` | yes      | the port on your machine, 1 to 65535.                                                                        |
| `forward.to`    | yes      | where it lands on the far side, written `host:port`.                                                         |
| `forward.label` | no       | the name printed next to the URL. Skip it and you get a port number to read at 3am.                          |

`host`, `port` and `jump` hand the work to `~/.ssh/config` instead of copying
it. tuna builds an argument list and runs the real ssh binary, so your aliases,
`ProxyJump`, the agent, `known_hosts` and the host-key prompt keep working the
way you set them up.

tuna validates the file before it does anything: unique names, at least one
forward, ports in range, a `host:port` on the far side. A key it does not know
is an error too, since `forwards` instead of `forward` would otherwise leave
you a destination with no tunnel and no complaint.

Recency lives in `~/.local/state/tuna/recent`, honouring `XDG_STATE_HOME`. One
name per line, most recent first. The order is the data, so a file you can read
with `cat` you can repair with `vim`. Names that left the config get dropped
when tuna reads it, which is the whole cleanup mechanism.

## Reconnection

Three attempts per outage, waiting 1s, 2s and 4s.

Hold a tunnel for thirty seconds and the counter resets. It counts per outage
rather than per session, so a tunnel you leave open all day survives five
network changes while a destination that is genuinely down gives up in three
quick tries. `ssh -N` stays quiet when things go well, so how long the process
lived is the least fragile success signal available.

**Ctrl-C never relaunches.** tuna catches the interrupt itself rather than
reading ssh's exit code, because a tunnel you cannot kill is the worst thing
this program could do to you.

Two failures skip the retries and hand you ssh's own words, since both would
fail the same way three times over: `Permission denied` and
`Could not resolve hostname`. Host down, network gone, wifi switched, laptop
woken from sleep: all worth another try.

A third one never reaches ssh. tuna binds the local port first, the same
syscall `ssh -L` is about to make, so the picker marks a destination you cannot
open before you choose it:

```text
    control-plane   Duplicati on the control plane                  ● 8201 in use
```

Choose it anyway and you get the port and a way to find what holds it, instead
of three lines of ssh diagnostics arriving after a banner that promised a
tunnel:

```text
    ✗ control-plane: local port 8201 is already taken
      lsof -nP -iTCP:8201 -sTCP:LISTEN   shows what has it
```

`Address already in use` stays on the hopeless list as a backstop, since a port
can be taken in the moment between the check and the launch.

tuna always passes `-o ExitOnForwardFailure=yes`. Without it, ssh stays
connected with a dead forward and you find out from a browser tab minutes
later.

## Details you may care about

Colour follows [NO_COLOR](https://no-color.org), never reaches a pipe, and
`CLICOLOR_FORCE` overrides both. The layout gives way in a fixed order as your
terminal narrows: port labels first, then the ports, then the descriptions, so
no line ever wraps. Closing a tunnel leaves no `^C` on screen, because tuna
clears the terminal flag that echoes it and puts it back on the way out.

`tuna --preview` prints the list without opening anything, and takes a width so
you can check the layout at a size you do not own: `tuna --preview 60`.

## What tuna does not do

| Not here                                        | When to reconsider                                                                                                                                                     |
| ----------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| A generic `tuna ip port`                        | Never, absent proof otherwise. That is `ssh -N -L 8200:10.0.0.5:8200 my-host` typed by hand, and an ad-hoc form would cost an argument parser to save twenty characters. |
| Opening the browser                             | The day it genuinely grates. tuna prints the URL and your terminal makes it clickable, and the choice turns ambiguous the moment a destination has two forwards.        |
| Several tunnels at once, a daemon, `stop`, logs | When several terminal tabs become a nuisance. The cost sits after the launching: PIDs, a state file, orphans after a crash, and logs to store and to read.              |
| Mesh URLs, interactive ssh                      | Neither needs help. Mesh URLs already open in a browser, and `ssh hypervisor` types fine on its own.                                                                    |

## License

[GPL-3.0](LICENSE).
