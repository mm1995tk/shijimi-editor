package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// gitDiffApp makes a repository with a committed file a.txt (a..e) whose
// working copy has "b" removed and "x" added after "d", plus an untracked file.
func gitDiffApp(t *testing.T) (*App, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git がありません")
	}
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	file := filepath.Join(dir, "a.txt")
	os.WriteFile(file, []byte("a\nb\nc\nd\ne\n"), 0o644)
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-q", "-m", "init")
	os.WriteFile(file, []byte("a\nc\nd\nx\ne\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("n1\nn2\n"), 0o644)
	a := New(dir, file)
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	a.gd.on = true
	a.gd.base = baseHead
	a.applyChanges(loadChanges(a.gd.gen, a.root, baseHead).(changesLoadedMsg))
	return a, file
}

func TestLoadChangesListsModifiedAndUntracked(t *testing.T) {
	a, _ := gitDiffApp(t)
	var got []string
	for _, f := range a.gd.files {
		got = append(got, f.Path()+":"+f.Status)
	}
	if strings.Join(got, ",") != "a.txt:modified,new.txt:added" {
		t.Errorf("files = %v", got)
	}
}

func TestDiffViewRows(t *testing.T) {
	a, _ := gitDiffApp(t)
	tb := a.curText()
	dv := a.diffFor(tb)
	if dv == nil {
		t.Fatal("no diff view for a changed file")
	}
	// split: a | (b -> -) | c d | (- -> x) | e
	want := []dispRow{{0, 0, false}, {1, -1, true}, {2, 1, false}, {3, 2, false}, {-1, 3, true}, {4, 4, false}, {5, 5, false}}
	if len(dv.rows) != len(want) {
		t.Fatalf("split rows = %v", dv.rows)
	}
	for i := range want {
		if dv.rows[i] != want[i] {
			t.Errorf("split row %d = %v, want %v", i, dv.rows[i], want[i])
		}
	}
	a.gd.split = false
	dv = a.diffFor(tb)
	if len(dv.rows) != 7 || dv.rows[1] != (dispRow{1, -1, true}) || dv.rows[4] != (dispRow{-1, 3, true}) {
		t.Errorf("unified rows = %v", dv.rows)
	}
	if !dv.changed[3] || dv.changed[0] {
		t.Errorf("changed = %v", dv.changed)
	}
}

func TestDiffViewFollowsEdits(t *testing.T) {
	a, _ := gitDiffApp(t)
	tb := a.curText()
	if dv := a.diffFor(tb); dv.changed[1] {
		t.Fatalf("line c should be unchanged")
	}
	tb.buf.SetCursor(1, 0, false)
	tb.buf.Insert("Z")
	dv := a.diffFor(tb)
	if !dv.changed[1] {
		t.Errorf("edited line is not marked changed: %v", dv.changed)
	}
}

func TestRenderDiffTextShowsWholeFile(t *testing.T) {
	a, _ := gitDiffApp(t)
	for _, split := range []bool{true, false} {
		a.gd.split = split
		view := ansi.Strip(a.View())
		for _, s := range []string{"a.txt", "比較元: HEAD", "CHANGES", "new.txt"} {
			if !strings.Contains(view, s) {
				t.Errorf("split=%v: view lacks %q", split, s)
			}
		}
		// the removed line appears (on the base side) and so does the added one
		if !strings.Contains(view, "b") || !strings.Contains(view, "x") {
			t.Errorf("split=%v: missing removed/added line", split)
		}
	}
}

func TestSidebarSplit7to3(t *testing.T) {
	a, _ := gitDiffApp(t)
	if got, want := a.explorerH(), (a.h-1)*7/10; got != want {
		t.Errorf("explorerH = %d, want %d", got, want)
	}
	a.gd.on = false
	if got := a.explorerH(); got != a.h-1 {
		t.Errorf("explorerH off = %d", got)
	}
}

func TestOpenNewFileShowsAllLinesAdded(t *testing.T) {
	a, _ := gitDiffApp(t)
	for i, f := range a.gd.files {
		if f.Path() == "new.txt" {
			a.gd.sel = i
		}
	}
	a.openChange(a.gd.file())
	dv := a.diffFor(a.curText())
	if dv == nil {
		t.Fatal("no diff view")
	}
	for i := 0; i < 2; i++ {
		if !dv.changed[i] {
			t.Errorf("line %d of the new file is not marked added", i)
		}
	}
}

func TestToggleGitDiffAndKeys(t *testing.T) {
	a, _ := gitDiffApp(t)
	a.gd.on = false
	a.toggleGitDiff()
	if !a.gd.on {
		t.Fatal("not on")
	}
	a.setFocus(focusChanges)
	a.applyChanges(loadChanges(a.gd.gen, a.root, baseHead).(changesLoadedMsg))
	a.handleChangesKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if a.gd.split {
		t.Errorf("s did not switch to unified")
	}
	a.handleChangesKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if a.gd.sel != 1 {
		t.Errorf("sel = %d", a.gd.sel)
	}
	a.toggleGitDiff()
	if a.gd.on || a.focus == focusChanges {
		t.Errorf("not turned off: on=%v focus=%v", a.gd.on, a.focus)
	}
}

func TestGitDiffCommand(t *testing.T) {
	a, _ := gitDiffApp(t)
	a.gitDiffCommand("unified")
	if a.gd.split {
		t.Error("unified")
	}
	a.gitDiffCommand("split")
	if !a.gd.split {
		t.Error("split")
	}
	a.gitDiffCommand("off")
	if a.gd.on {
		t.Error("off")
	}
	a.gitDiffCommand("bogus")
	if !a.msgErr {
		t.Error("bogus accepted")
	}
}

func TestPRBaseFallsBackToHead(t *testing.T) {
	a, _ := gitDiffApp(t)
	m := changesLoadedMsg{gen: a.gd.gen, base: basePR, err: errString("no pr")}
	a.gd.base = basePR
	a.applyChanges(m)
	if a.gd.base != baseHead || !a.msgErr {
		t.Errorf("base=%s msgErr=%v", a.gd.base, a.msgErr)
	}
}

func TestChangesListClickAndTab(t *testing.T) {
	a, _ := gitDiffApp(t)
	click := func(y int) {
		a.handleMouse(tea.MouseMsg{X: 2, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	}
	h := a.explorerH()
	click(h + 2) // second file
	if a.curText() == nil || filepath.Base(a.curText().buf.Path) != "new.txt" {
		t.Errorf("second row opened %v", a.curText().buf.Path)
	}
	click(h + 1) // first file
	if filepath.Base(a.curText().buf.Path) != "a.txt" {
		t.Errorf("first row opened %v", a.curText().buf.Path)
	}
	a.setFocus(focusExplorer)
	a.handleExplorerKey(tea.KeyMsg{Type: tea.KeyTab})
	if a.focus != focusChanges {
		t.Errorf("Tab from the tree: focus=%v", a.focus)
	}
	a.toggleSide()
	if a.focus == focusChanges {
		t.Errorf("focus stays on the hidden list")
	}
}

func TestFullPreviewDoesNotBlockEditingInGitDiff(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git がありません")
	}
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	file := filepath.Join(dir, "a.md")
	os.WriteFile(file, []byte("a\nb\n"), 0o644)
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-q", "-m", "init")
	os.WriteFile(file, []byte("a\nc\n"), 0o644)
	a := New(dir, file)
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	a.gd.on = true
	a.gd.base = baseHead
	a.applyChanges(loadChanges(a.gd.gen, a.root, baseHead).(changesLoadedMsg))

	tb := a.curText()
	tb.preview = previewFull
	if a.diffFile(tb) == nil || a.fullPreview(tb) {
		t.Fatalf("diffFile %v fullPreview %v", a.diffFile(tb), a.fullPreview(tb))
	}
	ver := tb.buf.Version
	for _, k := range []string{"i", "Z"} {
		a.handleHelixKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
	}
	if tb.buf.Version == ver {
		t.Fatal("editing was blocked while the diff is visible")
	}
}
