package ui

import (
	"runtime/debug"
	"strings"
)

// Version and Commit are set at build time with
// -ldflags "-X github.com/mm1995tk/gh-editor/internal/ui.Version=... -X github.com/mm1995tk/gh-editor/internal/ui.Commit=...".
var Version, Commit = "", ""

// versionLabel is the build's version as shown on the status line, such as
// "v0.1 (be64521)". Values missing from ldflags come from the build info:
// the module version recorded by go install and the commit recorded by go build.
func versionLabel() string {
	v, c := Version, Commit
	if bi, ok := debug.ReadBuildInfo(); ok {
		if v == "" && bi.Main.Version != "(devel)" {
			v = bi.Main.Version
		}
		if c == "" {
			c = buildSetting(bi, "vcs.revision")
			if c != "" && buildSetting(bi, "vcs.modified") == "true" {
				c += "-dirty"
			}
		}
	}
	return formatVersion(v, c)
}

func buildSetting(bi *debug.BuildInfo, key string) string {
	for _, s := range bi.Settings {
		if s.Key == key {
			return s.Value
		}
	}
	return ""
}

func formatVersion(v, commit string) string {
	label := "dev"
	if v != "" {
		label = "v" + strings.TrimPrefix(v, "v")
	}
	if commit == "" {
		return label
	}
	hash, dirty := strings.CutSuffix(commit, "-dirty")
	if len(hash) > 7 {
		hash = hash[:7]
	}
	if dirty {
		hash += "-dirty"
	}
	return label + " (" + hash + ")"
}
