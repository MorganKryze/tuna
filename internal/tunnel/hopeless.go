// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

package tunnel

import "strings"

// hopelessMarkers are the failures that would repeat identically three times
// over, so retrying them only delays the message the operator needs. Anything
// else — host down, network gone, wifi switched, laptop woken from sleep —
// is worth another try, which is the whole reason this list is short and
// closed rather than a guess at what looks fatal.
var hopelessMarkers = []string{
	"Address already in use",     // another tunnel already holds the local port
	"Permission denied",          // the key does not get through
	"Could not resolve hostname", // the alias or the name is wrong
}

// Hopeless reports whether stderr shows a failure retrying cannot fix, and
// returns the offending line so the operator reads ssh's own words rather
// than a paraphrase of them.
func Hopeless(stderr string) (string, bool) {
	for _, line := range strings.Split(stderr, "\n") {
		for _, marker := range hopelessMarkers {
			if strings.Contains(line, marker) {
				return strings.TrimSpace(line), true
			}
		}
	}
	return "", false
}
