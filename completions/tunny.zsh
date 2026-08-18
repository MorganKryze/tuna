#compdef tunny
# SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
# SPDX-License-Identifier: GPL-3.0-only

_tunny() {
    _arguments -s \
        '--no-retry[one attempt, no reconnection]' \
        '--preview[print the list without opening anything]' \
        '--width[with --preview, draw at this many columns]:columns:' \
        '--list[print destination names, one per line]' \
        '--version[print the version and exit]' \
        '(-h --help)'{-h,--help}'[print a usage summary]' \
        '1::destination:->dest'
    case $state in
        dest) _values 'destination' ${(f)"$(tunny --list 2>/dev/null)"} ;;
    esac
}
_tunny "$@"
