package core

import (
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
)

func TestLineDiffEdit(t *testing.T) {
	cases := []struct{ old, new string }{
		{"a\nb\nc\n", "a\nB\nc\n"},
		{"a\nb\nc\n", "a\nc\n"},
		{"a\nb\nc\n", "b\nc\n"},
		{"a\nb\nc", "a\nb"},
		{"a\nb\n", "a\nx\ny\nb\n"},
		{"b\n", "a\nb\n"},
		{"a\n", "a\nb\nc\n"},
		{"a\nb\n", "a\nb\n"},
		{"x", ""},
		{"", "x\n"},
	}
	for _, c := range cases {
		b := buffer.New()
		b.InsertAt(buffer.Pos{}, c.old)
		if e, ok := lineDiffEdit(b, c.new); ok {
			b.ApplyEdits([]buffer.Edit{e})
		}
		if got := b.Text(); got != c.new {
			t.Errorf("%q -> %q: got %q", c.old, c.new, got)
		}
	}
}
