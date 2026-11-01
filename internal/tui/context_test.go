package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
)

func TestSendContextPreview(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	os.WriteFile(f, []byte("one\ntwo\nthree\n"), 0o644)
	a := newApp(dir, f)
	a.w, a.h = 100, 30
	if a.sendContext(); a.ctx == nil || !strings.HasPrefix(a.ctx.Text, "a.txt:1\n") {
		t.Fatalf("ctx = %+v", a.ctx)
	}
	a.handleKey(Key{Code: KeyEsc})
	if a.ctx != nil {
		t.Errorf("esc did not close preview")
	}
	b := a.CurText().Buf
	b.SelectRange(buffer.Pos{Row: 0, Col: 1}, buffer.Pos{Row: 1, Col: 2})
	a.sendContext()
	if a.ctx == nil || !strings.HasPrefix(a.ctx.Text, "a.txt:1-2\n```\nne\ntw\n```") {
		t.Errorf("selection ctx = %+v", a.ctx)
	}
	if len(a.render()) == 0 {
		t.Errorf("empty view")
	}
}
