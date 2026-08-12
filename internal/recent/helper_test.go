// SPDX-FileCopyrightText: 2026 Morgan Kryze <contact@libresoftware.cloud>
// SPDX-License-Identifier: GPL-3.0-only

package recent

import "github.com/MorganKryze/tunny/internal/config"

func names(dests []config.Destination) []string {
	out := make([]string, len(dests))
	for i, d := range dests {
		out[i] = d.Name
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
