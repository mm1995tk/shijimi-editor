package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/core"
)

// The shj in the primary terminal of a GUI workspace has no file tree: the GUI
// shows it.
func TestTreeHiddenUsesFullWidth(t *testing.T) {
	a := newTestApp(t, "hello\n")
	if !strings.Contains(ansi.Strip(a.render()), "EXPLORER") || a.layout().sideW == 0 {
		t.Fatal("the tree is shown by default")
	}
	a.SetTreeHidden(true)
	if l := a.layout(); l.sideW != 0 || l.rightX != 0 || l.rightW != a.w {
		t.Errorf("layout %+v", l)
	}
	if strings.Contains(ansi.Strip(a.render()), "EXPLORER") {
		t.Error("the tree is drawn")
	}
	// Nothing brings the tree back.
	a.Update(core.Key{Code: 'b', Mod: core.ModCtrl})
	a.Update(core.Key{Code: 'e', Mod: core.ModCtrl})
	a.ExecCommand("tree")
	a.SpaceMode("e")
	if a.Focus != core.FocusEditor || a.layout().sideW != 0 {
		t.Errorf("focus=%v side=%d", a.Focus, a.layout().sideW)
	}
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	a.SetTreeHidden(false)
	if a.layout().sideW == 0 {
		t.Error("the tree must come back")
	}
}

func TestTreeHiddenKeepsChangesInDiffMode(t *testing.T) {
	a, _ := gitDiffApp(t)
	a.SetTreeHidden(true)
	l := a.layout()
	if l.sideW == 0 || a.ExplorerH() != 0 {
		t.Fatalf("layout %+v explorerH %d", l, a.ExplorerH())
	}
	view := ansi.Strip(a.render())
	if strings.Contains(view, "EXPLORER") || !strings.Contains(view, "new.txt") {
		t.Errorf("view:\n%s", view)
	}
	a.SetFocus(core.FocusEditor)
	a.SpaceMode("e")
	if a.Focus != core.FocusChanges {
		t.Errorf("Space e: focus=%v", a.Focus)
	}
	// Closing diff mode takes the sidebar away and the keys to the editor.
	a.ToggleGitDiff()
	if a.Focus == core.FocusChanges || a.Focus == core.FocusExplorer || a.layout().sideW != 0 {
		t.Errorf("after diff: focus=%v side=%d", a.Focus, a.layout().sideW)
	}
}

// With no tab open and no tree, the Space leader still works.
func TestTreeHiddenNoTabSpaceLeader(t *testing.T) {
	a := mustNew(t.TempDir(), "")
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	a.SetTreeHidden(true)
	a.Update(core.Key{Code: core.KeySpace, Text: " "})
	a.Update(textKey("f"))
	if !a.Pal.Open {
		t.Error("Space f must open the file palette")
	}
}
