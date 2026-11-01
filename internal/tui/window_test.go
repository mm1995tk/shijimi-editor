package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// splitApp is an app with a.txt open on the left and b.txt on the right.
func splitApp(t *testing.T) (*App, []string) {
	t.Helper()
	a := newTestApp(t, "one\n")
	ps := writeFiles(t, a.root, "b.txt", "c.txt")
	a.Update(Key{Code: KeyEscape})
	a.openFile(ps[0])
	a.trackLast()
	a.openFile(a.tabs[0].path())
	a.trackLast()
	keys(a, "space", "w", "v")
	return a, ps
}

func TestSplitShowsTwoFiles(t *testing.T) {
	a, ps := splitApp(t)
	if !a.split || a.pane != 1 {
		t.Fatalf("split=%v pane=%d", a.split, a.pane)
	}
	if a.other == nil || a.other.path() != a.tabs[0].path() || a.cur().path() != ps[0] {
		t.Fatalf("other=%v cur=%v", a.other, a.cur())
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

func TestSplitFocusMovesCommands(t *testing.T) {
	a, ps := splitApp(t)
	keys(a, "space", "w", "h")
	a.Update(Key{Code: KeyEscape})
	if a.pane != 0 || a.cur().path() != a.tabs[0].path() || a.other.path() != ps[0] {
		t.Fatalf("pane=%d cur=%s", a.pane, a.cur().path())
	}
	// Ctrl+2 goes back right
	a.handleKey(Key{Code: '2', Mod: ModCtrl})
	if a.pane != 1 || a.cur().path() != ps[0] {
		t.Fatalf("pane=%d cur=%s", a.pane, a.cur().path())
	}
	// Space b puts the chosen file in the focused pane; the other pane keeps its file
	a.bufferPicker()
	a.pal.onPick(0)
	a.pal.close()
	a.Update(Key{Code: KeyEscape})
	if a.cur().path() != a.tabs[0].path() || a.other.path() != ps[0] {
		t.Fatalf("cur=%s other=%s", a.cur().path(), a.other.path())
	}
}

func TestSplitOpeningOtherPaneFileMovesFocus(t *testing.T) {
	a, ps := splitApp(t)
	left := a.tabs[0].path()
	a.openFile(left)
	a.Update(Key{Code: KeyEscape})
	if a.pane != 0 || a.cur().path() != left || a.other.path() != ps[0] {
		t.Fatalf("pane=%d cur=%s other=%s", a.pane, a.cur().path(), a.other.path())
	}
}

func TestSplitSwitchLastActsOnFocusedPane(t *testing.T) {
	a, ps := splitApp(t)
	a.openFile(ps[1]) // right pane: b -> c
	a.Update(Key{Code: KeyEscape})
	a.switchLast()
	a.Update(Key{Code: KeyEscape})
	if a.cur().path() != ps[0] || a.pane != 1 || a.other.path() != a.tabs[0].path() {
		t.Fatalf("cur=%s pane=%d other=%s", a.cur().path(), a.pane, a.other.path())
	}
}

func TestSplitClose(t *testing.T) {
	a, ps := splitApp(t)
	a.handleKey(Key{Code: '\\', Mod: ModCtrl})
	if a.split || a.cur().path() != ps[0] || a.other != nil {
		t.Fatalf("split=%v cur=%s", a.split, a.cur().path())
	}
	// Space w q keeps the other pane's file
	a.prevTab = a.tabs[0]
	keys(a, "space", "w", "v")
	keys(a, "space", "w", "q")
	if a.split || a.cur().path() != ps[0] || len(a.tabs) != 2 {
		t.Fatalf("split=%v cur=%s tabs=%d", a.split, a.cur().path(), len(a.tabs))
	}
}

func TestSplitCloseFileCollapses(t *testing.T) {
	a, ps := splitApp(t)
	a.closeTab(true)
	a.Update(Key{Code: KeyEscape})
	if a.split || a.cur() == nil || a.cur().path() != a.tabs[0].path() || ps == nil {
		t.Fatalf("split=%v cur=%v", a.split, a.cur())
	}
}

func TestSplitMouseFocusesPane(t *testing.T) {
	a, ps := splitApp(t)
	l := a.layout()
	a.handleMouse(a.locate(Mouse{X: l.otherX + 3, Y: l.editY + 1, Action: MousePress, Button: MouseLeft}))
	if a.pane != 0 || a.other.path() != ps[0] {
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
