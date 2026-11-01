package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/lsp"
)

// Quit is carried out by the TUI: tea.Quit, and the program is marked as ending.
func TestQuitEffect(t *testing.T) {
	a := newTestApp(t, "one\n")
	cmd := a.effectCmd(Quit{})
	if cmd == nil {
		t.Fatal("no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("Quit must end the program")
	}
	if !a.quitting {
		t.Fatal("quitting not set")
	}
}

func TestStartTimerDeliversEvent(t *testing.T) {
	a := newTestApp(t, "one\n")
	cmd := a.effectCmd(StartTimer{After: 1, Fire: HoverDwell{seq: 7}})
	if m, ok := cmd().(HoverDwell); !ok || m.seq != 7 {
		t.Fatalf("got %#v", cmd())
	}
}

// cmdOf is the tea.Cmd that carries out effs, for tests that run the result.
func cmdOf(a *App, effs []Effect) tea.Cmd { return a.effectCmd(effs...) }
