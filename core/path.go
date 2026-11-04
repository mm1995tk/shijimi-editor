package core

import (
	"path/filepath"
	"strings"
)

// SkipDirs are the directories that file walks (project search, the file
// palette) do not descend into.
var SkipDirs = map[string]bool{".git": true, "node_modules": true, ".direnv": true, "target": true, "dist": true, ".next": true}

// RelTo is p relative to root, or p itself when it is outside root.
func RelTo(root, p string) string {
	if r, err := filepath.Rel(root, p); err == nil && !strings.HasPrefix(r, "..") {
		return r
	}
	return p
}
