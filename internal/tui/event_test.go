package tui

import (
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// Quit is carried out by the TUI: tea.Quit, and the program is marked as ending.
func TestQuitEffect(t *testing.T) {
	a := newTestApp(t, "one\n")
	cmd := a.effectCmd(core.Quit{})
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
	cmd := a.effectCmd(core.StartTimer{After: 1, Fire: core.HoverDwell{Seq: 7}})
	if m, ok := cmd().(core.HoverDwell); !ok || m.Seq != 7 {
		t.Fatalf("got %#v", cmd())
	}
}

// cmdOf is the tea.Cmd that carries out effs, for tests that run the result.
func cmdOf(a *App, effs []core.Effect) tea.Cmd { return a.effectCmd(effs...) }
