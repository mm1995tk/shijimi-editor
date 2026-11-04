package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mm1995tk/shijimi-editor/core"
)

func TestSendContextPreview(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	os.WriteFile(f, []byte("one\ntwo\nthree\n"), 0o644)
	a := mustNew(dir, f)
	a.w, a.h = 100, 30
	a.pushSizes()
	if a.SendContext(); a.Ctx == nil || !strings.HasPrefix(a.Ctx.Text, "a.txt:1\n") {
		t.Fatalf("ctx = %+v", a.Ctx)
	}
	a.HandleKey(core.Key{Code: KeyEsc})
	if a.Ctx != nil {
		t.Errorf("esc did not close preview")
	}
	b := a.CurText().Buf
	b.SelectRange(core.Pos{Row: 0, Col: 1}, core.Pos{Row: 1, Col: 2})
	a.SendContext()
	if a.Ctx == nil || !strings.HasPrefix(a.Ctx.Text, "a.txt:1-2\n```\nne\ntw\n```") {
		t.Errorf("selection ctx = %+v", a.Ctx)
	}
	if len(a.render()) == 0 {
		t.Errorf("empty view")
	}
}
