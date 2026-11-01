package tui

import (
	"github.com/mm1995tk/shijimi-editor/internal/core"
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
	a.HandleKey(textKey(key))
}

func TestResizeKeys(t *testing.T) {
	a := resizeApp(t)
	a.showTerm = true
	side, panel := a.layout().sideW, a.layout().panelH

	a.StartResize()
	a.HandleKey(core.Key{Code: KeyRight})
	a.HandleKey(core.Key{Code: KeyUp})
	l := a.layout()
	if l.sideW != side+resizeStep || l.panelH != panel+resizeStep {
		t.Fatalf("side %d->%d panel %d->%d", side, l.sideW, panel, l.panelH)
	}

	// keys do not leak to the editor while resizing, and Esc leaves the mode
	a.HandleKey(core.Key{Code: KeyEsc})
	if a.resizeMode {
		t.Fatal("still in resize mode")
	}

	// the size is kept across close and reopen
	a.ToggleSide()
	a.ToggleSide()
	if a.layout().sideW != side+resizeStep {
		t.Fatalf("size lost: %d", a.layout().sideW)
	}

	a.StartResize()
	press(a, "=")
	if l := a.layout(); l.sideW != side || l.panelH != panel {
		t.Fatalf("reset: %d %d", l.sideW, l.panelH)
	}
}

func TestResizeClamp(t *testing.T) {
	a := resizeApp(t)
	a.showTerm = true
	a.StartResize()
	for range 200 {
		a.HandleKey(core.Key{Code: KeyLeft})
		a.HandleKey(core.Key{Code: KeyDown})
	}
	if l := a.layout(); l.sideW != minSideW || l.panelH != 6 {
		t.Fatalf("min: %d %d", l.sideW, l.panelH)
	}
	for range 200 {
		a.HandleKey(core.Key{Code: KeyRight})
		a.HandleKey(core.Key{Code: KeyUp})
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
	a.StartResize()
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
	mouse := func(act core.MouseAction, x, y int) {
		a.HandleMouse(a.locate(x, y, core.Mouse{Action: act, Button: core.MouseLeft}))
	}

	mouse(core.MousePress, l.sideW, 5)
	mouse(core.MouseMotion, 40, 5)
	mouse(core.MouseRelease, 40, 5)
	if a.layout().sideW != 40 || a.resizing != core.ResizeNone {
		t.Fatalf("side drag: %d", a.layout().sideW)
	}

	mouse(core.MousePress, 60, l.panelY)
	mouse(core.MouseMotion, 60, l.panelY-5)
	mouse(core.MouseRelease, 60, l.panelY-5)
	if got := a.layout().panelH; got != l.panelH+5 {
		t.Fatalf("panel drag: %d want %d", got, l.panelH+5)
	}
}

func TestResizeWithCompletionOpen(t *testing.T) {
	a := resizeApp(t)
	a.showTerm = true
	a.Comp = &core.CompletionPopup{}
	panel := a.layout().panelH
	a.StartResize()
	a.HandleKey(core.Key{Code: KeyUp})
	if a.layout().panelH != panel+resizeStep {
		t.Fatalf("panel %d->%d", panel, a.layout().panelH)
	}
}

func TestResizeLostRelease(t *testing.T) {
	a := resizeApp(t)
	a.resizing = core.ResizeSide
	a.HandleMouse(a.locate(60, 5, core.Mouse{Action: core.MousePress, Button: core.MouseLeft}))
	if a.resizing != core.ResizeNone {
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

	a.resizing = core.ResizeDiff
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

	a.resizing = core.ResizeNone
	a.StartResize()
	press(a, "=")
	if got := a.diffLeftWidth(w); got != (w-1)/2 {
		t.Fatalf("reset %d", got)
	}
}

func TestResizeKeysFollowFocus(t *testing.T) {
	a, _ := gitDiffApp(t)
	a.Gd.Split = true
	a.showTerm = true
	if a.render(); !a.SplitShown() || a.DiffFile(a.CurText()) == nil {
		t.Skip("split is not shown")
	}
	l := a.layout()
	w := a.dvGeom(l).RightW + a.dvGeom(l).LeftW + 1
	side, panel, left, tree := l.sideW, l.panelH, a.diffLeftWidth(w), a.ExplorerH()

	// the editor: ← → move the │ of the split diff, ↑ ↓ the terminal
	a.Focus = core.FocusEditor
	a.StartResize()
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
	a.Focus = core.FocusChanges
	press(a, "right")
	press(a, "up")
	if got := a.layout().sideW; got != side+resizeStep {
		t.Fatalf("side %d -> %d", side, got)
	}
	if got := a.ExplorerH(); got != tree-resizeStep {
		t.Fatalf("tree %d -> %d", tree, got)
	}
	press(a, "down")
	if got := a.ExplorerH(); got != tree {
		t.Fatalf("tree back %d -> %d", tree, got)
	}

	// the terminal: ← → move the sidebar edge
	a.Focus = core.FocusTerminal
	press(a, "left")
	if got := a.layout().sideW; got != side {
		t.Fatalf("side back %d -> %d", side, got)
	}
}

// On a wide screen one percent is more than a key step; the border still moves.
func TestResizeKeysWideScreen(t *testing.T) {
	a, _ := gitDiffApp(t)
	a.Gd.Split = true
	a.Update(tea.WindowSizeMsg{Width: 300, Height: 300})
	if a.render(); !a.SplitShown() || a.DiffFile(a.CurText()) == nil {
		t.Skip("split is not shown")
	}
	a.Focus = core.FocusEditor
	a.StartResize()
	for _, k := range []string{"l", "h"} {
		before := a.dvGeom(a.layout()).LeftW
		press(a, k)
		if got := a.dvGeom(a.layout()).LeftW; got == before {
			t.Fatalf("%s: diff left stays %d", k, got)
		}
	}
	a.Focus = core.FocusChanges
	for _, k := range []string{"j", "k"} {
		before := a.ExplorerH()
		press(a, k)
		if got := a.ExplorerH(); got == before {
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
	a.Focus = core.FocusEditor
	a.StartResize()

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
	a.CurText().Preview = core.PreviewSplit
	l := a.layout()
	edit := a.editW(a.CurText(), l)
	press(a, "l")
	if got := a.editW(a.CurText(), a.layout()); got <= edit {
		t.Fatalf("editor %d -> %d", edit, got)
	}
	if got := a.layout().agentW; got != agent+resizeStep {
		t.Fatalf("panel moved: %d", got)
	}
}
