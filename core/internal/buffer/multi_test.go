package buffer

import (
	"regexp"
	"testing"
)

func multiBuf(text string, n int) *Buffer {
	b := New()
	b.Insert(text)
	b.undo = nil
	b.Cur = Pos{}
	for range n {
		b.CopySelection(1)
	}
	return b
}

func TestMultiCursorTypeDeleteUndo(t *testing.T) {
	b := multiBuf("a\nb\nc", 2)
	if len(b.Extra()) != 2 {
		t.Fatalf("cursors = %d", len(b.Extra())+1)
	}
	b.ForEachCursor(func() { b.Insert("xy") })
	if got := b.Text(); got != "xya\nxyb\nxyc" {
		t.Fatalf("insert -> %q", got)
	}
	b.ForEachCursor(func() { b.Backspace() })
	if got := b.Text(); got != "xa\nxb\nxc" {
		t.Fatalf("backspace -> %q", got)
	}
	b.Undo()
	b.Undo()
	if got := b.Text(); got != "a\nb\nc" {
		t.Fatalf("undo -> %q", got)
	}
	if b.HasExtra() {
		t.Fatal("undo should drop the extra cursors")
	}
}

func TestMultiCursorNewlineKeepsOthersAligned(t *testing.T) {
	b := multiBuf("a\nb", 1)
	b.ForEachCursor(func() { b.Insert("1\n2") })
	if got := b.Text(); got != "1\n2a\n1\n2b" {
		t.Fatalf("got %q", got)
	}
}

func TestCopySelectionStopsAtEnds(t *testing.T) {
	b := multiBuf("a\nb", 5)
	if len(b.Extra()) != 1 {
		t.Fatalf("cursors = %d", len(b.Extra())+1)
	}
	b.CopySelection(-1)
	b.ClearExtra()
	if b.HasExtra() {
		t.Fatal("ClearExtra")
	}
}

func TestSelectMatches(t *testing.T) {
	b := New()
	b.Insert("foo 1\nbar 22\nbaz")
	b.SelectAll()
	if err := b.SelectMatches(regexp.MustCompile(`\d+`)); err != nil {
		t.Fatal(err)
	}
	if len(b.Extra()) != 1 {
		t.Fatalf("cursors = %d", len(b.Extra())+1)
	}
	b.ForEachCursor(func() { b.DeleteSelection() })
	if got := b.Text(); got != "foo \nbar \nbaz" {
		t.Fatalf("got %q", got)
	}
	b.SelectAll()
	if err := b.SelectMatches(regexp.MustCompile(`zzz`)); err == nil {
		t.Fatal("expected no-match error")
	}
}

func TestBackwardRange(t *testing.T) {
	r := Range{Anchor: Pos{0, 3}, HasAnchor: true, Cur: Pos{0, 0}}
	if r.Start() != (Pos{0, 0}) || r.End() != (Pos{0, 3}) {
		t.Fatalf("Start/End = %v %v", r.Start(), r.End())
	}
	b := New()
	b.Insert("a1 b2")
	b.Anchor, b.Cur = &Pos{0, 5}, Pos{0, 0}
	if err := b.SelectMatches(regexp.MustCompile(`\d`)); err != nil || len(b.Extra()) != 1 {
		t.Fatalf("err=%v cursors=%d", err, len(b.Extra())+1)
	}
	b.setRanges([]Range{
		{Anchor: Pos{0, 4}, HasAnchor: true, Cur: Pos{0, 0}},
		{Anchor: Pos{0, 5}, HasAnchor: true, Cur: Pos{0, 2}},
	}, 0)
	if b.HasExtra() {
		t.Fatal("overlapping backward ranges should merge")
	}
}
