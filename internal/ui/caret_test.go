package ui

import "testing"

func TestCaretFollowsTyping(t *testing.T) {
	a := newTestApp(t, "日本語\n")
	l := a.layout()

	keys(a, "l", "l")
	want, wantY := a.cursorScreen(a.curText(), l)
	if x, y, ok := a.caretPos(l); !ok || x != want || y != wantY {
		t.Errorf("editor caret = (%d,%d,%v), want (%d,%d)", x, y, ok, want, wantY)
	}

	keys(a, ":", "wq")
	if x, y, ok := a.caretPos(l); !ok || x != 3 || y != a.h-1 {
		t.Errorf("cmdline caret = (%d,%d,%v), want (3,%d)", x, y, ok, a.h-1)
	}
	keys(a, "esc")

	a.focus = focusExplorer
	if _, _, ok := a.caretPos(l); ok {
		t.Error("explorer focused: want no caret")
	}
}
