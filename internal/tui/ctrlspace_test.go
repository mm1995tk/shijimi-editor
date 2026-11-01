package tui

import (
	"testing"
)

func TestCtrlSpaceInEditor(t *testing.T) {
	ctrlSpace := Key{Code: KeySpace, Mod: ModCtrl}

	t.Run("vscode keeps editor focus", func(t *testing.T) {
		a := newTestApp(t, "hello")
		a.helix = false
		a.focus = FocusEditor
		a.Update(ctrlSpace)
		if a.showTerm || a.focus != FocusEditor {
			t.Fatalf("showTerm=%v focus=%v, want editor focus without terminal", a.showTerm, a.focus)
		}
	})

	t.Run("helix insert keeps editor focus", func(t *testing.T) {
		a := newTestApp(t, "hello")
		a.focus = FocusEditor
		a.mode = ModeInsert
		a.Update(ctrlSpace)
		if a.showTerm || a.focus != FocusEditor {
			t.Fatalf("showTerm=%v focus=%v, want editor focus without terminal", a.showTerm, a.focus)
		}
	})

	t.Run("helix normal toggles terminal", func(t *testing.T) {
		a := newTestApp(t, "hello")
		a.focus = FocusEditor
		a.mode = ModeNormal
		a.Update(ctrlSpace)
		if !a.showTerm || a.focus != FocusTerminal {
			t.Fatalf("showTerm=%v focus=%v, want terminal", a.showTerm, a.focus)
		}
	})
}

func TestCtrlSpaceWithoutTextTab(t *testing.T) {
	a := newTestApp(t, "hello")
	a.helix = false
	a.tabs = nil
	a.focus = FocusEditor
	a.Update(Key{Code: KeySpace, Mod: ModCtrl})
	if !a.showTerm || a.focus != FocusTerminal {
		t.Fatalf("showTerm=%v focus=%v, want terminal", a.showTerm, a.focus)
	}
}
