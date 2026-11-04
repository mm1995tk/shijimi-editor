package core

import (
	"strings"
	"testing"
)

func TestTextAreaEdits(t *testing.T) {
	ta := NewTextArea("あいう")
	ta.Update(Key{Code: 'x', Text: "x"})
	ta.Update(Key{Code: KeyLeft})
	ta.Update(Key{Code: KeyBackspace})
	if ta.Value() != "あいx" {
		t.Fatalf("got %q", ta.Value())
	}
	ta.Update(Paste{Content: "a\r\nb"})
	if ta.Value() != "あいa\nbx" {
		t.Fatalf("got %q", ta.Value())
	}
	if ta.Line() != 1 || ta.Column() != 1 {
		t.Fatalf("cursor %d,%d", ta.Line(), ta.Column())
	}
	ta.Update(Key{Code: 'u', Mod: ModCtrl})
	if ta.Value() != "あいa\nx" {
		t.Fatalf("got %q", ta.Value())
	}
	ta.Update(Key{Code: KeyBackspace})
	ta.Update(Key{Code: KeyBackspace})
	if ta.Value() != "あいx" {
		t.Fatalf("got %q", ta.Value())
	}
}

func TestTextAreaWrapsAndScrolls(t *testing.T) {
	ta := NewTextArea(strings.Repeat("a", 25))
	ta.SetWidth(10)
	if rows := ta.Rows(); rows[0] != strings.Repeat("a", 10) || rows[2] != strings.Repeat("a", 5) {
		t.Fatalf("rows %q", rows)
	}
	if x, y, ok := ta.CursorCell(); !ok || x != 5 || y != 2 {
		t.Fatalf("cursor %d,%d,%v", x, y, ok)
	}
	ta.SetValue(strings.Repeat("l\n", 10) + "end")
	if x, y, ok := ta.CursorCell(); !ok || x != 3 || y != textAreaRows-1 {
		t.Fatalf("cursor %d,%d,%v", x, y, ok)
	}
	if ta.Rows()[textAreaRows-1] != "end" {
		t.Fatalf("rows %q", ta.Rows())
	}
}
