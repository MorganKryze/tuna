// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

package pick

import "unicode/utf8"

// readKey turns raw bytes into a Key. The hard case is that an arrow and a
// bare Escape start with the same byte: arrows arrive as ESC [ A, a lone ESC
// arrives alone. Read it wrong and either Escape never fires, or it fires on
// every arrow.
func readKey(buf []byte) (Key, rune) {
	switch {
	case len(buf) == 0:
		return KeyNone, 0
	case buf[0] == 0x1b && len(buf) >= 3 && buf[1] == '[':
		switch buf[2] {
		case 'A':
			return KeyUp, 0
		case 'B':
			return KeyDown, 0
		}
		// Left, right, Home and the rest: recognised as a sequence, then
		// ignored. Falling through to the bare-ESC case below would quit the
		// picker on a keystroke nobody meant as "cancel".
		return KeyNone, 0
	case buf[0] == 0x1b:
		return KeyEsc, 0
	case buf[0] == '\r' || buf[0] == '\n':
		return KeyEnter, 0
	case buf[0] == 0x7f || buf[0] == 0x08:
		return KeyBackspace, 0
	// Ctrl-C in raw mode is a byte, not a signal: the terminal is not
	// generating signals while we hold it.
	case buf[0] == 0x03:
		return KeyEsc, 0
	case buf[0] >= 0x20:
		// Decoded, not cast. Casting the first byte turned "é" into "Ã" and
		// dropped the second half, which then reached the filter as invalid
		// UTF-8 — against a config file that ships accented descriptions.
		//
		// One rune per read, deliberately: a paste arrives as one read and
		// only its first character lands. Filtering is a few keystrokes on a
		// list of a dozen, and buffering the rest would buy a case nobody
		// meets.
		r, size := utf8.DecodeRune(buf)
		if r == utf8.RuneError && size <= 1 {
			return KeyNone, 0 // a stray byte is not a character
		}
		return KeyRune, r
	}
	return KeyNone, 0
}
