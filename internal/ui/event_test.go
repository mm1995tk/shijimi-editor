package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// The core answers an Event with Effects and never touches bubbletea.
func TestWatchTickEffects(t *testing.T) {
	a := newTestApp(t, "one\n")
	effs := a.handleEvent(WatchTick{})
	var timer *StartTimer
	for _, e := range effs {
		if st, ok := e.(StartTimer); ok {
			timer = &st
		}
	}
	if timer == nil || timer.After != watchInterval {
		t.Fatalf("WatchTick must schedule the next one: %#v", effs)
	}
	if _, ok := timer.Fire.(WatchTick); !ok {
		t.Fatalf("timer fires %T", timer.Fire)
	}
}

func TestStaleHoldScrollTickIsDropped(t *testing.T) {
	a := newTestApp(t, "one\n")
	if effs := a.handleEvent(HoldScrollTick{seq: a.holdSeq + 1}); len(effs) != 0 {
		t.Fatalf("stale tick produced %#v", effs)
	}
}

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
