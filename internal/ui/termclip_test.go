package ui

import (
	"strings"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/vt"
)

// addTermTab adds a terminal tab with no shell behind it.
func addTermTab(a *App) *termTab {
	a.term.nextID++
	tab := &termTab{id: a.term.nextID, vt: vt.New(vt.Options{Cols: 20, Rows: 5})}
	a.term.tabs = append(a.term.tabs, tab)
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
