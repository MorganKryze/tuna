<!-- Before cutting the release, check the README still shows what the picker
     actually looks like:  just build && ./tuna --preview
     Update the two code blocks at the top if the drawing has moved.

     Create the release with these notes, which also creates the tag and so
     starts the workflow:  gh release create vX.Y.Z --notes-file …
     The workflow then attaches the binaries to it.

     Sections in this order; delete a section entirely when it is empty. -->

One or two sentences: what this release is about, in plain words.

## ⚠️ Breaking

- What breaks, who is affected, and the exact migration step. A config key
  that changed name belongs here with both spellings. Never leave it
  implied; delete the section only when there is truly nothing.

## ✨ Features

- New capability, phrased from the side of someone opening a tunnel: the
  config key, the line on screen, the behaviour they will notice.

## 🐛 Fixes

- What was wrong, now right.

## 🧹 Internal

- Chores, CI, docs, dependencies: anything invisible from the terminal.

## 📦 Binaries

```sh
go install github.com/MorganKryze/tuna/src/cmd/tuna@vX.Y.Z
```

Or download one below — darwin and linux, amd64 and arm64. `SHA256SUMS` is
attached next to them, and every file carries a build attestation tying it to
the workflow that produced it:

```sh
sha256sum -c SHA256SUMS --ignore-missing
gh attestation verify tuna-darwin-arm64 --repo MorganKryze/tuna
```
