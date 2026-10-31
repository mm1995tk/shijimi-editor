package ui

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
	ps := writeFiles(t, a.root, "b.txt", "c.txt")
	if !a.openTransient(ps[0]) || !a.openTransient(ps[1]) {
		t.Fatal("openTransient failed")
	}
	if len(a.tabs) != 2 || a.active != 1 || a.tabs[1].path() != ps[1] || !a.tabs[1].transient {
		t.Fatalf("tabs = %d active = %d", len(a.tabs), a.active)
	}
	if a.tabs[0].transient {
		t.Fatal("a tab opened normally became transient")
	}
}

// A replaced transient tab keeps its place among the tabs.
func TestTransientTabKeepsPosition(t *testing.T) {
	a := newTestApp(t, "one\n")
	ps := writeFiles(t, a.root, "b.txt", "c.txt", "d.txt")
	a.openTransient(ps[0])
	a.openFile(ps[1])
	a.openTransient(ps[2])
	if len(a.tabs) != 3 || a.tabs[1].path() != ps[2] || a.active != 1 {
		t.Fatalf("tabs = %d tab1 = %s active = %d", len(a.tabs), a.tabs[1].path(), a.active)
	}
}

func TestTransientTabKeptByEditOrOpen(t *testing.T) {
	a := newTestApp(t, "one\n")
	a.helix = false
	ps := writeFiles(t, a.root, "b.txt", "c.txt", "d.txt")

	a.openTransient(ps[0])
	a.focus = focusEditor
	a.Update(Key{Code: 'x', Text: "x"})
	if a.tabs[1].transient {
		t.Fatal("an edit did not keep the tab")
	}

	a.openTransient(ps[1])
	a.openFile(ps[1])
	if a.tabs[2].transient {
		t.Fatal("opening normally did not keep the tab")
	}

	// a file already open normally stays as it is
	a.openTransient(ps[0])
	if a.active != 1 || a.tabs[1].transient {
		t.Fatalf("active = %d transient = %v", a.active, a.tabs[1].transient)
	}
	a.openTransient(ps[2])
	if len(a.tabs) != 4 {
		t.Fatalf("tabs = %d", len(a.tabs))
	}
}

func TestExplorerKeysTransient(t *testing.T) {
	a := newTestApp(t, "one\n")
	ps := writeFiles(t, a.root, "b.txt")
	a.explorer.refresh()
	a.explorer.reveal(ps[0])
	a.focus = focusExplorer
	keys(a, "l")
	if len(a.tabs) != 2 || !a.tabs[1].transient || a.focus != focusExplorer {
		t.Fatalf("l: tabs = %d focus = %v", len(a.tabs), a.focus)
	}
	keys(a, "enter")
	if len(a.tabs) != 2 || a.tabs[1].transient || a.focus != focusEditor {
		t.Fatalf("enter: tabs = %d focus = %v", len(a.tabs), a.focus)
	}
}

func TestExplorerClickTransientDoubleKeeps(t *testing.T) {
	a := newTestApp(t, "one\n")
	ps := writeFiles(t, a.root, "b.txt")
	a.explorer.refresh()
	a.explorer.reveal(ps[0])
	l := a.layout()
	y := l.top + 1 + a.explorer.sel - a.explorer.scroll
	click := func() {
		a.handleMouse(Mouse{X: 2, Y: y, Action: mousePress, Button: MouseLeft})
		a.handleMouse(Mouse{X: 2, Y: y, Action: mouseRelease, Button: MouseLeft})
	}
	click()
	if len(a.tabs) != 2 || !a.tabs[1].transient || a.focus != focusExplorer {
		t.Fatalf("click: tabs = %d focus = %v", len(a.tabs), a.focus)
	}
	click()
	if len(a.tabs) != 2 || a.tabs[1].transient || a.focus != focusEditor {
		t.Fatalf("double click: tabs = %d focus = %v", len(a.tabs), a.focus)
	}
}

func TestTransientTabKeptOnDiskChange(t *testing.T) {
	a := newTestApp(t, "one\n")
	ps := writeFiles(t, a.root, "b.txt")

	a.openTransient(ps[0])
	if err := os.WriteFile(ps[0], []byte("changed outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Minute)
	os.Chtimes(ps[0], future, future)
	if !a.checkDiskChanges() {
		t.Fatal("the change on disk was not picked up")
	}
	a.keepTransients()
	if !a.tabs[1].transient {
		t.Fatal("a change on disk made the tab normal")
	}
}
