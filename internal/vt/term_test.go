package vt

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func newT(cols, rows int) (*Term, *bytes.Buffer) {
	var reply bytes.Buffer
	return New(Options{Cols: cols, Rows: rows, Reply: &reply}), &reply
}

func snap(t *Term) *Snapshot {
	s := &Snapshot{}
	t.Snapshot(s)
	return s
}

// rowText joins the characters of row y; wide characters count once.
func rowText(s *Snapshot, y int) string {
	var sb strings.Builder
	for _, c := range s.Cells[y] {
		if c.Width == 0 {
			continue
		}
		if c.Rune == 0 {
			sb.WriteByte(' ')
		} else {
			sb.WriteString(c.Text())
		}
	}
	return strings.TrimRight(sb.String(), " ")
}

func TestWideCharsTakeTwoCells(t *testing.T) {
	term, _ := newT(10, 3)
	term.Write([]byte("日本語a"))
	s := snap(term)
	if s.CursorX != 7 || s.CursorY != 0 {
		t.Fatalf("cursor = %d,%d want 7,0", s.CursorX, s.CursorY)
	}
	for i, w := range []int8{2, 0, 2, 0, 2, 0, 1} {
		if s.Cells[0][i].Width != w {
			t.Errorf("cell %d width = %d want %d", i, s.Cells[0][i].Width, w)
		}
	}
	if got := rowText(s, 0); got != "日本語a" {
		t.Fatalf("row = %q", got)
	}
}

func TestWideCharWrapsAtLastColumn(t *testing.T) {
	term, _ := newT(5, 3)
	term.Write([]byte("abcd日"))
	s := snap(term)
	if rowText(s, 0) != "abcd" || rowText(s, 1) != "日" {
		t.Fatalf("rows = %q %q", rowText(s, 0), rowText(s, 1))
	}
	if s.CursorX != 2 || s.CursorY != 1 {
		t.Fatalf("cursor = %d,%d want 2,1", s.CursorX, s.CursorY)
	}
}

func TestOverwritingHalfOfWideCharBlanksIt(t *testing.T) {
	term, _ := newT(6, 2)
	term.Write([]byte("日本\x1b[1;2Hx"))
	s := snap(term)
	if got := rowText(s, 0); got != " x本" {
		t.Fatalf("row = %q", got)
	}
}

func TestCombiningMarkJoinsPreviousCell(t *testing.T) {
	term, _ := newT(6, 2)
	term.Write([]byte("éx"))
	s := snap(term)
	if got := s.Cells[0][0].Text(); got != "é" {
		t.Fatalf("cell = %q", got)
	}
	if s.CursorX != 2 {
		t.Fatalf("cursor x = %d", s.CursorX)
	}
}

func TestSplitUTF8AcrossWrites(t *testing.T) {
	term, _ := newT(6, 2)
	b := []byte("あ")
	term.Write(b[:1])
	term.Write(b[1:2])
	term.Write(b[2:])
	if got := rowText(snap(term), 0); got != "あ" {
		t.Fatalf("row = %q", got)
	}
}

func TestScrollbackKeepsLinesThatScrolledOff(t *testing.T) {
	term, _ := newT(10, 3)
	for i := 1; i <= 6; i++ {
		fmt.Fprintf(term, "line%d\r\n", i)
	}
	if n := term.Scrollback(); n != 4 {
		t.Fatalf("scrollback = %d want 4", n)
	}
	if got := rowText(snap(term), 0); got != "line5" {
		t.Fatalf("live top = %q", got)
	}
	term.ScrollView(2)
	s := snap(term)
	if rowText(s, 0) != "line3" || rowText(s, 2) != "line5" || s.Offset != 2 || s.CursorVisible {
		t.Fatalf("view = %q %q off=%d cur=%v", rowText(s, 0), rowText(s, 2), s.Offset, s.CursorVisible)
	}
	// New output keeps the scrolled view where it is.
	term.Write([]byte("line7\r\n"))
	if got := rowText(snap(term), 0); got != "line3" {
		t.Fatalf("view moved: %q", got)
	}
	if got := term.ScrollView(100); got != term.Scrollback() {
		t.Fatalf("view not clamped: %d", got)
	}
	term.ScrollToBottom()
	if snap(term).Offset != 0 {
		t.Fatal("not at bottom")
	}
}

func TestScrollbackLimit(t *testing.T) {
	term := New(Options{Cols: 5, Rows: 2, Scrollback: 10})
	for i := 0; i < 100; i++ {
		fmt.Fprintf(term, "%d\r\n", i)
	}
	if n := term.Scrollback(); n < 10 || n > 13 {
		t.Fatalf("scrollback = %d", n)
	}
	off := New(Options{Cols: 5, Rows: 2, Scrollback: -1})
	off.Write([]byte("a\r\nb\r\nc\r\n"))
	if off.Scrollback() != 0 {
		t.Fatal("scrollback not disabled")
	}
}

func TestAltScreenHasNoScrollbackAndRestores(t *testing.T) {
	term, _ := newT(10, 3)
	term.Write([]byte("main\r\n"))
	term.Write([]byte("\x1b[?1049h\x1b[Halt\r\n\r\n\r\n\r\n"))
	s := snap(term)
	if !s.Modes.AltScreen || term.Scrollback() != 0 {
		t.Fatalf("alt=%v sb=%d", s.Modes.AltScreen, term.Scrollback())
	}
	term.Write([]byte("\x1b[?1049l"))
	s = snap(term)
	if s.Modes.AltScreen || rowText(s, 0) != "main" || s.CursorY != 1 {
		t.Fatalf("restore: alt=%v row=%q cy=%d", s.Modes.AltScreen, rowText(s, 0), s.CursorY)
	}
}

func TestSGR(t *testing.T) {
	term, _ := newT(20, 2)
	term.Write([]byte("\x1b[1;31ma\x1b[38;5;200;48;2;1;2;3mb\x1b[38:2::9:8:7mc\x1b[0md\x1b[7;4me"))
	s := snap(term)
	a, b, c, d, e := s.Cells[0][0], s.Cells[0][1], s.Cells[0][2], s.Cells[0][3], s.Cells[0][4]
	if i, _ := a.FG.Index(); i != 1 || a.Attr != AttrBold {
		t.Errorf("a: %+v", a)
	}
	if i, _ := b.FG.Index(); i != 200 {
		t.Errorf("b fg: %+v", b)
	}
	if r, g, bl, ok := b.BG.Components(); !ok || r != 1 || g != 2 || bl != 3 {
		t.Errorf("b bg: %+v", b)
	}
	if r, g, bl, ok := c.FG.Components(); !ok || r != 9 || g != 8 || bl != 7 {
		t.Errorf("c fg: %+v", c)
	}
	if !d.FG.IsDefault() || d.Attr != 0 {
		t.Errorf("d: %+v", d)
	}
	if e.Attr != AttrReverse|AttrUnderline {
		t.Errorf("e: %+v", e)
	}
}

func TestUnderlineColorArgumentsAreSkipped(t *testing.T) {
	term, _ := newT(10, 1)
	term.Write([]byte("\x1b[58;2;1;2;3;31ma"))
	if i, ok := snap(term).Cells[0][0].FG.Index(); !ok || i != 1 {
		t.Fatal("31 after 58;2;r;g;b not applied")
	}
}

func TestEraseUsesCurrentBackground(t *testing.T) {
	term, _ := newT(5, 2)
	term.Write([]byte("\x1b[44m\x1b[2J"))
	if i, ok := snap(term).Cells[1][4].BG.Index(); !ok || i != 4 {
		t.Fatal("erased cell lost background")
	}
}

func TestScrollRegion(t *testing.T) {
	term, _ := newT(5, 5)
	term.Write([]byte("1\r\n2\r\n3\r\n4\r\n5"))
	term.Write([]byte("\x1b[2;4r\x1b[4;1H\n"))
	s := snap(term)
	got := []string{rowText(s, 0), rowText(s, 1), rowText(s, 2), rowText(s, 3), rowText(s, 4)}
	want := []string{"1", "3", "4", "", "5"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rows = %q want %q", got, want)
		}
	}
	if term.Scrollback() != 0 {
		t.Fatal("partial region must not feed the scrollback")
	}
}

func TestInsertDeleteLinesAndChars(t *testing.T) {
	term, _ := newT(6, 3)
	term.Write([]byte("abcdef\r\nxyz\r\n123\x1b[1;2H\x1b[2P"))
	if got := rowText(snap(term), 0); got != "adef" {
		t.Fatalf("DCH: %q", got)
	}
	term.Write([]byte("\x1b[1;2H\x1b[@"))
	if got := rowText(snap(term), 0); got != "a def" {
		t.Fatalf("ICH: %q", got)
	}
	term.Write([]byte("\x1b[2;1H\x1b[L"))
	s := snap(term)
	if rowText(s, 1) != "" || rowText(s, 2) != "xyz" {
		t.Fatalf("IL: %q %q", rowText(s, 1), rowText(s, 2))
	}
	term.Write([]byte("\x1b[M"))
	if got := rowText(snap(term), 1); got != "xyz" {
		t.Fatalf("DL: %q", got)
	}
}

func TestDECGraphics(t *testing.T) {
	term, _ := newT(5, 1)
	term.Write([]byte("\x1b(0lqk\x1b(Bq"))
	if got := rowText(snap(term), 0); got != "┌─┐q" {
		t.Fatalf("row = %q", got)
	}
}

func TestRepliesGoToWriter(t *testing.T) {
	term, reply := newT(20, 10)
	term.Write([]byte("\x1b[3;5H\x1b[6n\x1b[5n\x1b[c"))
	if got := reply.String(); got != "\x1b[3;5R\x1b[0n\x1b[?62;22c" {
		t.Fatalf("reply = %q", got)
	}
}

func TestModes(t *testing.T) {
	term, _ := newT(10, 3)
	term.Write([]byte("\x1b[?1h\x1b[?2004h\x1b[?1002h\x1b[?1006h"))
	m := term.Modes()
	if !m.AppCursor || !m.Bracketed || m.Mouse != MouseDrag || !m.MouseSGR {
		t.Fatalf("modes = %+v", m)
	}
	term.Write([]byte("\x1b[?2004l\x1b[?1002l"))
	m = term.Modes()
	if m.Bracketed || m.Mouse != MouseOff {
		t.Fatalf("modes = %+v", m)
	}
}

func TestTitle(t *testing.T) {
	term, _ := newT(10, 3)
	term.Write([]byte("\x1b]0;hello 世界\x07"))
	if term.Title() != "hello 世界" {
		t.Fatalf("title = %q", term.Title())
	}
	term.Write([]byte("\x1b]2;two\x1b\\"))
	if term.Title() != "two" {
		t.Fatalf("title = %q", term.Title())
	}
}

func TestCursorVisibility(t *testing.T) {
	term, _ := newT(10, 3)
	term.Write([]byte("\x1b[?25l"))
	if snap(term).CursorVisible {
		t.Fatal("cursor should be hidden")
	}
}

func TestResizeMovesTopLinesToScrollbackAndBack(t *testing.T) {
	term, _ := newT(10, 4)
	term.Write([]byte("a\r\nb\r\nc\r\nd"))
	term.Resize(10, 2)
	s := snap(term)
	if rowText(s, 0) != "c" || rowText(s, 1) != "d" || term.Scrollback() != 2 || s.CursorY != 1 {
		t.Fatalf("shrink: %q %q sb=%d cy=%d", rowText(s, 0), rowText(s, 1), term.Scrollback(), s.CursorY)
	}
	term.Resize(10, 4)
	s = snap(term)
	if rowText(s, 0) != "a" || rowText(s, 3) != "d" || s.CursorY != 3 || term.Scrollback() != 0 {
		t.Fatalf("grow: %q %q sb=%d cy=%d", rowText(s, 0), rowText(s, 3), term.Scrollback(), s.CursorY)
	}
}

func TestResizeNarrowerCutsWideCharCleanly(t *testing.T) {
	term, _ := newT(6, 2)
	term.Write([]byte("abc日"))
	term.Resize(4, 2)
	s := snap(term)
	if got := rowText(s, 0); got != "abc" {
		t.Fatalf("row = %q", got)
	}
}

func TestAutowrapOffOverwritesLastColumn(t *testing.T) {
	term, _ := newT(3, 2)
	term.Write([]byte("\x1b[?7labcde"))
	s := snap(term)
	if rowText(s, 0) != "abe" || rowText(s, 1) != "" {
		t.Fatalf("rows = %q %q", rowText(s, 0), rowText(s, 1))
	}
}

func TestRepeatAndTabs(t *testing.T) {
	term, _ := newT(20, 1)
	term.Write([]byte("a\x1b[3b\tb"))
	if got := rowText(snap(term), 0); got != "aaaa    b" {
		t.Fatalf("row = %q", got)
	}
}

func TestEncodeMouse(t *testing.T) {
	cases := []struct {
		name string
		mode MouseMode
		sgr  bool
		ev   MouseEvent
		want string
	}{
		{"off", MouseOff, true, MouseEvent{Button: ButtonLeft}, ""},
		{"sgr press", MouseClick, true, MouseEvent{Button: ButtonLeft, X: 4, Y: 9}, "\x1b[<0;5;10M"},
		{"sgr release", MouseClick, true, MouseEvent{Button: ButtonLeft, Action: MouseRelease, X: 4, Y: 9}, "\x1b[<0;5;10m"},
		{"sgr wheel", MouseClick, true, MouseEvent{Button: WheelUp, X: 0, Y: 0}, "\x1b[<64;1;1M"},
		{"sgr ctrl right", MouseClick, true, MouseEvent{Button: ButtonRight, Ctrl: true}, "\x1b[<18;1;1M"},
		{"motion ignored in click mode", MouseClick, true, MouseEvent{Button: ButtonLeft, Action: MouseMotion}, ""},
		{"drag motion", MouseDrag, true, MouseEvent{Button: ButtonLeft, Action: MouseMotion, X: 1}, "\x1b[<32;2;1M"},
		{"bare motion needs any", MouseDrag, true, MouseEvent{Button: ButtonNone, Action: MouseMotion}, ""},
		{"any motion", MouseAny, true, MouseEvent{Button: ButtonNone, Action: MouseMotion}, "\x1b[<35;1;1M"},
		{"legacy press", MouseClick, false, MouseEvent{Button: ButtonLeft, X: 1, Y: 2}, "\x1b[M" + string([]byte{32, 34, 35})},
		{"legacy release", MouseClick, false, MouseEvent{Button: ButtonLeft, Action: MouseRelease}, "\x1b[M#!!"},
		{"legacy out of range", MouseClick, false, MouseEvent{Button: ButtonLeft, X: 300}, ""},
	}
	for _, c := range cases {
		got := string(encodeMouse(c.mode, c.sgr, c.ev))
		if got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestEmojiClustersTakeTheirDrawnWidth(t *testing.T) {
	for _, s := range []string{"❤️", "👨‍👩‍👧", "👍\U0001f3fd"} {
		term, _ := newT(10, 2)
		term.Write([]byte(s + "x"))
		sn := snap(term)
		if sn.Cells[0][0].Width != 2 || sn.Cells[0][1].Width != 0 || sn.Cells[0][2].Text() != "x" || sn.CursorX != 3 {
			t.Errorf("%q: widths %d,%d cell2=%q cursor=%d", s, sn.Cells[0][0].Width, sn.Cells[0][1].Width, sn.Cells[0][2].Text(), sn.CursorX)
		}
	}
}

func TestControlCharactersAreNotStored(t *testing.T) {
	term, _ := newT(10, 2)
	term.Write([]byte("a\u009bb\u007f\u0085c​d"))
	sn := snap(term)
	if got := rowText(sn, 0); got != "abcd" {
		t.Fatalf("row = %q", got)
	}
	for _, c := range sn.Cells[0] {
		if c.Comb != "" {
			t.Fatalf("combining text stored: %q", c.Comb)
		}
	}
}

func TestOSC52CopyIsTakenOnce(t *testing.T) {
	term, _ := newT(10, 3)
	term.Write([]byte("\x1b]52;c;5pel5pys6Kqe\x07"))
	if s, ok := term.TakeClipboard(); !ok || s != "日本語" {
		t.Fatalf("TakeClipboard = %q, %v; want 日本語", s, ok)
	}
	if _, ok := term.TakeClipboard(); ok {
		t.Fatal("TakeClipboard returned the text twice")
	}
	term.Write([]byte("\x1b]52;c;?\x07"))
	if _, ok := term.TakeClipboard(); ok {
		t.Fatal("a clipboard query was taken as a copy")
	}
}
