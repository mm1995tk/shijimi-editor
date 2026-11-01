package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/vt"
)

// addTermTab sets a terminal shell with no PTY behind it.
func addTermTab(a *App) *termTab {
	a.term.nextID++
	tab := &termTab{id: a.term.nextID, vt: vt.New(vt.Options{Cols: 20, Rows: 5})}
	a.term.sh = tab
	return tab
}

func TestTerminalOSC52Copies(t *testing.T) {
	a := newTestApp(t, "x\n")
	tab := addTermTab(a)
	tab.vt.Write([]byte("\x1b]52;c;5pel5pys6Kqe\x07"))
	a.Update(core.TermOutputMsg{ID: tab.id})
	if a.Clipboard != "日本語" {
		t.Fatalf("clipboard = %q, want 日本語", a.Clipboard)
	}
	if a.MsgErr || !strings.Contains(a.Msg, "3 文字") {
		t.Fatalf("msg = %q (err=%v), want a notice with 3 文字", a.Msg, a.MsgErr)
	}
}

func TestTerminalOSC52TooLarge(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.Clipboard = "keep"
	tab := addTermTab(a)
	tab.vt.Write([]byte("\x1b]52;c;" + strings.Repeat("A", 3<<20) + "\x07"))
	a.Update(core.TermOutputMsg{ID: tab.id})
	if a.Clipboard != "keep" || !a.MsgErr {
		t.Fatalf("clipboard = %q msg = %q (err=%v); want unchanged and an error", a.Clipboard, a.Msg, a.MsgErr)
	}
}

func TestTerminalDragSelectCopies(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.showTerm = true
	tab := addTermTab(a)
	a.View() // sizes the tab to the panel
	tab.vt.Write([]byte("hello world\r\n日本語です"))
	a.View()
	l := a.layout()
	x0, y0 := l.rightX, l.panelY+2
	a.HandleMouse(a.locate(x0+6, y0, core.Mouse{Action: core.MousePress, Button: core.MouseLeft}))
	a.HandleMouse(a.locate(x0+3, y0+1, core.Mouse{Action: core.MouseMotion, Button: core.MouseLeft}))
	a.HandleMouse(a.locate(x0+3, y0+1, core.Mouse{Action: core.MouseRelease, Button: core.MouseLeft}))
	if want := "world\n日本"; a.Clipboard != want {
		t.Fatalf("clipboard = %q, want %q", a.Clipboard, want)
	}
	if a.Focus != core.FocusTerminal || !strings.Contains(a.Msg, "8 文字") {
		t.Fatalf("focus = %v msg = %q", a.Focus, a.Msg)
	}
	if !a.term.sel.on {
		t.Fatal("selection should stay drawn after the release")
	}
	a.Update(core.Key{Code: 'a', Text: "a"})
	if a.term.sel.on {
		t.Fatal("a key press should clear the selection")
	}
}

func TestTerminalClickDoesNotCopy(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.showTerm = true
	tab := addTermTab(a)
	a.View()
	tab.vt.Write([]byte("hello"))
	a.Clipboard = "keep"
	l := a.layout()
	a.HandleMouse(a.locate(l.rightX+1, l.panelY+2, core.Mouse{Action: core.MousePress, Button: core.MouseLeft}))
	a.HandleMouse(a.locate(l.rightX+1, l.panelY+2, core.Mouse{Action: core.MouseRelease, Button: core.MouseLeft}))
	if a.Clipboard != "keep" || a.term.sel.on {
		t.Fatalf("clipboard = %q sel = %+v; a click should not copy", a.Clipboard, a.term.sel)
	}
}

func TestTerminalShiftDragSelectsInMouseProgram(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.showTerm = true
	tab := addTermTab(a)
	a.View()
	tab.vt.Write([]byte("\x1b[?1002h\x1b[?1006habc"))
	a.View()
	l := a.layout()
	y := l.panelY + 2
	a.HandleMouse(a.locate(l.rightX, y, core.Mouse{Action: core.MousePress, Button: core.MouseLeft, Shift: true}))
	a.HandleMouse(a.locate(l.rightX+2, y, core.Mouse{Action: core.MouseMotion, Button: core.MouseLeft, Shift: true}))
	a.HandleMouse(a.locate(l.rightX+2, y, core.Mouse{Action: core.MouseRelease, Button: core.MouseLeft, Shift: true}))
	if a.Clipboard != "abc" || a.term.grab {
		t.Fatalf("clipboard = %q grab = %v, want abc copied without the program", a.Clipboard, a.term.grab)
	}
}

func TestTerminalSelectionRendersReversed(t *testing.T) {
	cells := make([]vt.Cell, 4)
	for i, r := range "abcd" {
		cells[i] = vt.Cell{Rune: r, Width: 1}
	}
	plain := renderVTRowSel(cells, 4, -1, 0, 0)
	sel := renderVTRowSel(cells, 4, -1, 1, 3)
	if plain == sel || ansi.Strip(sel) != "abcd" {
		t.Fatalf("plain = %q sel = %q", plain, sel)
	}
}
