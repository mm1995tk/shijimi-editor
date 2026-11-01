package tui

import (
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/core"
)

func TestCtrlSpaceInEditor(t *testing.T) {
	ctrlSpace := core.Key{Code: KeySpace, Mod: core.ModCtrl}

	t.Run("vscode keeps editor focus", func(t *testing.T) {
		a := newTestApp(t, "hello")
		a.Helix = false
		a.Focus = core.FocusEditor
		a.Update(ctrlSpace)
		if a.showTerm || a.Focus != core.FocusEditor {
			t.Fatalf("showTerm=%v focus=%v, want editor focus without terminal", a.showTerm, a.Focus)
		}
	})

	t.Run("helix insert keeps editor focus", func(t *testing.T) {
		a := newTestApp(t, "hello")
		a.Focus = core.FocusEditor
		a.Mode = core.ModeInsert
		a.Update(ctrlSpace)
		if a.showTerm || a.Focus != core.FocusEditor {
			t.Fatalf("showTerm=%v focus=%v, want editor focus without terminal", a.showTerm, a.Focus)
		}
	})

	t.Run("helix normal toggles terminal", func(t *testing.T) {
		a := newTestApp(t, "hello")
		a.Focus = core.FocusEditor
		a.Mode = core.ModeNormal
		a.Update(ctrlSpace)
		if !a.showTerm || a.Focus != core.FocusTerminal {
			t.Fatalf("showTerm=%v focus=%v, want terminal", a.showTerm, a.Focus)
		}
	})
}

func TestCtrlSpaceWithoutTextTab(t *testing.T) {
	a := newTestApp(t, "hello")
	a.Helix = false
	a.Tabs = nil
	a.Focus = core.FocusEditor
	a.Update(core.Key{Code: KeySpace, Mod: core.ModCtrl})
	if !a.showTerm || a.Focus != core.FocusTerminal {
		t.Fatalf("showTerm=%v focus=%v, want terminal", a.showTerm, a.Focus)
	}
}
