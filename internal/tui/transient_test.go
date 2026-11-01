package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExplorerClickTransientDoubleKeeps(t *testing.T) {
	a := newTestApp(t, "one\n")
	ps := writeFiles(t, a.root, "b.txt")
	a.explorer.refresh()
	a.explorer.reveal(ps[0])
	l := a.layout()
	y := l.top + 1 + a.explorer.sel - a.explorer.scroll
	click := func() {
		// the adapter counts the presses (App.clicks), not the core
		a.HandleMouse(a.locate(2, y, Mouse{Action: MousePress, Button: MouseLeft, Clicks: a.clicks(2, y)}))
		a.HandleMouse(a.locate(2, y, Mouse{Action: MouseRelease, Button: MouseLeft}))
	}
	click()
	if len(a.tabs) != 2 || !a.tabs[1].transient || a.focus != FocusExplorer {
		t.Fatalf("click: tabs = %d focus = %v", len(a.tabs), a.focus)
	}
	click()
	if len(a.tabs) != 2 || a.tabs[1].transient || a.focus != FocusEditor {
		t.Fatalf("double click: tabs = %d focus = %v", len(a.tabs), a.focus)
	}
}
