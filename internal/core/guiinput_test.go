package core

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// A W3C key name becomes the Key the TUI gets for the same press.
func TestKeyFromW3C(t *testing.T) {
	for _, c := range []struct {
		key, text string
		mod       Mod
		want      string // Key.String()
		code      rune
		ok        bool
	}{
		{"a", "a", 0, "a", 'a', true},
		{"A", "A", ModShift, "A", 'a', true},
		{"a", "", ModCtrl, "ctrl+a", 'a', true},
		{"a", "a", ModCtrl, "ctrl+a", 'a', true}, // Ctrl types nothing
		{"A", "", ModCtrl | ModShift, "ctrl+shift+a", 'a', true},
		{" ", " ", 0, "space", KeySpace, true},
		{"Enter", "\r", 0, "enter", KeyEnter, true},
		{"Escape", "", 0, "esc", KeyEscape, true},
		{"Tab", "", ModShift, "shift+tab", KeyTab, true},
		{"ArrowLeft", "", ModAlt, "alt+left", KeyLeft, true},
		{"F5", "", 0, "f5", KeyF5, true},
		{"?", "?", ModShift, "?", '?', true},
		{"Process", "日本", 0, "日本", KeyExtended, true}, // an input method commits
		{"Unidentified", "あ", 0, "あ", 'あ', true},
		{"Shift", "", ModShift, "", 0, false},
		{"Dead", "", 0, "", 0, false},
	} {
		k, ok := KeyFromW3C(c.key, c.text, c.mod)
		if ok != c.ok {
			t.Errorf("%q %v: ok = %v", c.key, c.mod, ok)
			continue
		}
		if ok && (k.String() != c.want || k.Code != c.code) {
			t.Errorf("%q %v: %q (code %q), want %q (code %q)", c.key, c.mod, k.String(), k.Code, c.want, c.code)
		}
	}
	if ModOf([]string{"ctrl", "shift", "alt", "super", "hyper"}) != ModCtrl|ModShift|ModAlt|ModSuper {
		t.Error("ModOf")
	}
}

// headlessCore is a core of `shj serve` with the file name holding text open
// and the keys on the editor.
func headlessCore(t *testing.T, text string) (*Core, string) {
	t.Helper()
	dir := t.TempDir()
	p := writeFile(t, dir, "a.txt", text)
	sh := NewShared()
	c := newHeadlessCore(sh, dir)
	t.Cleanup(func() { c.Close(); sh.Close() })
	if !c.OpenFile(p) {
		t.Fatalf("open: %s", c.Msg)
	}
	c.Focus = FocusEditor
	return c, p
}

// The pointer on a pane puts the cursor on the character under the cell:
// the core maps the cell to the rune (a wide character takes two cells), and
// a drag selects.
func TestPaneMouseClickAndDrag(t *testing.T) {
	c, p := headlessCore(t, "あいう\nabcdef\n")
	c.BuildScreen(view(40, 10))
	b := c.CurText().Buf
	c.Update(Mouse{Pane: p, Row: 0, Col: 3, Button: MouseLeft, Action: MousePress, Clicks: 1})
	if b.Cur.Row != 0 || b.Cur.Col != 1 {
		t.Fatalf("cursor at %+v, want 0:1 (the second wide character)", b.Cur)
	}
	c.Update(Mouse{Pane: p, Row: 1, Col: 4, Button: MouseLeft, Action: MouseMotion})
	c.Update(Mouse{Pane: p, Row: 1, Col: 4, Button: MouseLeft, Action: MouseRelease})
	s, e, ok := b.Selection()
	if !ok || s.Row != 0 || s.Col != 1 || e.Row != 1 {
		t.Fatalf("selection %v %v %v", s, e, ok)
	}
	if c.IsDragging() {
		t.Error("still dragging after the release")
	}
	// a pane not on the screen is nothing
	c.Update(Mouse{Pane: "nope", Row: 0, Col: 0, Button: MouseLeft, Action: MousePress})
	if b.Cur.Row != 1 {
		t.Errorf("a press on no pane moved the cursor to %+v", b.Cur)
	}
}

// The wheel scrolls the text a row a step.
func TestPaneMouseWheel(t *testing.T) {
	c, p := headlessCore(t, strings.Repeat("line\n", 100))
	c.BuildScreen(view(40, 10))
	for range 3 {
		c.Update(Mouse{Pane: p, Button: MouseWheelDown, Action: MousePress})
	}
	if got := c.CurText().Buf.ScrollRow; got != 3 {
		t.Fatalf("scrolled to %d, want 3", got)
	}
}

// What an input method composes shows underlined at the cursor and never
// edits the buffer; the end of the composition takes it away.
func TestPreeditShownAtCursor(t *testing.T) {
	c, _ := headlessCore(t, "abc\n")
	c.Update(Key{Code: 'l', Text: "l"})
	c.Update(Composition{Text: "にほ", Phase: CompositionStart})
	sc := c.BuildScreen(view(40, 10))
	var got []string
	for _, s := range sc.Files.Panes[0].Lines[0].Spans {
		if s.Underline {
			got = append(got, "_"+s.Text+"_")
		} else {
			got = append(got, s.Text)
		}
	}
	if strings.Join(got, "") != "a_にほ_bc" {
		t.Errorf("line: %v", got)
	}
	if text := c.CurText().Buf.Text(); text != "abc\n" {
		t.Errorf("the buffer is %q", text)
	}
	c.Update(Composition{Phase: CompositionEnd})
	sc = c.BuildScreen(view(40, 10))
	for _, s := range sc.Files.Panes[0].Lines[0].Spans {
		if s.Underline {
			t.Errorf("still composing: %+v", sc.Files.Panes[0].Lines[0])
		}
	}
}

// file.open shows the file with the cursor where it says (0-based). Line 0 is
// a place like any other; without a place, an open file keeps its cursor.
func TestOpenAt(t *testing.T) {
	c, _ := headlessCore(t, "x\n")
	p := writeFile(t, c.Root, "b.txt", "one\ntwo\nthree\n")
	if _, ok := c.OpenAt(p, &screen.Pos{Line: 2, Col: 3}, ""); !ok {
		t.Fatalf("open: %s", c.Msg)
	}
	tb := c.CurText()
	if tb.Path() != p || tb.Buf.Cur.Row != 2 || tb.Buf.Cur.Col != 3 || c.Focus != FocusEditor {
		t.Errorf("at %s %+v focus %v", tb.Path(), tb.Buf.Cur, c.Focus)
	}
	c.OpenAt(p, nil, "")
	if tb.Buf.Cur.Row != 2 || tb.Buf.Cur.Col != 3 {
		t.Errorf("opening again with no place moved the cursor to %+v", tb.Buf.Cur)
	}
	c.OpenAt(p, &screen.Pos{}, "")
	if tb.Buf.Cur.Row != 0 || tb.Buf.Cur.Col != 0 {
		t.Errorf("line 0 left the cursor at %+v", tb.Buf.Cur)
	}
}

// A core of Workspaces carries out the Effects it asks for: its watch timer
// fires, and the change is reported. Once the workspace is closed, nothing
// runs, and the goroutines that waited on the core's channels are gone.
func TestWorkspaceRunsEffects(t *testing.T) {
	sh := NewShared()
	defer sh.Close()
	ws := NewWorkspaces(sh, nil)
	defer ws.CloseAll()
	changed := make(chan struct{}, 1)
	ws.SetOnChange(func() {
		select {
		case changed <- struct{}{}:
		default:
		}
	})
	before := runtime.NumGoroutine()
	w := ws.Open(t.TempDir())
	select {
	case <-changed:
	case <-time.After(10 * time.Second):
		t.Fatal("the watch timer never came back")
	}
	ws.Close(w.Root)
	ran := false
	w.Apply(func(*Core) []Effect { ran = true; return nil })
	if ran {
		t.Error("a closed workspace still runs")
	}
	deadline := time.Now().Add(10 * time.Second)
	for runtime.NumGoroutine() > before {
		if time.Now().After(deadline) {
			t.Fatalf("%d goroutines left after Close, %d before Open", runtime.NumGoroutine(), before)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
