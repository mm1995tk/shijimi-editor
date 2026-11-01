package tui

import (
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"testing"
)

func TestExplorerClickTransientDoubleKeeps(t *testing.T) {
	a := newTestApp(t, "one\n")
	ps := writeFiles(t, a.Root, "b.txt")
	a.Explorer.Refresh()
	a.Explorer.Reveal(ps[0])
	l := a.layout()
	y := l.top + 1 + a.Explorer.Sel - a.Explorer.Scroll
	click := func() {
		// the adapter counts the presses (App.clicks), not the core
		a.HandleMouse(a.locate(2, y, core.Mouse{Action: core.MousePress, Button: core.MouseLeft, Clicks: a.clicks(2, y)}))
		a.HandleMouse(a.locate(2, y, core.Mouse{Action: core.MouseRelease, Button: core.MouseLeft}))
	}
	click()
	if len(a.Tabs) != 2 || !a.Tabs[1].Transient || a.Focus != core.FocusExplorer {
		t.Fatalf("click: tabs = %d focus = %v", len(a.Tabs), a.Focus)
	}
	click()
	if len(a.Tabs) != 2 || a.Tabs[1].Transient || a.Focus != core.FocusEditor {
		t.Fatalf("double click: tabs = %d focus = %v", len(a.Tabs), a.Focus)
	}
}
