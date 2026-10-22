package ui

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
	a := NewSingle(p)
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
	a, _ := newSingleApp(t, "a.txt", "hello\n")
	for _, k := range []string{"ctrl+p", "ctrl+b", "ctrl+`", "ctrl+j", "ctrl+g", "ctrl+e", "ctrl+n", "alt+i"} {
		switch k {
		case "ctrl+p":
			a.handleKey(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
		case "ctrl+b":
			a.handleKey(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
		case "ctrl+`":
			a.handleKey(tea.KeyPressMsg{Code: '`', Mod: tea.ModCtrl})
		case "ctrl+j":
			a.handleKey(tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl})
		case "ctrl+g":
			a.handleKey(tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl})
		case "ctrl+e":
			a.handleKey(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl})
		case "ctrl+n":
			a.handleKey(tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl})
		case "alt+i":
			a.handleKey(tea.KeyPressMsg{Code: 'i', Mod: tea.ModAlt})
		}
		if a.pal.open || a.showSide || a.showTerm || a.gd.on || a.showAgent || len(a.tabs) != 1 || a.focus != focusEditor {
			t.Fatalf("%s opened something", k)
		}
	}
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
