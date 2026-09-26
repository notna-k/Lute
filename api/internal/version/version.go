// Package version is core's build version, set with -ldflags "-X github.com/lute/api/internal/version.Version=0.2.0".
package version

import (
	"strconv"
	"strings"
)

var Version = "dev"

// Older reports whether version a is an older release than b. Anything that is not a
// plain x.y.z release, such as "dev", is never older.
func Older(a, b string) bool {
	pa, ok := parse(a)
	if !ok {
		return false
	}
	pb, ok := parse(b)
	if !ok {
		return false
	}
	for i := range pa {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return false
}

func parse(v string) ([3]int, bool) {
	var out [3]int
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// MinorTag is the image tag an agent of this core should run: "0.2" for 0.2.5, "latest" for dev builds.
func MinorTag(v string) string {
	p, ok := parse(v)
	if !ok {
		return "latest"
	}
	return strconv.Itoa(p[0]) + "." + strconv.Itoa(p[1])
}
