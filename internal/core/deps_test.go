package core

import (
	"os/exec"
	"strings"
	"testing"
)

// bannedDeps are the import path prefixes core must not depend on
// (docs/design/gui-protocol.md): the terminal UI libraries and the front end.
var bannedDeps = []string{
	"charm.land/",
	"github.com/charmbracelet/",
	"github.com/mm1995tk/shijimi-editor/internal/tui",
}

// allowedDeps are exceptions to bannedDeps. github.com/charmbracelet/x/ansi
// still measures text in terminal cells in the Markdown preview, the agent
// panel and a few more places that build rows to draw; those go to the front
// end in a later step.
var allowedDeps = []string{
	"github.com/charmbracelet/x/ansi",
}

func TestNoTerminalDeps(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
next:
	for dep := range strings.FieldsSeq(string(out)) {
		for _, ok := range allowedDeps {
			if dep == ok || strings.HasPrefix(dep, ok+"/") {
				continue next
			}
		}
		for _, banned := range bannedDeps {
			if strings.HasPrefix(dep, banned) {
				t.Errorf("internal/core depends on %s", dep)
			}
		}
	}
}
