package buffer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPoolSharesText(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(p, []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(p, link); err != nil {
		t.Fatal(err)
	}
	pool := NewPool()
	a, err := pool.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	b, err := pool.Open(link)
	if err != nil {
		t.Fatal(err)
	}
	if a.Doc != b.Doc || pool.Len() != 1 {
		t.Fatal("a symbolic link to an open file is another text")
	}
	b.Cur = Pos{1, 2}
	a.Insert("X")
	if b.Text() != "Xone\ntwo\n" || a.Cur != (Pos{0, 1}) || b.Cur != (Pos{1, 2}) {
		t.Errorf("text %q, cursors %v %v", b.Text(), a.Cur, b.Cur)
	}
	a.Insert("\n")
	if b.Cur != (Pos{2, 2}) {
		t.Errorf("a line added above moved B's cursor to %v, want {2 2}", b.Cur)
	}
	// B undoes A's edits; A's cursor is made to fit
	a.Cur = Pos{1, 0}
	b.Undo()
	b.Undo()
	a.Settle()
	if a.Cur.Row > len(a.Lines)-1 {
		t.Errorf("A's cursor %v is outside the text", a.Cur)
	}
	a.Release()
	if pool.Len() != 1 {
		t.Error("the text went with the first view")
	}
	b.Release()
	if pool.Len() != 0 {
		t.Error("the text stayed after its last view")
	}
}

func TestPoolFollowsRename(t *testing.T) {
	dir := t.TempDir()
	p, q := filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")
	if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pool := NewPool()
	a, _ := pool.Open(p)
	if err := os.Rename(p, q); err != nil {
		t.Fatal(err)
	}
	a.SetPath(q)
	b, err := pool.Open(q)
	if err != nil {
		t.Fatal(err)
	}
	if a.Doc != b.Doc {
		t.Error("the renamed file opened as a second text")
	}
	c, _ := pool.Open(p)
	if c.Doc == a.Doc {
		t.Error("the old name still finds the renamed file")
	}
}
