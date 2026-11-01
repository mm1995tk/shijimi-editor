package tui

import (
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
)

func move(a *App, x, y int) tea.Cmd {
	return a.effectCmd(a.HandleMouse(a.locate(x, y, core.Mouse{Action: core.MouseMotion, Button: core.MouseNone}))...)
}

func TestMouseHoverDwellKeepsCursor(t *testing.T) {
	a := newTestApp(t, "hello world\nsecond\n")
	l := a.layout()
	b := a.CurText().Buf
	x := l.rightX + core.GutterWidth(b) + 7 // on "o" of world
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
	gw := core.GutterWidth(a.CurText().Buf)
	move(a, l.rightX+gw+1, l.editY)
	seq := a.HoverSeq
	move(a, l.rightX+gw+3, l.editY)
	if cmd := a.HoverDwell(core.HoverDwell{seq}); cmd != nil {
		t.Fatal("a timer from an earlier cell must be dropped")
	}
}

func TestMouseHoverPopupStaysOnPointerAndCloses(t *testing.T) {
	a := newTestApp(t, "hello world\nsecond\n")
	l := a.layout()
	gw := core.GutterWidth(a.CurText().Buf)
	x, y := l.rightX+gw+2, l.editY
	move(a, x, y)
	a.HoverText, a.HoverScroll = "text", 0
	a.HoverAt = &buffer.Pos{Row: y - l.editY, Col: x - l.rightX - core.GutterWidth(a.CurText().Buf)}
	a.HoverWord = core.WordSpan{0, 0, 5}
	move(a, x, y+1) // onto the popup below the word
	if a.HoverText == "" {
		t.Fatal("popup closed while the pointer is on it")
	}
	move(a, l.rightX+gw+30, y+10) // away
	if a.HoverText != "" || a.HoverAt != nil {
		t.Fatal("popup should close when the pointer leaves")
	}
}

func TestMouseHoverReplyDroppedWhenStale(t *testing.T) {
	a := newTestApp(t, "hello world\n")
	tb := a.Cur()
	m := &core.MouseHover{Tab: tb, Seq: a.HoverSeq}
	a.HoverSeq++
	a.Update(core.HoverMsg{Text: "x", Mouse: m})
	if a.HoverText != "" {
		t.Fatal("reply for an old pointer position was shown")
	}
	m.Seq = a.HoverSeq
	a.Update(core.HoverMsg{Text: "x", Mouse: m})
	if a.HoverText != "x" || a.HoverAt == nil {
		t.Fatal("current reply should show at the pointer")
	}
}

func TestMouseHoverDisabled(t *testing.T) {
	a := newTestApp(t, "hello world\n")
	a.Cfg.Editor.MouseHover = false
	l := a.layout()
	if cmd := move(a, l.rightX+core.GutterWidth(a.CurText().Buf)+1, l.editY); cmd != nil {
		t.Fatal("disabled: no timer")
	}
}

func wheelDown(a *App, x, y int) {
	a.HandleMouse(a.locate(x, y, core.Mouse{Action: core.MousePress, Button: core.MouseWheelDown}))
}

func TestMouseHoverWheelOutsidePopupScrollsBuffer(t *testing.T) {
	lines := ""
	for range 60 {
		lines += "hello world\n"
	}
	a := newTestApp(t, lines)
	l := a.layout()
	b := a.CurText().Buf
	x, y := l.rightX+core.GutterWidth(b)+2, l.editY
	a.HoverText, a.HoverAt = "text", &buffer.Pos{Row: y - l.editY, Col: x - l.rightX - core.GutterWidth(a.CurText().Buf)}
	a.HoverWord = core.WordSpan{0, 0, 5}
	wheelDown(a, x, y) // on the word, outside the popup below it
	if a.HoverText != "" || a.HoverAt != nil {
		t.Fatal("popup should close when the wheel turns outside it")
	}
	if b.ScrollRow == 0 {
		t.Fatal("the wheel outside the popup should scroll the buffer")
	}
}

func TestMouseHoverWheelOnPopupScrollsPopup(t *testing.T) {
	a := newTestApp(t, "hello world\nsecond\n")
	l := a.layout()
	b := a.CurText().Buf
	x, y := l.rightX+core.GutterWidth(b)+2, l.editY
	long := ""
	for i := range 40 {
		long += "line " + string(rune('a'+i%26)) + "\n\n"
	}
	a.HoverText, a.HoverAt = long, &buffer.Pos{Row: y - l.editY, Col: x - l.rightX - core.GutterWidth(a.CurText().Buf)}
	wheelDown(a, x, y+1) // on the popup
	if a.HoverText == "" || a.HoverScroll == 0 {
		t.Fatalf("wheel on the popup should scroll it: text=%v scroll=%d", a.HoverText != "", a.HoverScroll)
	}
	if b.ScrollRow != 0 {
		t.Fatal("buffer must not scroll while the popup takes the wheel")
	}
}

func TestMouseHoverStaysAcrossWordAndClosesOutside(t *testing.T) {
	a := newTestApp(t, "hello world\n")
	l := a.layout()
	gw := core.GutterWidth(a.CurText().Buf)
	x, y := l.rightX+gw+1, l.editY
	a.Focus = core.FocusEditor
	cmd := move(a, x, y)
	if cmd == nil {
		t.Fatal("entering a word should start the timer")
	}
	seq := a.HoverSeq
	if cmd := move(a, x+2, y); cmd != nil || a.HoverSeq != seq {
		t.Fatal("moving inside the same word must not restart the timer")
	}
	a.HoverText, a.HoverAt = "text", &buffer.Pos{Row: y - l.editY, Col: x + 2 - l.rightX - core.GutterWidth(a.CurText().Buf)}
	move(a, x+3, y) // "hello" is columns 0-4; still inside
	if a.HoverText == "" {
		t.Fatal("popup closed while moving within the word")
	}
	move(a, l.rightX+gw+7, y) // "world"
	if a.HoverText != "" {
		t.Fatal("popup should close on another word")
	}
}

func TestMouseHoverNeedsEditorFocus(t *testing.T) {
	for _, f := range []core.Focus{core.FocusExplorer, core.FocusTerminal} {
		a := newTestApp(t, "hello world\n")
		a.Focus = f
		l := a.layout()
		if cmd := move(a, l.rightX+core.GutterWidth(a.CurText().Buf)+1, l.editY); cmd != nil {
			t.Fatalf("focus %v: timer started", f)
		}
		a.HoverCell = core.ScreenPt{l.rightX + core.GutterWidth(a.CurText().Buf) + 1, l.editY}
		if cmd := a.HoverDwell(core.HoverDwell{a.HoverSeq}); cmd != nil {
			t.Fatalf("focus %v: dwell sent a request", f)
		}
	}
}

func TestMouseHoverPopupDoesNotTakeKeys(t *testing.T) {
	a := newTestApp(t, "hello world\n")
	a.Focus = core.FocusEditor
	a.HoverText, a.HoverAt = "text", &buffer.Pos{Col: 2}
	a.HoverScroll = 0
	a.HandleKey(textKey(string('y')))
	if a.HoverText != "" || a.HoverAt != nil {
		t.Fatal("a pointer popup should close on any key")
	}
	if a.Clipboard == "text" {
		t.Fatal("y must not copy the pointer popup")
	}
	// A popup opened from the keyboard keeps its key handling.
	a.HoverText, a.HoverAt = "text", nil
	a.HandleKey(textKey(string('y')))
	if a.HoverText == "" {
		t.Fatal("keyboard popup should still handle y (copy), staying open")
	}
}

func TestMouseHoverKeptAfterWheelOnPopup(t *testing.T) {
	a := newTestApp(t, "hello world\nsecond line\n")
	a.Focus = core.FocusEditor
	l := a.layout()
	gw := core.GutterWidth(a.CurText().Buf)
	x, y := l.rightX+gw+1, l.editY
	long := ""
	for range 40 {
		long += "line\n\n"
	}
	a.HoverText, a.HoverAt = long, &buffer.Pos{Row: y - l.editY, Col: x - l.rightX - core.GutterWidth(a.CurText().Buf)}
	a.HoverWord = core.WordSpan{0, 0, 5}
	a.HoverCell = core.ScreenPt{x, y}
	wheelDown(a, x, y+1) // on the popup
	if a.HoverText == "" || a.HoverWord != (core.WordSpan{0, 0, 5}) {
		t.Fatal("wheel on the popup must keep the popup and its word")
	}
	if cmd := move(a, x+2, y); cmd != nil || a.HoverText == "" {
		t.Fatal("back on the word after scrolling: the popup must stay and no timer start")
	}
}

func TestMouseHoverOverKeyboardPopupAsksNothing(t *testing.T) {
	a := newTestApp(t, "hello world\nsecond line\nthird\n")
	a.Focus = core.FocusEditor
	l := a.layout()
	a.HoverText, a.HoverAt = "text", nil // opened with Space k
	x, y, ok := a.hoverPos(l)
	if !ok {
		t.Fatal("no popup rect")
	}
	if cmd := move(a, x+2, y+1); cmd != nil {
		t.Fatal("the pointer on a keyboard popup must not start a hover for the text below")
	}
	if a.HoverWord != (core.WordSpan{}) || a.HoverText != "text" {
		t.Fatal("popup replaced or word armed")
	}
}

func TestMouseHoverBlockedByKeyOwners(t *testing.T) {
	for name, set := range map[string]func(*App){
		"cmdline": func(a *App) { a.CmdOpen = true },
		"resize":  func(a *App) { a.resizeMode = true },
		"palette": func(a *App) { a.Pal.Open = true },
	} {
		a := newTestApp(t, "hello world\n")
		a.Focus = core.FocusEditor
		set(a)
		l := a.layout()
		x, y := l.rightX+core.GutterWidth(a.CurText().Buf)+1, l.editY
		if cmd := move(a, x, y); cmd != nil {
			t.Fatalf("%s: timer started", name)
		}
		a.HoverCell = core.ScreenPt{x, y}
		if cmd := a.HoverDwell(core.HoverDwell{a.HoverSeq}); cmd != nil {
			t.Fatalf("%s: dwell sent a request", name)
		}
	}
}

func TestMouseHoverClosedByKeyInCommandLine(t *testing.T) {
	a := newTestApp(t, "hello world\n")
	a.Focus = core.FocusEditor
	a.CmdOpen = true
	a.HoverText, a.HoverAt = "text", &buffer.Pos{Col: 2}
	a.HandleKey(textKey(string('w')))
	if a.HoverText != "" || a.HoverAt != nil {
		t.Fatal("the pointer popup should close on a key typed in the command line")
	}
	if !a.CmdOpen {
		t.Fatal("the command line must stay open")
	}
}
