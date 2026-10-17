package ui

import (
	"runtime/debug"
	"strings"
)

// Version is set at build time with
// -ldflags "-X github.com/mm1995tk/gh-editor/internal/ui.Version=...".
var Version = ""

// versionLabel is the build's version as shown on the status line: the ldflags
// value, else the module version recorded by go install, else "dev".
func versionLabel() string {
	v := Version
	if v == "" {
		if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "(devel)" {
			v = bi.Main.Version
		}
	}
	if v == "" {
		return "dev"
	}
	return "v" + strings.TrimPrefix(v, "v")
}
