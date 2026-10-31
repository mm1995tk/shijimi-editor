package ui

import (
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"

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
	a.Update(termOutputMsg{id: tab.id})
	if a.clipboard != "日本語" {
		t.Fatalf("clipboard = %q, want 日本語", a.clipboard)
	}
	if a.msgErr || !strings.Contains(a.msg, "3 文字") {
		t.Fatalf("msg = %q (err=%v), want a notice with 3 文字", a.msg, a.msgErr)
	}
}

func TestTerminalOSC52TooLarge(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.clipboard = "keep"
	tab := addTermTab(a)
	tab.vt.Write([]byte("\x1b]52;c;" + strings.Repeat("A", 3<<20) + "\x07"))
	a.Update(termOutputMsg{id: tab.id})
	if a.clipboard != "keep" || !a.msgErr {
		t.Fatalf("clipboard = %q msg = %q (err=%v); want unchanged and an error", a.clipboard, a.msg, a.msgErr)
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
	a.handleMouse(a.locate(Mouse{X: x0 + 6, Y: y0, Action: MousePress, Button: MouseLeft}))
	a.handleMouse(a.locate(Mouse{X: x0 + 3, Y: y0 + 1, Action: MouseMotion, Button: MouseLeft}))
	a.handleMouse(a.locate(Mouse{X: x0 + 3, Y: y0 + 1, Action: MouseRelease, Button: MouseLeft}))
	if want := "world\n日本"; a.clipboard != want {
		t.Fatalf("clipboard = %q, want %q", a.clipboard, want)
	}
	if a.focus != focusTerminal || !strings.Contains(a.msg, "8 文字") {
		t.Fatalf("focus = %v msg = %q", a.focus, a.msg)
	}
	if !a.term.sel.on {
		t.Fatal("selection should stay drawn after the release")
	}
	a.Update(Key{Code: 'a', Text: "a"})
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
	a.clipboard = "keep"
	l := a.layout()
	a.handleMouse(a.locate(Mouse{X: l.rightX + 1, Y: l.panelY + 2, Action: MousePress, Button: MouseLeft}))
	a.handleMouse(a.locate(Mouse{X: l.rightX + 1, Y: l.panelY + 2, Action: MouseRelease, Button: MouseLeft}))
	if a.clipboard != "keep" || a.term.sel.on {
		t.Fatalf("clipboard = %q sel = %+v; a click should not copy", a.clipboard, a.term.sel)
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
	a.handleMouse(a.locate(Mouse{X: l.rightX, Y: y, Action: MousePress, Button: MouseLeft, Shift: true}))
	a.handleMouse(a.locate(Mouse{X: l.rightX + 2, Y: y, Action: MouseMotion, Button: MouseLeft, Shift: true}))
	a.handleMouse(a.locate(Mouse{X: l.rightX + 2, Y: y, Action: MouseRelease, Button: MouseLeft, Shift: true}))
	if a.clipboard != "abc" || a.term.grab {
		t.Fatalf("clipboard = %q grab = %v, want abc copied without the program", a.clipboard, a.term.grab)
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
