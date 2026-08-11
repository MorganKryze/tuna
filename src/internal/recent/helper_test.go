package recent

import "github.com/MorganKryze/tuna/src/internal/config"

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
