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

# Régénère la capture du README depuis la sortie réelle du binaire.
# À relancer dès que le dessin du picker change.
shot: build
    python3 scripts/shot.py "$PWD/tuna" 78

# À lancer une fois après le clone : sans ça, le hook n'existe pas.
hooks:
    git config core.hooksPath githooks
