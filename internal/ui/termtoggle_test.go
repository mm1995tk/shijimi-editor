package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestTerminalToggleKeys(t *testing.T) {
	ctrlBacktick := tea.KeyPressMsg{Code: '`', Mod: tea.ModCtrl}
	ctrlJ := tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl}

	a := newTestApp(t, "hello")
	a.focus = focusEditor
	a.mode = modeNormal
	a.Update(ctrlBacktick)
	if !a.showTerm || a.focus != focusTerminal {
		t.Fatalf("ctrl+` from editor: showTerm=%v focus=%v, want terminal", a.showTerm, a.focus)
	}

	// Shift+Enter arrives as ctrl+j from terminals that send LF; it belongs
	// to the shell.
	a.Update(ctrlJ)
	if !a.showTerm || a.focus != focusTerminal {
		t.Fatalf("ctrl+j in terminal: showTerm=%v focus=%v, want terminal kept", a.showTerm, a.focus)
	}

	a.Update(ctrlBacktick)
	if a.showTerm || a.focus == focusTerminal {
		t.Fatalf("ctrl+` in terminal: showTerm=%v focus=%v, want closed", a.showTerm, a.focus)
	}

	a.Update(ctrlJ)
	if !a.showTerm || a.focus != focusTerminal {
		t.Fatalf("ctrl+j from editor: showTerm=%v focus=%v, want terminal", a.showTerm, a.focus)
	}
}
