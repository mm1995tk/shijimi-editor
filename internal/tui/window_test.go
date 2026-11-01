package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestSplitShowsTwoFiles(t *testing.T) {
	a, ps := splitApp(t)
	if !a.split || a.pane != 1 {
		t.Fatalf("split=%v pane=%d", a.split, a.pane)
	}
	if a.other == nil || a.other.Path() != a.tabs[0].Path() || a.Cur().Path() != ps[0] {
		t.Fatalf("other=%v cur=%v", a.other, a.Cur())
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
	a.HandleMouse(a.locate(l.otherX+3, l.editY+1, Mouse{Action: MousePress, Button: MouseLeft}))
	if a.pane != 0 || a.other.Path() != ps[0] {
		t.Fatalf("pane=%d", a.pane)
	}
}

func TestSplitOffInDiffAndSingle(t *testing.T) {
	a, _ := splitApp(t)
	a.gd.on = true
	if l := a.layout(); l.split {
		t.Fatal("split in diff mode")
	}
	a.gd.on = false
	if l := a.layout(); !l.split {
		t.Fatal("split did not come back")
	}
}


func TestSplitFocusMovesCommands(t *testing.T) {
	a, ps := splitApp(t)
	keys(a, "space", "w", "h")
	a.Update(Key{Code: KeyEscape})
	if a.pane != 0 || a.Cur().Path() != a.tabs[0].Path() || a.other.Path() != ps[0] {
		t.Fatalf("pane=%d cur=%s", a.pane, a.Cur().Path())
	}
	// Ctrl+2 goes back right
	a.handleKey(Key{Code: '2', Mod: ModCtrl})
	if a.pane != 1 || a.Cur().Path() != ps[0] {
		t.Fatalf("pane=%d cur=%s", a.pane, a.Cur().Path())
	}
	// Space b puts the chosen file in the focused pane; the other pane keeps its file
	a.bufferPicker()
	a.pal.onPick(0)
	a.pal.close()
	a.Update(Key{Code: KeyEscape})
	if a.Cur().Path() != a.tabs[0].Path() || a.other.Path() != ps[0] {
		t.Fatalf("cur=%s other=%s", a.Cur().Path(), a.other.Path())
	}
}

func TestSplitOpeningOtherPaneFileMovesFocus(t *testing.T) {
	a, ps := splitApp(t)
	left := a.tabs[0].Path()
	a.openFile(left)
	a.Update(Key{Code: KeyEscape})
	if a.pane != 0 || a.Cur().Path() != left || a.other.Path() != ps[0] {
		t.Fatalf("pane=%d cur=%s other=%s", a.pane, a.Cur().Path(), a.other.Path())
	}
}

func TestSplitSwitchLastActsOnFocusedPane(t *testing.T) {
	a, ps := splitApp(t)
	a.openFile(ps[1]) // right pane: b -> c
	a.Update(Key{Code: KeyEscape})
	a.switchLast()
	a.Update(Key{Code: KeyEscape})
	if a.Cur().Path() != ps[0] || a.pane != 1 || a.other.Path() != a.tabs[0].Path() {
		t.Fatalf("cur=%s pane=%d other=%s", a.Cur().Path(), a.pane, a.other.Path())
	}
}

func TestSplitClose(t *testing.T) {
	a, ps := splitApp(t)
	a.handleKey(Key{Code: '\\', Mod: ModCtrl})
	if a.split || a.Cur().Path() != ps[0] || a.other != nil {
		t.Fatalf("split=%v cur=%s", a.split, a.Cur().Path())
	}
	// Space w q keeps the other pane's file
	a.prevTab = a.tabs[0]
	keys(a, "space", "w", "v")
	keys(a, "space", "w", "q")
	if a.split || a.Cur().Path() != ps[0] || len(a.tabs) != 2 {
		t.Fatalf("split=%v cur=%s tabs=%d", a.split, a.Cur().Path(), len(a.tabs))
	}
}
