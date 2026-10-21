package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// agentSelApp opens the panel with a log and returns the layout.
func agentSelApp(t *testing.T, log ...logEntry) (*App, layout) {
	t.Helper()
	a := newTestApp(t, "x\n")
	a.handleKey(tea.KeyPressMsg{Code: 'i', Mod: tea.ModAlt})
	a.ag.log = log
	a.render()
	return a, a.layout()
}

func agentRowY(a *App, l layout, i int) int { return l.top + a.ag.view.y0 + (i - a.ag.view.start) }

func TestAgentDragSelectCopies(t *testing.T) {
	a, l := agentSelApp(t, logEntry{kind: logInfo, text: "hello world\n日本語です"})
	y := agentRowY(a, l, 0)
	// the log starts with one column of indent
	a.handleMouse(mouseEvent{X: l.agentX + 7, Y: y, Action: mousePress, Button: tea.MouseLeft})
	a.handleMouse(mouseEvent{X: l.agentX + 4, Y: y + 1, Action: mouseMotion, Button: tea.MouseLeft})
	a.handleMouse(mouseEvent{X: l.agentX + 4, Y: y + 1, Action: mouseRelease, Button: tea.MouseLeft})
	if want := "world\n日本"; a.clipboard != want {
		t.Fatalf("clipboard = %q, want %q", a.clipboard, want)
	}
	if !strings.Contains(a.msg, "8 文字") || !a.ag.sel.on {
		t.Fatalf("msg = %q sel = %+v", a.msg, a.ag.sel)
	}
	drawn := strings.Join(a.renderAgent(l.agentW, l.bodyH, true), "\n")
	a.ag.sel.on = false
	plain := strings.Join(a.renderAgent(l.agentW, l.bodyH, true), "\n")
	a.ag.sel.on = true
	if drawn == plain || ansi.Strip(drawn) != ansi.Strip(plain) {
		t.Fatal("selection should change only the style of the text")
	}
	a.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	if a.ag.sel.on {
		t.Fatal("a key press should clear the selection")
	}
}

func TestAgentSelectionJoinsWrappedRows(t *testing.T) {
	long := strings.Repeat("word ", 20) + "https://example.com/" + strings.Repeat("x", 80)
	a, l := agentSelApp(t,
		logEntry{kind: logAgent, text: long},
		logEntry{kind: logInfo, text: long})
	n := len(a.ag.view.rows)
	if n < 6 {
		t.Fatalf("only %d rows", n)
	}
	a.ag.scroll = 0
	a.render()
	top := a.ag.view.start
	w := a.ag.view.w
	// from the first row to the last, all of it
	a.ag.sel = agentSel{on: true, a: agentPos{top, 0}, b: agentPos{top + n - 1, w - 1}}
	got := a.ag.sel.text(a.ag.view.rows, w)
	if want := long + "\n" + long; got != want {
		t.Fatalf("copied\n%q\nwant\n%q", got, want)
	}
	_ = l
}

func TestAgentSelectionOutsidePanel(t *testing.T) {
	a, l := agentSelApp(t, logEntry{kind: logInfo, text: "abc\ndef"})
	y := agentRowY(a, l, 0)
	a.handleMouse(mouseEvent{X: l.agentX + 1, Y: y, Action: mousePress, Button: tea.MouseLeft})
	// dragged far left of the panel and below it
	a.handleMouse(mouseEvent{X: 0, Y: y + 1, Action: mouseMotion, Button: tea.MouseLeft})
	a.handleMouse(mouseEvent{X: 0, Y: a.h - 1, Action: mouseRelease, Button: tea.MouseLeft})
	if a.ag.sel.dragging || a.clipboard == "" || strings.Contains(a.clipboard, "x") {
		t.Fatalf("sel = %+v clipboard = %q", a.ag.sel, a.clipboard)
	}
}

func TestAgentClickDoesNotCopy(t *testing.T) {
	a, l := agentSelApp(t, logEntry{kind: logInfo, text: "hello"})
	a.focus = focusEditor
	a.clipboard = "keep"
	y := agentRowY(a, l, 0)
	a.handleMouse(mouseEvent{X: l.agentX + 2, Y: y, Action: mousePress, Button: tea.MouseLeft})
	a.handleMouse(mouseEvent{X: l.agentX + 2, Y: y, Action: mouseRelease, Button: tea.MouseLeft})
	if a.clipboard != "keep" || a.ag.sel.on || a.focus != focusAgent {
		t.Fatalf("clipboard = %q sel = %+v focus = %v", a.clipboard, a.ag.sel, a.focus)
	}
}

func TestAgentSelectionStaysOnTextWhenScrolled(t *testing.T) {
	var log []logEntry
	for i := 0; i < 60; i++ {
		log = append(log, logEntry{kind: logInfo, text: strings.Repeat("a", i%9+1)})
	}
	a, l := agentSelApp(t, log...)
	y := agentRowY(a, l, a.ag.view.start+2)
	a.handleMouse(mouseEvent{X: l.agentX + 1, Y: y, Action: mousePress, Button: tea.MouseLeft})
	a.handleMouse(mouseEvent{X: l.agentX + 3, Y: y, Action: mouseRelease, Button: tea.MouseLeft})
	before := ansi.Strip(a.ag.view.rows[a.ag.sel.a.row].s)
	a.handleMouse(mouseEvent{X: l.agentX + 3, Y: y, Action: mousePress, Button: tea.MouseWheelUp})
	a.render()
	if !a.ag.sel.on || ansi.Strip(a.ag.view.rows[a.ag.sel.a.row].s) != before {
		t.Fatalf("selection moved: %+v", a.ag.sel)
	}
}
