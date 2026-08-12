# SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
# SPDX-License-Identifier: GPL-3.0-only

complete -c tunny -f
complete -c tunny -n __fish_use_subcommand -a '(tunny --list 2>/dev/null)' -d destination
complete -c tunny -l no-retry -d 'one attempt, no reconnection'
complete -c tunny -l preview  -d 'print the list without opening anything'
complete -c tunny -l version  -d 'print the version and exit'
complete -c tunny -s h -l help -d 'print a usage summary'
