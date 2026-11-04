package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/core"
)

// The core's Exit (the Port) ends the program: tea.Quit, and it is marked as ending.
func TestExitEndsProgram(t *testing.T) {
	a := newTestApp(t, "one\n")
	a.Exit()
	cmd := a.effectCmd()
	if cmd == nil {
		t.Fatal("no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("Exit must end the program")
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
