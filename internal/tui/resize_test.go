package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func resizeApp(t *testing.T) *App {
	a := New(t.TempDir(), "")
	a.w, a.h = 120, 40
	return a
}

func press(a *App, key string) {
	a.handleKey(textKey(key))
}

func TestResizeKeys(t *testing.T) {
	a := resizeApp(t)
	a.showTerm = true
	side, panel := a.layout().sideW, a.layout().panelH

	a.startResize()
	a.handleKey(Key{Code: KeyRight})
	a.handleKey(Key{Code: KeyUp})
	l := a.layout()
	if l.sideW != side+resizeStep || l.panelH != panel+resizeStep {
		t.Fatalf("side %d->%d panel %d->%d", side, l.sideW, panel, l.panelH)
	}

	// keys do not leak to the editor while resizing, and Esc leaves the mode
	a.handleKey(Key{Code: KeyEsc})
	if a.resizeMode {
		t.Fatal("still in resize mode")
	}

	// the size is kept across close and reopen
	a.toggleSide()
	a.toggleSide()
	if a.layout().sideW != side+resizeStep {
		t.Fatalf("size lost: %d", a.layout().sideW)
	}

	a.startResize()
	press(a, "=")
	if l := a.layout(); l.sideW != side || l.panelH != panel {
		t.Fatalf("reset: %d %d", l.sideW, l.panelH)
	}
}

func TestResizeClamp(t *testing.T) {
	a := resizeApp(t)
	a.showTerm = true
	a.startResize()
	for range 200 {
		a.handleKey(Key{Code: KeyLeft})
		a.handleKey(Key{Code: KeyDown})
	}
	if l := a.layout(); l.sideW != minSideW || l.panelH != 6 {
		t.Fatalf("min: %d %d", l.sideW, l.panelH)
	}
	for range 200 {
		a.handleKey(Key{Code: KeyRight})
		a.handleKey(Key{Code: KeyUp})
	}
	l := a.layout()
	if l.sideW > a.w/2 || l.panelH > a.h-8 {
		t.Fatalf("max: %d %d", l.sideW, l.panelH)
	}
	// shrinking the window pulls the stored size back into range
	a.w, a.h = 40, 12
	l = a.layout()
	if l.rightW < 10 || l.editH < 0 {
		t.Fatalf("small window: %+v", l)
	}
}

func TestResizePreviewRatio(t *testing.T) {
	a := resizeApp(t)
	l := a.layout()
	def := a.editorWidth(l.rightW)
	if def != (l.rightW-1)/2 {
		t.Fatalf("default split changed: %d", def)
	}
	a.startResize()
	press(a, ">")
	if a.editorWidth(l.rightW) <= def {
		t.Fatal("editor should grow")
	}
	for range 50 {
		press(a, "<")
	}
	if w := a.editorWidth(l.rightW); w < minPaneW {
		t.Fatalf("editor too narrow: %d", w)
	}
}

func TestResizeMouseDrag(t *testing.T) {
	a := resizeApp(t)
	a.showTerm = true
	l := a.layout()
	mouse := func(act MouseAction, x, y int) {
		a.handleMouse(a.locate(Mouse{X: x, Y: y, Action: act, Button: MouseLeft}))
	}

	mouse(MousePress, l.sideW, 5)
	mouse(MouseMotion, 40, 5)
	mouse(MouseRelease, 40, 5)
	if a.layout().sideW != 40 || a.resizing != resizeNone {
		t.Fatalf("side drag: %d", a.layout().sideW)
	}

	mouse(MousePress, 60, l.panelY)
	mouse(MouseMotion, 60, l.panelY-5)
	mouse(MouseRelease, 60, l.panelY-5)
	if got := a.layout().panelH; got != l.panelH+5 {
		t.Fatalf("panel drag: %d want %d", got, l.panelH+5)
	}
}

func TestResizeWithCompletionOpen(t *testing.T) {
	a := resizeApp(t)
	a.showTerm = true
	a.comp = &completionPopup{}
	panel := a.layout().panelH
	a.startResize()
	a.handleKey(Key{Code: KeyUp})
	if a.layout().panelH != panel+resizeStep {
		t.Fatalf("panel %d->%d", panel, a.layout().panelH)
	}
}

func TestResizeLostRelease(t *testing.T) {
	a := resizeApp(t)
	a.resizing = resizeSide
	a.handleMouse(a.locate(Mouse{X: 60, Y: 5, Action: MousePress, Button: MouseLeft}))
	if a.resizing != resizeNone {
		t.Fatal("stale drag state kept")
	}
}

func TestDiffSplitResize(t *testing.T) {
	a := resizeApp(t)
	l := a.layout()
	w := l.rightW - 1
	if got := a.diffLeftWidth(w); got != (w-1)/2 {
		t.Fatalf("default %d", got)
	}

	a.resizing = resizeDiff
	a.dragResize(l, l.rightX+30, l.editY)
	if got := a.diffLeftWidth(w); got < 29 || got > 30 {
		t.Fatalf("drag to 30: %d", got)
	}

	a.dragResize(l, l.rightX, l.editY)
	if got := a.diffLeftWidth(w); got != minPaneW {
		t.Fatalf("left min: %d", got)
	}
	a.dragResize(l, l.rightX+w, l.editY)
	if got := a.diffLeftWidth(w); w-1-got < minPaneW {
		t.Fatalf("right min: left %d of %d", got, w)
	}

	a.resizing = resizeNone
	a.startResize()
	press(a, "=")
	if got := a.diffLeftWidth(w); got != (w-1)/2 {
		t.Fatalf("reset %d", got)
	}
}

func TestResizeKeysFollowFocus(t *testing.T) {
	a, _ := gitDiffApp(t)
	a.gd.split = true
	a.showTerm = true
	if a.render(); !a.splitShown() || a.diffFile(a.curText()) == nil {
		t.Skip("split is not shown")
	}
	l := a.layout()
	w := a.dvGeom(l).rightW + a.dvGeom(l).leftW + 1
	side, panel, left, tree := l.sideW, l.panelH, a.diffLeftWidth(w), a.explorerH()

	// the editor: ← → move the │ of the split diff, ↑ ↓ the terminal
	a.focus = focusEditor
	a.startResize()
	press(a, "l")
	press(a, "k")
	if got := a.diffLeftWidth(w); got <= left {
		t.Fatalf("diff left %d -> %d", left, got)
	}
	if l := a.layout(); l.sideW != side || l.panelH != panel+resizeStep {
		t.Fatalf("side %d->%d panel %d->%d", side, l.sideW, panel, l.panelH)
	}
	press(a, "h")
	press(a, "h")
	if got := a.diffLeftWidth(w); got >= left {
		t.Fatalf("diff left %d -> %d", left, got)
	}

	// CHANGES: ← → move the sidebar edge, ↑ ↓ the border above CHANGES
	a.focus = focusChanges
	press(a, "right")
	press(a, "up")
	if got := a.layout().sideW; got != side+resizeStep {
		t.Fatalf("side %d -> %d", side, got)
	}
	if got := a.explorerH(); got != tree-resizeStep {
		t.Fatalf("tree %d -> %d", tree, got)
	}
	press(a, "down")
	if got := a.explorerH(); got != tree {
		t.Fatalf("tree back %d -> %d", tree, got)
	}

	// the terminal: ← → move the sidebar edge
	a.focus = focusTerminal
	press(a, "left")
	if got := a.layout().sideW; got != side {
		t.Fatalf("side back %d -> %d", side, got)
	}
}

// On a wide screen one percent is more than a key step; the border still moves.
func TestResizeKeysWideScreen(t *testing.T) {
	a, _ := gitDiffApp(t)
	a.gd.split = true
	a.Update(tea.WindowSizeMsg{Width: 300, Height: 300})
	if a.render(); !a.splitShown() || a.diffFile(a.curText()) == nil {
		t.Skip("split is not shown")
	}
	a.focus = focusEditor
	a.startResize()
	for _, k := range []string{"l", "h"} {
		before := a.dvGeom(a.layout()).leftW
		press(a, k)
		if got := a.dvGeom(a.layout()).leftW; got == before {
			t.Fatalf("%s: diff left stays %d", k, got)
		}
	}
	a.focus = focusChanges
	for _, k := range []string{"j", "k"} {
		before := a.explorerH()
		press(a, k)
		if got := a.explorerH(); got == before {
			t.Fatalf("%s: tree stays %d", k, got)
		}
	}
}

func TestResizeKeysEditorBorders(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.md")
	os.WriteFile(file, []byte("# a\n"), 0o644)
	a := New(dir, file)
	a.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	a.focus = focusEditor
	a.startResize()

	// nothing on the right: the sidebar edge
	side := a.layout().sideW
	press(a, "l")
	if got := a.layout().sideW; got != side+resizeStep {
		t.Fatalf("side %d -> %d", side, got)
	}

	// the suggestions panel: its left edge
	a.showAgent = true
	agent := a.layout().agentW
	if agent == 0 {
		t.Fatal("panel not shown")
	}
	press(a, "h")
	if got := a.layout().agentW; got != agent+resizeStep {
		t.Fatalf("panel %d -> %d", agent, got)
	}

	// the Markdown preview comes before the panel
	a.curText().preview = previewSplit
	l := a.layout()
	edit := a.editW(a.curText(), l)
	press(a, "l")
	if got := a.editW(a.curText(), a.layout()); got <= edit {
		t.Fatalf("editor %d -> %d", edit, got)
	}
	if got := a.layout().agentW; got != agent+resizeStep {
		t.Fatalf("panel moved: %d", got)
	}
}
