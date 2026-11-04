package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/core"
)

// The sizes the core reads follow the window and the layout after each message.
func TestSizesFollowMessages(t *testing.T) {
	a := mustNew(t.TempDir(), "")
	same := func(when string) {
		t.Helper()
		l := a.layout()
		if a.Core.ScreenW() != a.w || a.Core.ScreenH() != a.h || a.Core.EditRows() != l.editH ||
			a.Core.EditorCols() != l.fullW || a.Core.SuggestionsShown() != (l.agentW > 0) {
			t.Fatalf("%s: core has %+v, layout %+v (screen %dx%d)", when, a.Core.Sizes(), l, a.w, a.h)
		}
	}
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if a.Core.ScreenW() != 120 || a.Core.ScreenH() != 40 {
		t.Fatalf("screen %dx%d", a.Core.ScreenW(), a.Core.ScreenH())
	}
	same("after the window size")

	rows, cols := a.Core.EditRows(), a.Core.EditorCols()
	a.Update(core.Key{Code: 'b', Mod: core.ModCtrl}) // the sidebar
	same("after the sidebar toggled")
	if a.Core.EditorCols() == cols {
		t.Fatal("the editor area did not change width with the sidebar")
	}
	a.Update(core.Key{Code: '`', Mod: core.ModCtrl}) // the terminal panel
	same("after the terminal toggled")
	if a.Core.EditRows() >= rows {
		t.Fatalf("the terminal panel left %d rows of %d", a.Core.EditRows(), rows)
	}
}
