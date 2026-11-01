package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeFiles(t *testing.T, dir string, names ...string) []string {
	t.Helper()
	ps := make([]string, len(names))
	for i, n := range names {
		ps[i] = filepath.Join(dir, n)
		if err := os.WriteFile(ps[i], []byte(n+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return ps
}

// The next transient tab takes the place of the previous one.
func TestTransientTabReplacesPrevious(t *testing.T) {
	a := newTestApp(t, "one\n")
	ps := writeFiles(t, a.Root, "b.txt", "c.txt")
	if !a.openTransient(ps[0]) || !a.openTransient(ps[1]) {
		t.Fatal("openTransient failed")
	}
	if len(a.Tabs) != 2 || a.Active != 1 || a.Tabs[1].Path() != ps[1] || !a.Tabs[1].Transient {
		t.Fatalf("tabs = %d active = %d", len(a.Tabs), a.Active)
	}
	if a.Tabs[0].Transient {
		t.Fatal("a tab opened normally became transient")
	}
}

// A replaced transient tab keeps its place among the tabs.
func TestTransientTabKeepsPosition(t *testing.T) {
	a := newTestApp(t, "one\n")
	ps := writeFiles(t, a.Root, "b.txt", "c.txt", "d.txt")
	a.openTransient(ps[0])
	a.OpenFile(ps[1])
	a.openTransient(ps[2])
	if len(a.Tabs) != 3 || a.Tabs[1].Path() != ps[2] || a.Active != 1 {
		t.Fatalf("tabs = %d tab1 = %s active = %d", len(a.Tabs), a.Tabs[1].Path(), a.Active)
	}
}

func TestTransientTabKeptByEditOrOpen(t *testing.T) {
	a := newTestApp(t, "one\n")
	a.Helix = false
	ps := writeFiles(t, a.Root, "b.txt", "c.txt", "d.txt")

	a.openTransient(ps[0])
	a.Focus = FocusEditor
	a.Update(Key{Code: 'x', Text: "x"})
	if a.Tabs[1].Transient {
		t.Fatal("an edit did not keep the tab")
	}

	a.openTransient(ps[1])
	a.OpenFile(ps[1])
	if a.Tabs[2].Transient {
		t.Fatal("opening normally did not keep the tab")
	}

	// a file already open normally stays as it is
	a.openTransient(ps[0])
	if a.Active != 1 || a.Tabs[1].Transient {
		t.Fatalf("active = %d transient = %v", a.Active, a.Tabs[1].Transient)
	}
	a.openTransient(ps[2])
	if len(a.Tabs) != 4 {
		t.Fatalf("tabs = %d", len(a.Tabs))
	}
}

func TestExplorerKeysTransient(t *testing.T) {
	a := newTestApp(t, "one\n")
	ps := writeFiles(t, a.Root, "b.txt")
	a.Explorer.Refresh()
	a.Explorer.Reveal(ps[0])
	a.Focus = FocusExplorer
	keys(a, "l")
	if len(a.Tabs) != 2 || !a.Tabs[1].Transient || a.Focus != FocusExplorer {
		t.Fatalf("l: tabs = %d focus = %v", len(a.Tabs), a.Focus)
	}
	keys(a, "enter")
	if len(a.Tabs) != 2 || a.Tabs[1].Transient || a.Focus != FocusEditor {
		t.Fatalf("enter: tabs = %d focus = %v", len(a.Tabs), a.Focus)
	}
}

func TestTransientTabKeptOnDiskChange(t *testing.T) {
	a := newTestApp(t, "one\n")
	ps := writeFiles(t, a.Root, "b.txt")

	a.openTransient(ps[0])
	if err := os.WriteFile(ps[0], []byte("changed outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Minute)
	os.Chtimes(ps[0], future, future)
	if !a.checkDiskChanges() {
		t.Fatal("the change on disk was not picked up")
	}
	a.KeepTransients()
	if !a.Tabs[1].Transient {
		t.Fatal("a change on disk made the tab normal")
	}
}
