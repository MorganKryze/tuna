# tuna

Pick an admin tunnel from a list instead of remembering its name, and keep it
alive when the wifi drops.

```text
  ❯ ▏tapez pour filtrer                                                      2/2

  ▌ hyperviseur   Cockpit + orchestrateur                             9090  9120
    vm-backup     Duplicati d'une VM, joignable seulement depuis l…         8201

    ↑↓ naviguer    ⏎ ouvrir    ⎋ annuler
```

Type to filter — the matched text is underlined as you go — arrows to move,
Enter to open. `tuna --preview` prints that list without opening anything,
which is also how the layout is checked at a width you do not have:
`tuna --preview 60`.

The tunnel then runs in the foreground and says what it is doing, because
`ssh -N` says nothing at all — without these lines an open tunnel, a dropped
one, a recovered one and a closed one look identical from the terminal:

```text
  ▌ hyperviseur   Cockpit + orchestrateur

    Cockpit   http://localhost:9090
    Komodo    http://localhost:9120

    ✓ tunnel ouvert · Ctrl-C pour fermer

    ⟳ connexion perdue — tentative 1/3 dans 1s
    ✓ connexion rétablie

    ✓ tunnel fermé
```

tuna exists because `just admin`, `just edge` and `just cp-backup` were three
names to remember, displayed nowhere, behind a tunnel that died at the first
network change and relaunched itself never. It replaces exactly that, and
nothing more: a `ssh -N -L` to the private UI of a machine that has no mesh
agent — a hypervisor, a control plane, a gateway.

## Install

```sh
go install github.com/MorganKryze/tuna/src/cmd/tuna@latest
```

Or grab a binary from the [Releases](https://github.com/MorganKryze/tuna/releases)
page — darwin and linux, amd64 and arm64, with a `SHA256SUMS` next to them.

## Getting started

```sh
mkdir -p ~/.config/tuna
curl -o ~/.config/tuna/destinations.toml \
  https://raw.githubusercontent.com/MorganKryze/tuna/main/destinations.example.toml
$EDITOR ~/.config/tuna/destinations.toml
tuna
```

There is no `tuna add`: the config is a file, and a file is edited in an
editor. There is no `tuna list` either — the picker **is** the list.

```sh
tuna              # the picker, most recent destination on top
tuna hyperviseur  # straight to it, no picker
tuna --no-retry   # one attempt, no reconnection
tuna --preview    # what the list looks like, without opening anything
```

## Configuration

`~/.config/tuna/destinations.toml`, honouring `XDG_CONFIG_HOME`. One
destination is one ssh invocation, with as many `forward` entries as there are
UIs behind it.

```toml
[[destination]]
name = "hyperviseur"
desc = "Cockpit + orchestrateur"
host = "mon-hote"
forward = [
  { local = 9090, to = "127.0.0.1:9090", label = "Cockpit" },
  { local = 9120, to = "10.0.0.5:9120", label = "Komodo" },
]

[[destination]]
name = "vm-backup"
desc = "Duplicati d'une VM, joignable seulement depuis l'hôte"
host = "debian@10.0.0.5"
port = 22022
jump = "mon-hote"
forward = [{ local = 8201, to = "127.0.0.1:8200", label = "Duplicati" }]
```

| Key             | Required | What it does                                                                                                       |
| --------------- | -------- | ------------------------------------------------------------------------------------------------------------------ |
| `name`          | yes      | what you type after `tuna`, and what the picker shows. Unique.                                                     |
| `desc`          | no       | the picker's second column, and part of what the filter searches.                                                  |
| `host`          | yes      | ssh's target. An alias from `~/.ssh/config` is the point: it already carries the user, the port and the key.       |
| `port`          | no       | becomes `-p`. Only for what the alias does not cover.                                                              |
| `jump`          | no       | becomes `-J`. A VM reachable only from its host.                                                                   |
| `forward`       | yes      | at least one. Each entry becomes one `-L`.                                                                         |
| `forward.local` | yes      | the port on your machine, 1–65535.                                                                                 |
| `forward.to`    | yes      | where it lands on the far side, written `host:port`.                                                               |
| `forward.label` | no       | the name printed next to the URL. Without it you get a port number, which is a worse thing to read at 3am.         |

`host`, `port` and `jump` delegate to `~/.ssh/config` rather than duplicating
it. tuna never reimplements ssh: it builds an argument list and runs the real
binary, so aliases, `ProxyJump`, the agent, `known_hosts` and the host-key
prompt all keep working the way you already configured them.

The file is validated at startup — unique names, at least one forward, ports
in range, a `host:port` on the far side — so a mistake fails with the offending
line rather than in the middle of an `ssh`. A key tuna does not know is an
error too: `forwards` instead of `forward` would otherwise be a destination
with no tunnel and no complaint.

Recency lives in `~/.local/state/tuna/recent` (honouring `XDG_STATE_HOME`),
one name per line, most recent first. The order **is** the data — no
timestamps, no JSON — so a file you can read with `cat` you can repair with
`vim`. Names that no longer exist in the config are dropped on read, which is
the entire cleanup mechanism.

## Reconnection

Three attempts per outage, with 1s, 2s and 4s between them.

A tunnel that held for 30 seconds resets the counter: the counter is per
outage, not per session, so a tunnel left open all day survives five network
changes while a destination that is genuinely down gives up in three quick
tries. `ssh -N` says nothing when things go well, so the lifetime of the
process is the least fragile success signal available.

**Ctrl-C never relaunches.** It is caught in the binary rather than inferred
from ssh's exit code, because a tunnel you cannot kill is the worst bug this
program could have.

Two failures skip the retries entirely and report ssh's own words, because
both would fail identically three times over: `Permission denied` and
`Could not resolve hostname`. Everything else — host down, network gone, wifi
switched, laptop woken from sleep — is worth another try.

A third, a local port already taken, never reaches ssh at all. It is knowable
before launching — binding the port is the same syscall `ssh -L` is about to
make — so the picker marks the destination before you choose it, and choosing
it anyway fails with the port and a way to find out what holds it, instead of
three lines of ssh diagnostics arriving after a banner that promised a tunnel:

```text
    control-plane   Duplicati du control-plane (Mongo Komodo)       ● 8201 pris
```

```text
tuna: control-plane : le port local 8201 est déjà pris
      lsof -nP -iTCP:8201 -sTCP:LISTEN   dit qui l'occupe
```

`Address already in use` stays on the hopeless list as a backstop: a port can
be taken in the moment between the check and the launch.

`-o ExitOnForwardFailure=yes` is always passed. Without it ssh stays connected
with a dead forward and you find out from a browser tab, minutes later.

"Came up" is read from ssh surviving three seconds, which is a proxy and worth
knowing as one: `-o ExitOnForwardFailure=yes` makes ssh exit within a moment
when a forward cannot be bound, so surviving that long means the forwards were
accepted. The case it gets wrong is a host that silently drops packets, where
ssh sits in connect for a minute and tuna will have called it open. The exact
answer would be to check that something is listening on the local port, and
both ways of asking are worse: binding it races with ssh's own bind — losing
that race causes the very failure it would be reporting on — and dialling it
opens a real connection through the tunnel to the service on the far side.

## What tuna does not do

| Not here                | When to reconsider                                                                                                                                        |
| ----------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| A generic `tuna ip port`| Never, absent proof otherwise. It is `ssh -N -L 8200:10.0.0.5:8200 mon-hote` typed by hand; an ad-hoc form would cost an argument parser to save 20 characters. |
| Opening the browser     | The day it genuinely grates. The URL is printed and terminals make it clickable, and it is ambiguous the moment a destination has two forwards.            |
| Several tunnels at once, a daemon, `stop`, logs | When several terminal tabs become a nuisance. The real cost is not the launching: it is PIDs, a state file, orphans after a crash, and logs to store and read. |
| Mesh URLs, interactive ssh | Neither needs help. Mesh URLs already open in a browser; `ssh hypervisor` types fine on its own.                                                        |

Closing a tunnel leaves no `^C` behind: that character is echoed by the
terminal driver rather than printed by tuna, and the flag responsible is
cleared while a tunnel is running. Colour follows
[NO_COLOR](https://no-color.org) and never reaches a pipe, and
the layout gives way in a fixed order as the terminal narrows — port labels
first, then the ports, then the descriptions — so nothing ever wraps.

The interface speaks French, because that is who it was built for. The code and
its comments are English.

## License

[GPL-3.0](LICENSE).
