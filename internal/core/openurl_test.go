package core

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestUrlAt(t *testing.T) {
	line := []rune("see (https://example.com/a?b=1). ok")
	for col, want := range map[int]string{5: "https://example.com/a?b=1", 30: "", 0: ""} {
		got, ok := urlAt(line, col)
		if got != want || ok != (want != "") {
			t.Errorf("col %d: got %q,%v want %q", col, got, ok, want)
		}
	}
}

// runCmd executes cmd and feeds its message back into the app.
func runCmd(a *App, cmd tea.Cmd) {
	if cmd != nil {
		a.Update(cmd())
	}
}

func TestOpenExternalMissingCommand(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := openExternal("https://example.com"); err == nil {
		t.Fatal("expected error when opener is missing")
	}
}
