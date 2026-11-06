package version

import (
	"regexp"
	"strings"
)

// Invalid versions (including development builds) are never considered newer.
func newerSemver(latest, current string) bool {
	pattern := regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
	a, b := pattern.FindStringSubmatch(latest), pattern.FindStringSubmatch(current)
	if a == nil || b == nil || !validPrerelease(a[4]) || !validPrerelease(b[4]) {
		return false
	}
	for i := 1; i <= 3; i++ {
		if cmp := compareNumeric(a[i], b[i]); cmp != 0 {
			return cmp > 0
		}
	}
	if a[4] == b[4] {
		return false
	}
	if a[4] == "" || b[4] == "" {
		return a[4] == ""
	}
	x, y := strings.Split(a[4], "."), strings.Split(b[4], ".")
	for i := 0; i < len(x) && i < len(y); i++ {
		if x[i] == y[i] {
			continue
		}
		xn, yn := numeric(x[i]), numeric(y[i])
		if xn && yn {
			return compareNumeric(x[i], y[i]) > 0
		}
		if xn != yn {
			return !xn
		}
		return x[i] > y[i]
	}
	return len(x) > len(y)
}

func numeric(s string) bool {
	return strings.Trim(s, "0123456789") == ""
}

func validPrerelease(s string) bool {
	for _, part := range strings.Split(s, ".") {
		if numeric(part) && len(part) > 1 && part[0] == '0' {
			return false
		}
	}
	return true
}

func compareNumeric(a, b string) int {
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return strings.Compare(a, b)
}
