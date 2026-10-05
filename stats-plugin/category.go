package main

import "strings"

// classify maps an exact DCS type id to a broad category, so the "Aircraft &
// vehicles" tab can group and label rows. It is a deliberately small heuristic:
// the manager has a fuller classifier backed by a categories file, but the
// plugin only needs a display hint and must not depend on the manager's
// internals. An unknown type returns "other".
func classify(typeID string) string {
	t := strings.ToLower(typeID)
	switch {
	case hasAnyPrefix(t, "uh-", "ah-", "mi-8", "mi-24", "mi-28", "ka-50", "ka-52", "oh-58") ||
		strings.Contains(t, "heli"):
		return "heli"
	case hasAnyPrefix(t, "f-", "f/a-", "a-10", "su-", "mig-", "jf-", "mirage", "av-8", "c-", "b-", "yak-", "l-39"):
		return "plane"
	case hasAnyPrefix(t, "uss_", "cvn", "cv-") || strings.Contains(t, "frigate") ||
		strings.Contains(t, "destroyer") || strings.Contains(t, "cruiser") || strings.Contains(t, "carrier") ||
		strings.Contains(t, "submarine"):
		return "ship"
	case hasAnyPrefix(t, "t-", "btr", "bmp", "m-", "sa-", "s-300", "s-400") ||
		strings.Contains(t, "sam") || strings.Contains(t, "aaa") || strings.Contains(t, "howitzer"):
		return "ground"
	default:
		return "other"
	}
}

func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
