package core

import (
	"strings"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/config"
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

func TestFormatterCommand(t *testing.T) {
	a := newTestApp(t, "one\ntwo\nthree\n")
	tb := a.CurText()
	b := tb.Buf
	// upper-cases stdin, and echoes the buffer name on the first line
	tb.Lang = &config.Language{Name: "x", Formatter: &config.Formatter{
		Command: "sh", Args: []string{"-c", `echo "$1"; tr a-z A-Z`, "sh", "%{buffer_name}"},
	}}
	b.Cur = buffer.Pos{Row: 2, Col: 2}
	msg := runOf(t, a.Format(false))
	a.Update(msg)
	if got, want := b.Text(), "a.txt\nONE\nTWO\nTHREE\n"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if b.Cur != (buffer.Pos{Row: 2, Col: 2}) {
		t.Fatalf("cursor moved: %v", b.Cur)
	}
	b.Undo()
	if got := b.Text(); got != "one\ntwo\nthree\n" {
		t.Fatalf("undo once -> %q", got)
	}
}

func TestFormatterFailureKeepsBuffer(t *testing.T) {
	a := newTestApp(t, "one\n")
	tb := a.CurText()
	for _, f := range []*config.Formatter{
		{Command: "sh", Args: []string{"-c", "echo boom >&2; echo garbage; exit 3"}},
		{Command: "shijimi-no-such-formatter"},
	} {
		tb.Lang = &config.Language{Name: "x", Formatter: f}
		a.Update(runOf(t, a.Format(false)))
		if got := tb.Buf.Text(); got != "one\n" {
			t.Fatalf("buffer changed: %q", got)
		}
	}
	tb.Lang.Formatter = &config.Formatter{Command: "sh", Args: []string{"-c", "echo boom >&2; exit 1"}}
	msg := runOf(t, a.Format(false)).(FormatterDoneMsg)
	if msg.Err == nil || !strings.Contains(msg.Err.Error(), "boom") {
		t.Fatalf("err = %v", msg.Err)
	}
}
