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

# Redraws the README's pictures from the binary's real output.
# Re-run it whenever the picker's drawing changes.
shot: build
    python3 scripts/shot.py "$PWD/tunny" 78

# Run once after cloning: without it the hook does not exist.
hooks:
    git config core.hooksPath githooks
