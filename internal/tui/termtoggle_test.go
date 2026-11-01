package tui

import (
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"testing"
)

func TestTerminalToggleKeys(t *testing.T) {
	ctrlBacktick := core.Key{Code: '`', Mod: core.ModCtrl}
	ctrlJ := core.Key{Code: 'j', Mod: core.ModCtrl}

	a := newTestApp(t, "hello")
	a.Focus = core.FocusEditor
	a.Mode = core.ModeNormal
	a.Update(ctrlBacktick)
	if !a.showTerm || a.Focus != core.FocusTerminal {
		t.Fatalf("ctrl+` from editor: showTerm=%v focus=%v, want terminal", a.showTerm, a.Focus)
	}

	// Shift+Enter arrives as ctrl+j from terminals that send LF; it belongs
	// to the shell.
	a.Update(ctrlJ)
	if !a.showTerm || a.Focus != core.FocusTerminal {
		t.Fatalf("ctrl+j in terminal: showTerm=%v focus=%v, want terminal kept", a.showTerm, a.Focus)
	}

	a.Update(ctrlBacktick)
	if a.showTerm || a.Focus == core.FocusTerminal {
		t.Fatalf("ctrl+` in terminal: showTerm=%v focus=%v, want closed", a.showTerm, a.Focus)
	}

	a.Update(ctrlJ)
	if !a.showTerm || a.Focus != core.FocusTerminal {
		t.Fatalf("ctrl+j from editor: showTerm=%v focus=%v, want terminal", a.showTerm, a.Focus)
	}
}
