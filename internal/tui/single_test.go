package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func newSingleApp(t *testing.T, name, content string) (*App, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if content != "" {
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a, err := NewSingle(p)
	if err != nil {
		t.Fatal(err)
	}
	a.w, a.h = 100, 30
	return a, p
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestSingleOpensOnlyThatFile(t *testing.T) {
	a, p := newSingleApp(t, "a.txt", "hello\n")
	if len(a.tabs) != 1 || a.tabs[0].path() != p {
		t.Fatalf("tabs = %d", len(a.tabs))
	}
	if a.showSide || a.showAgent || a.focus != focusEditor || a.gitTop != "" {
		t.Fatalf("side=%v agent=%v focus=%v", a.showSide, a.showAgent, a.focus)
	}
	if a.tabs[0].lang != nil {
		t.Fatal("language server attached")
	}
	if l := a.layout(); l.sideW != 0 || l.agentW != 0 {
		t.Fatalf("layout = %+v", l)
	}
	a.render()
}

func TestSingleBlocksOtherViews(t *testing.T) {
	ctrl := func(r rune) Key { return Key{Code: r, Mod: ModCtrl} }
	keys := []Key{ctrl('p'), ctrl('b'), ctrl('`'), ctrl('j'), ctrl('g'), ctrl('e'), ctrl('n'), ctrl('k'),
		{Code: 'i', Mod: ModAlt}, {Code: KeyF1}}
	for _, helix := range []bool{true, false} {
		a, _ := newSingleApp(t, "a.txt", "hello\n")
		a.helix = helix
		for _, k := range keys {
			a.handleKey(k)
			if a.pal.open || a.showSide || a.showTerm || a.gd.on || a.showAgent || len(a.tabs) != 1 || a.focus != focusEditor {
				t.Fatalf("helix=%v %s opened something", helix, k)
			}
		}
		if len(a.commands) != 0 {
			t.Fatal("palette commands are available")
		}
	}
	a, _ := newSingleApp(t, "a.txt", "hello\n")
	for _, c := range []string{"tree", "sh", "gd", "grep x", "agent", "e other.txt", "new"} {
		a.execCommand(c)
		if a.pal.open || a.showSide || a.showTerm || a.gd.on || a.showAgent || a.search.open || len(a.tabs) != 1 {
			t.Fatalf(":%s opened something", c)
		}
	}
}

func TestSingleQuitAndClose(t *testing.T) {
	a, _ := newSingleApp(t, "a.txt", "hello\n")
	if !isQuit(a.execCommand("q")) {
		t.Fatal(":q did not quit")
	}
	a, _ = newSingleApp(t, "a.txt", "hello\n")
	if !isQuit(a.closeTab(false)) {
		t.Fatal("closing the file did not quit")
	}
	a, _ = newSingleApp(t, "a.txt", "hello\n")
	a.tabs[0].buf.Insert("x")
	if isQuit(a.closeTab(false)) {
		t.Fatal("closing a modified file quit without asking")
	}
	if isQuit(a.execCommand("q")) {
		t.Fatal(":q quit with unsaved changes")
	}
}

func TestSingleNewFileCreatedOnSave(t *testing.T) {
	a, p := newSingleApp(t, "new.txt", "")
	if len(a.tabs) != 1 || a.tabs[0].buf.Text() != "" {
		t.Fatal("not an empty buffer")
	}
	if _, err := os.Stat(p); err == nil {
		t.Fatal("file created before saving")
	}
	a.tabs[0].buf.Insert("hi")
	if !isQuit(a.execCommand("wq")) {
		t.Fatal(":wq did not quit")
	}
	got, err := os.ReadFile(p)
	if err != nil || string(got) != "hi\n" && string(got) != "hi" {
		t.Fatalf("file = %q, %v", got, err)
	}
}
