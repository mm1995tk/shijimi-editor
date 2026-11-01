package core

import (
	"os/exec"
	"strings"
	"testing"
)

// core must stay free of the terminal UI libraries (docs/design/gui-protocol.md).
func TestNoTerminalDeps(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
	for _, dep := range strings.Fields(string(out)) {
		for _, banned := range []string{"charm.land/bubbletea", "charm.land/lipgloss", "github.com/charmbracelet/", "github.com/mm1995tk/shijimi-editor/internal/tui"} {
			if strings.HasPrefix(dep, banned) {
				t.Errorf("internal/core depends on %s", dep)
			}
		}
	}
}
