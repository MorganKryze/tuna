# SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
# SPDX-License-Identifier: GPL-3.0-only
#
# Destination names come from tunny itself rather than from a parsed config:
# not having to remember the name is the whole point of the tool, and a
# completion that guessed at the TOML would be a second parser to keep in step.
_tunny() {
    local cur=${COMP_WORDS[COMP_CWORD]}
    if [[ $cur == -* ]]; then
        COMPREPLY=($(compgen -W '--no-retry --preview --version --help' -- "$cur"))
        return
    fi
    COMPREPLY=($(compgen -W "$(tunny --list 2>/dev/null)" -- "$cur"))
}
complete -F _tunny tunny
