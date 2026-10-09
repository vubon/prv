package prv

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var rawVersion string

// Version returns the single source of truth version read directly from the VERSION file at compile time.
func Version() string {
	v := strings.TrimSpace(rawVersion)
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}
