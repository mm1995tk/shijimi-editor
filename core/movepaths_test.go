package core

import (
	"os"
	"path/filepath"
	"testing"
)

// 名前の変更に合わせて、開いているタブのパスが新しいパスに移る（フォルダなら中のファイルも）。
func TestMovePathsFollowsRename(t *testing.T) {
	a := newTestApp(t, "1\n")
	dir := filepath.Dir(a.CurText().Buf.Path)
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0o755)
	inner := filepath.Join(sub, "in.txt")
	os.WriteFile(inner, []byte("x\n"), 0o644)
	a.OpenFile(inner)
	moved := filepath.Join(dir, "moved")
	a.MovePaths(sub, moved)
	got := ""
	for _, tb := range a.Tabs {
		if tb.Kind == TabText && filepath.Base(tb.Buf.Path) == "in.txt" {
			got = tb.Buf.Path
		}
	}
	if want := filepath.Join(moved, "in.txt"); got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}
