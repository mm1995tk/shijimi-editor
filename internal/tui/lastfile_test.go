package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// The title line shows the path, and is empty when no file is open.
func TestRenderTitle(t *testing.T) {
	a := newTestApp(t, "one\n")
	if got := a.renderTitle(a.Cur(), 40, true); !strings.Contains(ansi.Strip(got), a.Cur().title()) {
		t.Fatalf("title %q", got)
	}
	a.closeTab(true)
	if got := a.renderTitle(a.Cur(), 10, true); strings.TrimSpace(ansi.Strip(got)) != "" {
		t.Fatalf("title %q", got)
	}
}
