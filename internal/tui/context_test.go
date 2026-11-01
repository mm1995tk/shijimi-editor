package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
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
	a.handleKey(Key{Code: KeyEsc})
	if a.ctx != nil {
		t.Errorf("esc did not close preview")
	}
	b := a.curText().buf
	b.SelectRange(buffer.Pos{Row: 0, Col: 1}, buffer.Pos{Row: 1, Col: 2})
	a.sendContext()
	if a.ctx == nil || !strings.HasPrefix(a.ctx.text, "a.txt:1-2\n```\nne\ntw\n```") {
		t.Errorf("selection ctx = %+v", a.ctx)
	}
	if len(a.render()) == 0 {
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

func TestSendReviewCommentsRelativeToRoot(t *testing.T) {
	a := New(t.TempDir(), "")
	a.gitTop = a.root
	a.root = filepath.Join(a.gitTop, "sub")
	if got := a.reviewToRoot("sub/pkg/x.go"); got != "pkg/x.go" {
		t.Errorf("got %q", got)
	}
}
