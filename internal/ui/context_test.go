package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mm1995tk/gh-editor/internal/buffer"
	"github.com/mm1995tk/gh-editor/internal/diff"
	"github.com/mm1995tk/gh-editor/internal/lsp"
)

func TestFormatContext(t *testing.T) {
	diags := []lsp.Diagnostic{
		{Range: lsp.Range{Start: lsp.Position{Line: 1}, End: lsp.Position{Line: 1, Character: 3}}, Severity: 1, Message: "bad\nmore", Source: "gopls"},
		{Range: lsp.Range{Start: lsp.Position{Line: 9}, End: lsp.Position{Line: 9}}, Severity: 2, Message: "far"},
	}
	got := formatContext("a.go", 1, 2, "x := 1\ny", diags)
	want := "a.go:2-3\n```\nx := 1\ny\n```\na.go:2: [エラー] bad (gopls)\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
	if got := formatContext("a.go", 4, 4, "", nil); got != "a.go:5\n" {
		t.Errorf("cursor only: %q", got)
	}
	long := strings.Repeat("l\n", 300)
	if got := formatContext("a.go", 0, 299, long, nil); !strings.Contains(got, "残り 101 行を省略") {
		t.Errorf("not truncated: %q", got[len(got)-60:])
	}
}

func TestSendContextPreview(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	os.WriteFile(f, []byte("one\ntwo\nthree\n"), 0o644)
	a := New(dir, f)
	a.w, a.h = 100, 30
	if a.sendContext(); a.ctx == nil || !strings.HasPrefix(a.ctx.text, "a.txt:1\n") {
		t.Fatalf("ctx = %+v", a.ctx)
	}
	a.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if a.ctx != nil {
		t.Errorf("esc did not close preview")
	}
	b := a.curText().buf
	b.SelectRange(buffer.Pos{Row: 0, Col: 1}, buffer.Pos{Row: 1, Col: 2})
	a.sendContext()
	if a.ctx == nil || !strings.HasPrefix(a.ctx.text, "a.txt:1-2\n```\nne\ntw\n```") {
		t.Errorf("selection ctx = %+v", a.ctx)
	}
	if len(a.View()) == 0 {
		t.Errorf("empty view")
	}
}

func TestSendContextLineSelection(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	os.WriteFile(f, []byte("one\ntwo\nthree\n"), 0o644)
	a := New(dir, f)
	b := a.curText().buf
	b.SelectRange(buffer.Pos{Row: 0, Col: 0}, buffer.Pos{Row: 2, Col: 0})
	a.sendContext()
	if want := "a.txt:1-2\n```\none\ntwo\n```\n"; a.ctx == nil || a.ctx.text != want {
		t.Errorf("got %+v want %q", a.ctx, want)
	}
}

func TestBracketedAfter(t *testing.T) {
	h, l := "\x1b[?2004h", "\x1b[?2004l"
	for _, c := range []struct {
		out  string
		cur  bool
		want bool
	}{
		{"plain", true, true},
		{"plain", false, false},
		{l + "out" + h, false, true},
		{h + "out" + l, true, false},
		{h, false, true},
		{l, true, false},
	} {
		if got := bracketedAfter([]byte(c.out), c.cur); got != c.want {
			t.Errorf("%q cur=%v: got %v", c.out, c.cur, got)
		}
	}
}

func TestSendReviewComments(t *testing.T) {
	a := New(t.TempDir(), "")
	a.w, a.h = 100, 30
	r := newReview(".", 0)
	r.loading, r.files = false, []*diff.File{{}}
	tb := &tab{kind: tabReview, rev: r}
	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}

	if a.handleReviewKey(tb, key); a.ctx != nil {
		t.Fatalf("preview opened without comments")
	}
	r.comments = append(r.comments, &reviewComment{Path: "a.go", Line: 3, Body: "直して", Pending: true})
	a.handleReviewKey(tb, key)
	want := reviewPrompt + "- a.go:3\n  直して\n"
	if a.ctx == nil || a.ctx.text != want {
		t.Errorf("ctx = %+v, want %q", a.ctx, want)
	}
}
