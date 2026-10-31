package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// ga and Ctrl+Tab go back and forth between the last two files.
func TestSwitchLastFile(t *testing.T) {
	a := newTestApp(t, "one\n")
	ps := writeFiles(t, a.root, "b.txt", "c.txt")
	a.trackLast()
	for _, p := range ps {
		a.openFile(p)
		a.trackLast()
	}
	a.switchLast()
	a.trackLast()
	if a.tabs[a.active].path() != ps[0] {
		t.Fatalf("active = %q", a.tabs[a.active].path())
	}
	a.handleKey(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModCtrl})
	a.trackLast()
	if a.tabs[a.active].path() != ps[1] {
		t.Fatalf("active = %q", a.tabs[a.active].path())
	}
}

// The title line shows the path, and is empty when no file is open.
func TestRenderTitle(t *testing.T) {
	a := newTestApp(t, "one\n")
	if got := a.renderTitle(40); !strings.Contains(ansi.Strip(got), a.cur().title()) {
		t.Fatalf("title %q", got)
	}
	a.closeTab(true)
	if got := a.renderTitle(10); strings.TrimSpace(ansi.Strip(got)) != "" {
		t.Fatalf("title %q", got)
	}
}

// The old tab-switching keys do nothing now.
func TestOldTabKeysGone(t *testing.T) {
	a := newTestApp(t, "one\n")
	a.openFile(writeFiles(t, a.root, "b.txt")[0])
	for _, k := range []tea.KeyPressMsg{
		{Code: tea.KeyPgUp, Mod: tea.ModCtrl},
		{Code: tea.KeyPgDown, Mod: tea.ModCtrl},
		{Code: '1', Mod: tea.ModAlt},
	} {
		a.handleKey(k)
		if a.active != 1 {
			t.Fatalf("%v switched to %d", k, a.active)
		}
	}
}
