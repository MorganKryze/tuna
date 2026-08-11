# Recettes de dev. `just` sans argument les liste.

default:
    @just --list

build:
    go build -o tuna ./src/cmd/tuna

test:
    go test ./...

coverage:
    go test -covermode=atomic -coverprofile=cov.out ./...
    go tool cover -func=cov.out | tail -1

lint:
    golangci-lint run

# À lancer une fois après le clone : sans ça, le hook n'existe pas.
hooks:
    git config core.hooksPath githooks
