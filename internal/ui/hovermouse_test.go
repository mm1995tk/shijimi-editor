package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func move(a *App, x, y int) tea.Cmd {
	return a.handleMouse(tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionMotion, Button: tea.MouseButtonNone})
}

func TestMouseHoverDwellKeepsCursor(t *testing.T) {
	a := newTestApp(t, "hello world\nsecond\n")
	l := a.layout()
	b := a.curText().buf
	x := l.rightX + gutterWidth(b) + 7 // on "o" of world
	if cmd := move(a, x, l.editY); cmd == nil {
		t.Fatal("moving onto text should start the dwell timer")
	}
	if b.Cur.Row != 0 || b.Cur.Col != 0 {
		t.Fatalf("cursor moved to %v", b.Cur)
	}
	pos, ok := a.mouseBufPos(l, x, l.editY)
	if !ok || pos.Row != 0 || pos.Col != 7 {
		t.Fatalf("pos %v ok=%v", pos, ok)
	}
	if cmd := move(a, x, l.editY); cmd != nil {
		t.Fatal("staying in the same cell must not restart the timer")
	}
	// Past the end of a line is not a word.
	if _, ok := a.mouseBufPos(l, x+20, l.editY); ok {
		t.Fatal("empty space after the line should not hover")
	}
}

func TestMouseHoverStaleDwellIgnored(t *testing.T) {
	a := newTestApp(t, "hello world\n")
	l := a.layout()
	gw := gutterWidth(a.curText().buf)
	move(a, l.rightX+gw+1, l.editY)
	seq := a.hoverSeq
	move(a, l.rightX+gw+3, l.editY)
	if cmd := a.hoverDwell(hoverDwellMsg{seq}); cmd != nil {
		t.Fatal("a timer from an earlier cell must be dropped")
	}
}

func TestMouseHoverPopupStaysOnPointerAndCloses(t *testing.T) {
	a := newTestApp(t, "hello world\nsecond\n")
	l := a.layout()
	gw := gutterWidth(a.curText().buf)
	x, y := l.rightX+gw+2, l.editY
	move(a, x, y)
	a.hoverText, a.hoverScroll = "text", 0
	a.hoverAt = &screenPt{x, y}
	a.hoverPos.Col = 2
	move(a, x, y+1) // onto the popup below the word
	if a.hoverText == "" {
		t.Fatal("popup closed while the pointer is on it")
	}
	move(a, l.rightX+gw+30, y+10) // away
	if a.hoverText != "" || a.hoverAt != nil {
		t.Fatal("popup should close when the pointer leaves")
	}
}

func TestMouseHoverReplyDroppedWhenStale(t *testing.T) {
	a := newTestApp(t, "hello world\n")
	tb := a.cur()
	m := &mouseHover{tab: tb, seq: a.hoverSeq, pt: screenPt{5, 1}}
	a.hoverSeq++
	a.Update(hoverMsg{text: "x", mouse: m})
	if a.hoverText != "" {
		t.Fatal("reply for an old pointer position was shown")
	}
	m.seq = a.hoverSeq
	a.Update(hoverMsg{text: "x", mouse: m})
	if a.hoverText != "x" || a.hoverAt == nil {
		t.Fatal("current reply should show at the pointer")
	}
}

func TestMouseHoverDisabled(t *testing.T) {
	a := newTestApp(t, "hello world\n")
	a.cfg.Editor.MouseHover = false
	l := a.layout()
	if cmd := move(a, l.rightX+gutterWidth(a.curText().buf)+1, l.editY); cmd != nil {
		t.Fatal("disabled: no timer")
	}
}

func wheelDown(a *App, x, y int) {
	a.handleMouse(tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown})
}

func TestMouseHoverWheelOutsidePopupScrollsBuffer(t *testing.T) {
	lines := ""
	for range 60 {
		lines += "hello world\n"
	}
	a := newTestApp(t, lines)
	l := a.layout()
	b := a.curText().buf
	x, y := l.rightX+gutterWidth(b)+2, l.editY
	a.hoverText, a.hoverAt = "text", &screenPt{x, y}
	a.hoverPos.Col = 2
	wheelDown(a, x, y) // on the word, outside the popup below it
	if a.hoverText != "" || a.hoverAt != nil {
		t.Fatal("popup should close when the wheel turns outside it")
	}
	if b.ScrollRow == 0 {
		t.Fatal("the wheel outside the popup should scroll the buffer")
	}
}

func TestMouseHoverWheelOnPopupScrollsPopup(t *testing.T) {
	a := newTestApp(t, "hello world\nsecond\n")
	l := a.layout()
	b := a.curText().buf
	x, y := l.rightX+gutterWidth(b)+2, l.editY
	long := ""
	for i := range 40 {
		long += "line " + string(rune('a'+i%26)) + "\n\n"
	}
	a.hoverText, a.hoverAt = long, &screenPt{x, y}
	wheelDown(a, x, y+1) // on the popup
	if a.hoverText == "" || a.hoverScroll == 0 {
		t.Fatalf("wheel on the popup should scroll it: text=%v scroll=%d", a.hoverText != "", a.hoverScroll)
	}
	if b.ScrollRow != 0 {
		t.Fatal("buffer must not scroll while the popup takes the wheel")
	}
}
