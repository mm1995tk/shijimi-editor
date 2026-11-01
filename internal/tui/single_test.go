package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/core"
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

func TestSingleOpensOnlyThatFile(t *testing.T) {
	a, p := newSingleApp(t, "a.txt", "hello\n")
	if len(a.Tabs) != 1 || a.Tabs[0].Path() != p {
		t.Fatalf("tabs = %d", len(a.Tabs))
	}
	if a.showSide || a.showAgent || a.Focus != core.FocusEditor || a.GitTop != "" {
		t.Fatalf("side=%v agent=%v focus=%v", a.showSide, a.showAgent, a.Focus)
	}
	if a.Tabs[0].Lang != nil {
		t.Fatal("language server attached")
	}
	if l := a.layout(); l.sideW != 0 || l.agentW != 0 {
		t.Fatalf("layout = %+v", l)
	}
	a.render()
}

func TestSingleBlocksOtherViews(t *testing.T) {
	ctrl := func(r rune) core.Key { return core.Key{Code: r, Mod: core.ModCtrl} }
	keys := []core.Key{ctrl('p'), ctrl('b'), ctrl('`'), ctrl('j'), ctrl('g'), ctrl('e'), ctrl('n'), ctrl('k'),
		{Code: 'i', Mod: core.ModAlt}, {Code: KeyF1}}
	for _, helix := range []bool{true, false} {
		a, _ := newSingleApp(t, "a.txt", "hello\n")
		a.Helix = helix
		for _, k := range keys {
			a.HandleKey(k)
			if a.Pal.Open || a.showSide || a.showTerm || a.Gd.On || a.showAgent || len(a.Tabs) != 1 || a.Focus != core.FocusEditor {
				t.Fatalf("helix=%v %s opened something", helix, k)
			}
		}
		if len(a.Commands) != 0 {
			t.Fatal("palette commands are available")
		}
	}
	a, _ := newSingleApp(t, "a.txt", "hello\n")
	for _, c := range []string{"tree", "sh", "gd", "grep x", "agent", "e other.txt", "new"} {
		a.ExecCommand(c)
		if a.Pal.Open || a.showSide || a.showTerm || a.Gd.On || a.showAgent || a.Search.Open || len(a.Tabs) != 1 {
			t.Fatalf(":%s opened something", c)
		}
	}
}

func TestSingleQuitAndClose(t *testing.T) {
	a, _ := newSingleApp(t, "a.txt", "hello\n")
	if !isQuit(a.effectCmd(a.ExecCommand("q")...)) {
		t.Fatal(":q did not quit")
	}
	a, _ = newSingleApp(t, "a.txt", "hello\n")
	if !isQuit(a.effectCmd(a.CloseTab(false)...)) {
		t.Fatal("closing the file did not quit")
	}
	a, _ = newSingleApp(t, "a.txt", "hello\n")
	a.Tabs[0].Buf.Insert("x")
	if isQuit(a.effectCmd(a.CloseTab(false)...)) {
		t.Fatal("closing a modified file quit without asking")
	}
	if isQuit(a.effectCmd(a.ExecCommand("q")...)) {
		t.Fatal(":q quit with unsaved changes")
	}
}

func TestSingleNewFileCreatedOnSave(t *testing.T) {
	a, p := newSingleApp(t, "new.txt", "")
	if len(a.Tabs) != 1 || a.Tabs[0].Buf.Text() != "" {
		t.Fatal("not an empty buffer")
	}
	if _, err := os.Stat(p); err == nil {
		t.Fatal("file created before saving")
	}
	a.Tabs[0].Buf.Insert("hi")
	if !isQuit(a.effectCmd(a.ExecCommand("wq")...)) {
		t.Fatal(":wq did not quit")
	}
	got, err := os.ReadFile(p)
	if err != nil || string(got) != "hi\n" && string(got) != "hi" {
		t.Fatalf("file = %q, %v", got, err)
	}
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}
