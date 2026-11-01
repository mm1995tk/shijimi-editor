package core

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// writeFile makes dir/name holding text.
func writeFile(t *testing.T, dir, name, text string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// Two workspaces in one process each have a core of their own: the files one
// opens and edits are not the other's, and both share the process's parts.
func TestWorkspacesEditSeparately(t *testing.T) {
	r1, r2 := t.TempDir(), t.TempDir()
	p1 := writeFile(t, r1, "a.txt", "one\n")
	p2 := writeFile(t, r2, "a.txt", "two\n")

	ws := NewWorkspaces(NewShared(), nil)
	defer ws.CloseAll()
	w1, w2 := ws.Open(r1), ws.Open(r2)
	if w1 == w2 {
		t.Fatal("two roots got the same workspace")
	}
	if ws.Open(r1) != w1 {
		t.Fatal("opening a root again made another core")
	}

	w1.Do(func(c *Core) {
		if !c.OpenFile(p1) {
			t.Fatalf("open %s: %s", p1, c.Msg)
		}
		c.Focus = FocusEditor
		keys(c, "i", "X", "esc")
	})
	w2.Do(func(c *Core) {
		if !c.OpenFile(p2) {
			t.Fatalf("open %s: %s", p2, c.Msg)
		}
		c.Focus = FocusEditor
		keys(c, "i", "Y", "esc")
	})

	var c1, c2 *Core
	w1.Do(func(c *Core) { c1 = c })
	w2.Do(func(c *Core) { c2 = c })
	if c1.Root == c2.Root {
		t.Fatalf("both cores are rooted at %s", c1.Root)
	}
	if got := c1.CurText().Buf.Text(); got != "Xone\n" {
		t.Errorf("workspace 1 = %q, want %q", got, "Xone\n")
	}
	if got := c2.CurText().Buf.Text(); got != "Ytwo\n" {
		t.Errorf("workspace 2 = %q, want %q", got, "Ytwo\n")
	}
	if !slices.Equal(c1.OpenPaths(), []string{p1}) || !slices.Equal(c2.OpenPaths(), []string{p2}) {
		t.Errorf("open files = %v and %v, want [%s] and [%s]", c1.OpenPaths(), c2.OpenPaths(), p1, p2)
	}
	if c1.shared != c2.shared {
		t.Error("the cores do not share the process's parts")
	}

	// Closing one workspace leaves the other's core and files alone.
	ws.Close(r1)
	if ws.Lookup(r1) != nil {
		t.Error("the closed workspace is still there")
	}
	if ws.Lookup(r2) != w2 || len(c2.OpenPaths()) != 1 {
		t.Error("closing workspace 1 touched workspace 2")
	}
	if b, _ := os.ReadFile(p1); string(b) != "one\n" {
		t.Errorf("closing wrote the file: %q", b)
	}
}

// Before the server stops, the unsaved edits of every workspace go next to
// their files as <name>.bk; files without edits get nothing, and the files
// themselves stay as they were.
func TestWorkspacesWriteBackups(t *testing.T) {
	r1, r2 := t.TempDir(), t.TempDir()
	p1 := writeFile(t, r1, "a.txt", "one\n")
	p2 := writeFile(t, r2, "b.txt", "two\n")
	clean := writeFile(t, r2, "c.txt", "three\n")

	ws := NewWorkspaces(NewShared(), nil)
	defer ws.CloseAll()
	ws.Open(r1).Do(func(c *Core) {
		c.OpenFile(p1)
		c.CurText().Buf.Lines[0] = []rune("ONE")
		c.CurText().Buf.Dirty = true
	})
	ws.Open(r2).Do(func(c *Core) {
		c.OpenFile(clean)
		c.OpenFile(p2)
		c.CurText().Buf.Lines[0] = []rune("TWO")
		c.CurText().Buf.Dirty = true
	})

	write := func(path string, data []byte) (string, error) {
		name := path + ".bk"
		return name, os.WriteFile(name, data, 0o600)
	}
	got, err := ws.WriteBackups(write)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	want := []string{p1 + ".bk", p2 + ".bk"}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("written = %v, want %v", got, want)
	}
	for p, text := range map[string]string{p1 + ".bk": "ONE\n", p2 + ".bk": "TWO\n", p1: "one\n", p2: "two\n"} {
		if b, _ := os.ReadFile(p); string(b) != text {
			t.Errorf("%s = %q, want %q", p, b, text)
		}
	}
	if _, err := os.Stat(clean + ".bk"); err == nil {
		t.Error("a file with no edits got a backup")
	}
}

// CloseFiles closes what has no unsaved edits and keeps the rest.
func TestCloseFilesKeepsUnsaved(t *testing.T) {
	r := t.TempDir()
	p1 := writeFile(t, r, "a.txt", "one\n")
	p2 := writeFile(t, r, "b.txt", "two\n")
	ws := NewWorkspaces(NewShared(), nil)
	defer ws.CloseAll()
	ws.Open(r).Do(func(c *Core) {
		c.OpenFile(p1)
		c.OpenFile(p2)
		c.CurText().Buf.Dirty = true
		if kept := c.CloseFiles(); kept != 1 {
			t.Fatalf("kept %d files, want 1", kept)
		}
		if !slices.Equal(c.OpenPaths(), []string{p2}) || c.Cur() == nil {
			t.Errorf("open files = %v, want [%s] shown", c.OpenPaths(), p2)
		}
		c.CurText().Buf.Dirty = false
		c.CloseFiles()
		if c.HasOpenFiles() || c.Cur() != nil {
			t.Errorf("files left open: %v", c.OpenPaths())
		}
	})
}

// The file shown stays shown when files before it are closed.
func TestCloseFilesKeepsShownFile(t *testing.T) {
	r := t.TempDir()
	var ps []string
	for _, n := range []string{"a", "b", "c", "d"} {
		ps = append(ps, writeFile(t, r, n+".txt", n+"\n"))
	}
	ws := NewWorkspaces(NewShared(), nil)
	defer ws.CloseAll()
	ws.Open(r).Do(func(c *Core) {
		for _, p := range ps {
			c.OpenFile(p)
		}
		if len(c.Tabs) != 4 {
			t.Fatalf("open files = %v", c.OpenPaths())
		}
		c.Tabs[1].Buf.Dirty = true // b
		c.Tabs[3].Buf.Dirty = true // d
		c.Active = 1
		c.CloseFiles()
		if !slices.Equal(c.OpenPaths(), []string{ps[1], ps[3]}) {
			t.Fatalf("open files = %v", c.OpenPaths())
		}
		if got := c.Cur().Path(); got != ps[1] {
			t.Errorf("shown %s, want %s", got, ps[1])
		}
	})
}
