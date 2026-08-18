# Dev recipes. `just` with no argument lists them.

default:
    @just --list

build:
    go build -o tunny ./cmd/tunny

test:
    go test ./...

# The race detector. Cheap, and it only means something now that the tests
# drive a real child process.
race:
    go test -race ./...

coverage:
    go test -covermode=atomic -coverprofile=cov.out ./...
    go tool cover -func=cov.out | tail -1

lint:
    golangci-lint run

# Fuzz the picker past the seed corpus `just test` already runs. Give it minutes.
fuzz time="60s":
    go test ./internal/pick/ -run xxx -fuzz FuzzTheFrameNeverOverflows -fuzztime {{time}}

# Compile for every platform PACKAGING.md promises.
cross:
    #!/usr/bin/env bash
    set -eu
    for t in linux/amd64 linux/arm64 linux/386 linux/arm linux/riscv64 \
             darwin/amd64 darwin/arm64 \
             freebsd/amd64 freebsd/arm64 netbsd/amd64 openbsd/amd64 openbsd/arm64; do
        GOOS="${t%%/*}" GOARCH="${t##*/}" go build ./... && echo "  ok    $t"
    done

# Redraws the README's pictures from the binary's real output.
# Re-run it whenever the picker's drawing changes.
shot: build
    python3 scripts/shot.py "$PWD/tunny" 78

# Run once after cloning: without it the hook does not exist.
hooks:
    git config core.hooksPath githooks
