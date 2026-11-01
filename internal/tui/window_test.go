package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/core"
)

func TestSplitShowsTwoFiles(t *testing.T) {
	a, ps := splitApp(t)
	if !a.Split || a.Pane != 1 {
		t.Fatalf("split=%v pane=%d", a.Split, a.Pane)
	}
	if a.Other == nil || a.Other.Path() != a.Tabs[0].Path() || a.Cur().Path() != ps[0] {
		t.Fatalf("other=%v cur=%v", a.Other, a.Cur())
	}
	out := ansi.Strip(a.render())
	for _, want := range []string{"a.txt", "b.txt"} {
		if !strings.Contains(out, want) {
			t.Fatalf("title %q missing:\n%s", want, out)
		}
	}
	l := a.layout()
	if l.otherW+l.rightW+1 != l.fullW || l.otherX >= l.rightX {
		t.Fatalf("layout %+v", l)
	}
	for row := range strings.SplitSeq(a.render(), "\n") {
		if w := ansi.StringWidth(row); w > a.w {
			t.Fatalf("row too wide (%d): %q", w, ansi.Strip(row))
		}
	}
}

func TestSplitMouseFocusesPane(t *testing.T) {
	a, ps := splitApp(t)
	l := a.layout()
	a.HandleMouse(a.locate(l.otherX+3, l.editY+1, core.Mouse{Action: core.MousePress, Button: core.MouseLeft}))
	if a.Pane != 0 || a.Other.Path() != ps[0] {
		t.Fatalf("pane=%d", a.Pane)
	}
}

func TestSplitOffInDiffAndSingle(t *testing.T) {
	a, _ := splitApp(t)
	a.Gd.On = true
	if l := a.layout(); l.split {
		t.Fatal("split in diff mode")
	}
	a.Gd.On = false
	if l := a.layout(); !l.split {
		t.Fatal("split did not come back")
	}
}

func TestSplitFocusMovesCommands(t *testing.T) {
	a, ps := splitApp(t)
	keys(a, "space", "w", "h")
	a.Update(core.Key{Code: KeyEscape})
	if a.Pane != 0 || a.Cur().Path() != a.Tabs[0].Path() || a.Other.Path() != ps[0] {
		t.Fatalf("pane=%d cur=%s", a.Pane, a.Cur().Path())
	}
	// Ctrl+2 goes back right
	a.HandleKey(core.Key{Code: '2', Mod: core.ModCtrl})
	if a.Pane != 1 || a.Cur().Path() != ps[0] {
		t.Fatalf("pane=%d cur=%s", a.Pane, a.Cur().Path())
	}
	// Space b puts the chosen file in the focused pane; the other pane keeps its file
	a.BufferPicker()
	a.Pal.OnPick(0)
	a.Pal.Close()
	a.Update(core.Key{Code: KeyEscape})
	if a.Cur().Path() != a.Tabs[0].Path() || a.Other.Path() != ps[0] {
		t.Fatalf("cur=%s other=%s", a.Cur().Path(), a.Other.Path())
	}
}

func TestSplitOpeningOtherPaneFileMovesFocus(t *testing.T) {
	a, ps := splitApp(t)
	left := a.Tabs[0].Path()
	a.OpenFile(left)
	a.Update(core.Key{Code: KeyEscape})
	if a.Pane != 0 || a.Cur().Path() != left || a.Other.Path() != ps[0] {
		t.Fatalf("pane=%d cur=%s other=%s", a.Pane, a.Cur().Path(), a.Other.Path())
	}
}

func TestSplitSwitchLastActsOnFocusedPane(t *testing.T) {
	a, ps := splitApp(t)
	a.OpenFile(ps[1]) // right pane: b -> c
	a.Update(core.Key{Code: KeyEscape})
	a.SwitchLast()
	a.Update(core.Key{Code: KeyEscape})
	if a.Cur().Path() != ps[0] || a.Pane != 1 || a.Other.Path() != a.Tabs[0].Path() {
		t.Fatalf("cur=%s pane=%d other=%s", a.Cur().Path(), a.Pane, a.Other.Path())
	}
}

func TestSplitClose(t *testing.T) {
	a, ps := splitApp(t)
	a.HandleKey(core.Key{Code: '\\', Mod: core.ModCtrl})
	if a.Split || a.Cur().Path() != ps[0] || a.Other != nil {
		t.Fatalf("split=%v cur=%s", a.Split, a.Cur().Path())
	}
	// Space w q keeps the other pane's file
	a.PrevTab = a.Tabs[0]
	keys(a, "space", "w", "v")
	keys(a, "space", "w", "q")
	if a.Split || a.Cur().Path() != ps[0] || len(a.Tabs) != 2 {
		t.Fatalf("split=%v cur=%s tabs=%d", a.Split, a.Cur().Path(), len(a.Tabs))
	}
}
