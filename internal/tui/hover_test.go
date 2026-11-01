package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

func TestHoverBoxShapeAndScroll(t *testing.T) {
	v := NewHoverView(goplsHover, 34, 6)
	if v.MaxScroll() == 0 {
		t.Fatal("expected scrollable content")
	}
	for _, s := range []int{0, 2, 99} {
		box := v.render(s)
		if len(box) != v.height+2 {
			t.Fatalf("scroll %d: %d rows", s, len(box))
		}
		w := ansi.StringWidth(box[0])
		for _, row := range box {
			if ansi.StringWidth(row) != w {
				t.Fatalf("scroll %d: ragged box", s)
			}
		}
	}
	if !strings.Contains(ansi.Strip(v.render(0)[v.height+1]), "↓") {
		t.Fatal("missing scroll indicator")
	}
}

func TestHoverBorderShowsCopyButtonAndHint(t *testing.T) {
	v := NewHoverView(goplsHover, 60, 5)
	box := v.render(0)
	if top := ansi.Strip(box[0]); !strings.Contains(top, hoverCopyLabel()+"─╮") {
		t.Fatalf("top border: %q", top)
	}
	bottom := ansi.Strip(box[len(box)-1])
	if !strings.Contains(bottom, "y コピー") || !strings.Contains(bottom, "↓") {
		t.Fatalf("bottom border: %q", bottom)
	}
	short := NewHoverView("short text that fits", 60, 5).render(0)
	if b := ansi.Strip(short[len(short)-1]); !strings.Contains(b, "y コピー") || strings.Contains(b, "スクロール") {
		t.Fatalf("unscrollable bottom border: %q", b)
	}
}

func TestHoverCopyKeyAndButton(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.hoverText = goplsHover
	keys(a, "y")
	if a.clipboard != goplsHover || a.hoverText == "" || a.msg == "" || a.msgErr {
		t.Fatalf("key: clipboard=%q hover=%v msg=%q", a.clipboard, a.hoverText != "", a.msg)
	}

	a.clipboard = ""
	l := a.layout()
	bx, by, ok := a.hoverPos(l)
	if !ok {
		t.Fatal("no hover position")
	}
	v := a.hoverView(l)
	n := ansi.StringWidth(hoverCopyLabel())
	col := v.width + 2 - n/2
	if !v.copyHit(col, 0) {
		t.Fatalf("copyHit(%d,0) = false", col)
	}
	a.HandleMouse(a.locate(bx+col, by, Mouse{Action: MousePress, Button: MouseLeft}))
	if a.clipboard != goplsHover || a.hoverText == "" {
		t.Fatalf("button: clipboard=%q hover=%v", a.clipboard, a.hoverText != "")
	}
	a.HandleMouse(a.locate(bx+col, by, Mouse{Action: MouseRelease, Button: MouseLeft}))
	if a.hoverText == "" {
		t.Fatal("release on the button closed the hover")
	}

	a.clipboard = ""
	a.HandleMouse(a.locate(bx+2, by+1, Mouse{Action: MousePress, Button: MouseLeft}))
	if a.clipboard != "" || a.hoverText != "" {
		t.Fatalf("click elsewhere: clipboard=%q hover=%v", a.clipboard, a.hoverText != "")
	}
}

func TestHoverOverlay(t *testing.T) {
	a := newTestApp(t, "hello world\nsecond\n")
	l := a.layout()
	long := "```go\nfunc f()\n```\n---\n"
	for i := range 30 {
		long += fmt.Sprintf("line %d\n", i)
	}
	a.hoverText = long
	a.hoverAt = &buffer.Pos{Row: 1, Col: 3}
	ov := a.hoverOverlay(l)
	if ov.Kind != screen.OverlayHover || ov.Pane != a.CurText().Path() || ov.At == nil || *ov.At != (screen.Pos{Line: 1, Col: 3}) {
		t.Fatalf("overlay = %+v", ov)
	}
	if ov.View == nil || ov.View.Total <= len(ov.Lines) || len(ov.Lines) != ov.View.Rows || ov.Hint != hoverHint() {
		t.Fatalf("view = %+v, %d lines, hint %q", ov.View, len(ov.Lines), ov.Hint)
	}
	if ov.Lines[0].Band != screen.StyleHoverCodeBg || !ov.Lines[1].Rule || ov.Lines[2].Band != "" {
		t.Fatalf("lines = %+v", ov.Lines[:3])
	}
	a.hoverAt = nil
	if at := a.hoverOverlay(l).At; at == nil || *at != ToScreenPos(a.CurText().Buf.Cur) {
		t.Fatalf("a keyboard hover hangs from the cursor, got %v", at)
	}
}

// A pointer hover keeps the place it opened at; the file may shrink (a reload)
// while it is open, and drawing must not fall off the end.
func TestHoverAnchorSurvivesShrinkingFile(t *testing.T) {
	a := newTestApp(t, "a\nb\nc\nd\nlast line\n")
	a.hoverText, a.hoverAt = "text", &buffer.Pos{Row: 4, Col: 6}
	b := a.CurText().Buf
	b.Lines, b.Cur = b.Lines[:1], buffer.Pos{}
	_ = a.render()
	if at := a.hoverOverlay(a.layout()).At; at == nil || at.Line != 0 {
		t.Fatalf("anchor = %v", at)
	}
}
