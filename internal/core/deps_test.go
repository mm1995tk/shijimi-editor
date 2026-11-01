package core

import (
	"os/exec"
	"strings"
	"testing"
)

// bannedDeps are the import path prefixes core must not depend on
// (docs/design/gui-protocol.md). Later P13 steps add to this list.
var bannedDeps = []string{
	"charm.land/bubbletea",
	"charm.land/lipgloss",
	"github.com/charmbracelet/",
	"github.com/mm1995tk/shijimi-editor/internal/tui",
}

func TestNoTerminalDeps(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
	for dep := range strings.FieldsSeq(string(out)) {
		for _, banned := range bannedDeps {
			if strings.HasPrefix(dep, banned) {
				t.Errorf("internal/core depends on %s", dep)
			}
		}
	}
}
