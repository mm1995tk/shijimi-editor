package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

func TestHoverBoxShapeAndScroll(t *testing.T) {
	v := core.NewHoverView(goplsHover, 34, 6)
	if v.MaxScroll() == 0 {
		t.Fatal("expected scrollable content")
	}
	for _, s := range []int{0, 2, 99} {
		box := drawHover(v.Overlay(s))
		if len(box) != v.Height+2 {
			t.Fatalf("scroll %d: %d rows", s, len(box))
		}
		w := ansi.StringWidth(box[0])
		for _, row := range box {
			if ansi.StringWidth(row) != w {
				t.Fatalf("scroll %d: ragged box", s)
			}
		}
	}
	if !strings.Contains(ansi.Strip(drawHover(v.Overlay(0))[v.Height+1]), "↓") {
		t.Fatal("missing scroll indicator")
	}
}

func TestHoverBorderShowsCopyButtonAndHint(t *testing.T) {
	v := core.NewHoverView(goplsHover, 60, 5)
	box := drawHover(v.Overlay(0))
	if top := ansi.Strip(box[0]); !strings.Contains(top, core.HoverCopyLabel()+"─╮") {
		t.Fatalf("top border: %q", top)
	}
	bottom := ansi.Strip(box[len(box)-1])
	if !strings.Contains(bottom, "y コピー") || !strings.Contains(bottom, "↓") {
		t.Fatalf("bottom border: %q", bottom)
	}
	short := drawHover(core.NewHoverView("short text that fits", 60, 5).Overlay(0))
	if b := ansi.Strip(short[len(short)-1]); !strings.Contains(b, "y コピー") || strings.Contains(b, "スクロール") {
		t.Fatalf("unscrollable bottom border: %q", b)
	}
}

func TestHoverCopyKeyAndButton(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.HoverText = goplsHover
	keys(a, "y")
	if a.Clipboard != goplsHover || a.HoverText == "" || a.Msg == "" || a.MsgErr {
		t.Fatalf("key: clipboard=%q hover=%v msg=%q", a.Clipboard, a.HoverText != "", a.Msg)
	}

	a.Clipboard = ""
	l := a.layout()
	bx, by, ok := a.hoverPos(l)
	if !ok {
		t.Fatal("no hover position")
	}
	v := a.hoverView(l)
	n := ansi.StringWidth(core.HoverCopyLabel())
	col := v.Width + 2 - n/2
	if !v.CopyHit(col, 0) {
		t.Fatalf("copyHit(%d,0) = false", col)
	}
	a.HandleMouse(a.locate(bx+col, by, core.Mouse{Action: core.MousePress, Button: core.MouseLeft}))
	if a.Clipboard != goplsHover || a.HoverText == "" {
		t.Fatalf("button: clipboard=%q hover=%v", a.Clipboard, a.HoverText != "")
	}
	a.HandleMouse(a.locate(bx+col, by, core.Mouse{Action: core.MouseRelease, Button: core.MouseLeft}))
	if a.HoverText == "" {
		t.Fatal("release on the button closed the hover")
	}

	a.Clipboard = ""
	a.HandleMouse(a.locate(bx+2, by+1, core.Mouse{Action: core.MousePress, Button: core.MouseLeft}))
	if a.Clipboard != "" || a.HoverText != "" {
		t.Fatalf("click elsewhere: clipboard=%q hover=%v", a.Clipboard, a.HoverText != "")
	}
}

func TestHoverOverlay(t *testing.T) {
	a := newTestApp(t, "hello world\nsecond\n")
	l := a.layout()
	long := "```go\nfunc f()\n```\n---\n"
	for i := range 30 {
		long += fmt.Sprintf("line %d\n", i)
	}
	a.HoverText = long
	a.HoverAt = &buffer.Pos{Row: 1, Col: 3}
	ov := a.hoverOverlay(l)
	if ov.Kind != screen.OverlayHover || ov.Pane != a.CurText().Path() || ov.At == nil || *ov.At != (screen.Pos{Line: 1, Col: 3}) {
		t.Fatalf("overlay = %+v", ov)
	}
	if ov.View == nil || ov.View.Total <= len(ov.Lines) || len(ov.Lines) != ov.View.Rows || ov.Hint != core.HoverHint() {
		t.Fatalf("view = %+v, %d lines, hint %q", ov.View, len(ov.Lines), ov.Hint)
	}
	if ov.Lines[0].Band != screen.StyleHoverCodeBg || !ov.Lines[1].Rule || ov.Lines[2].Band != "" {
		t.Fatalf("lines = %+v", ov.Lines[:3])
	}
	a.HoverAt = nil
	if at := a.hoverOverlay(l).At; at == nil || *at != core.ToScreenPos(a.CurText().Buf.Cur) {
		t.Fatalf("a keyboard hover hangs from the cursor, got %v", at)
	}
}

// A pointer hover keeps the place it opened at; the file may shrink (a reload)
// while it is open, and drawing must not fall off the end.
func TestHoverAnchorSurvivesShrinkingFile(t *testing.T) {
	a := newTestApp(t, "a\nb\nc\nd\nlast line\n")
	a.HoverText, a.HoverAt = "text", &buffer.Pos{Row: 4, Col: 6}
	b := a.CurText().Buf
	b.Lines, b.Cur = b.Lines[:1], buffer.Pos{}
	_ = a.render()
	if at := a.hoverOverlay(a.layout()).At; at == nil || at.Line != 0 {
		t.Fatalf("anchor = %v", at)
	}
}
