package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/diff"

	tea "charm.land/bubbletea/v2"
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
	a.applyChanges(loadChanges(a.gd.gen, a.root, baseHead, nil).(changesLoadedMsg))
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
	want := []dispRow{{0, 0, false, hunkNone}, {1, -1, true, hunkDel}, {2, 1, false, hunkNone}, {3, 2, false, hunkNone}, {-1, 3, true, hunkAdd}, {4, 4, false, hunkNone}, {5, 5, false, hunkNone}}
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
	if len(dv.rows) != 7 || dv.rows[1] != (dispRow{1, -1, true, hunkDel}) || dv.rows[4] != (dispRow{-1, 3, true, hunkAdd}) {
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
		view := ansi.Strip(a.render())
		for _, s := range []string{"a.txt", "比較元 HEAD", "DIFF", "CHANGES", "new.txt"} {
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
	if got, want := a.explorerH(), (a.h-3)*7/10; got != want {
		t.Errorf("explorerH = %d, want %d", got, want)
	}
	a.gd.on = false
	if got := a.explorerH(); got != a.h-1 {
		t.Errorf("explorerH off = %d", got)
	}
}

func openNewFile(t *testing.T, a *App) *diffView {
	t.Helper()
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
	return dv
}

func TestOpenNewFileAgainstHeadIsOnePane(t *testing.T) {
	a, _ := gitDiffApp(t)
	dv := openNewFile(t, a)
	for i := range dv.changed {
		if dv.changed[i] {
			t.Errorf("line %d of the new file is marked", i)
		}
	}
	if g := a.dvGeom(a.layout()); g.leftW != 0 || g.rightX != a.layout().rightX {
		t.Errorf("new file is split: %+v", g)
	}
	for _, k := range dv.barCells(len(dv.rows)) {
		if k != hunkAdd {
			t.Errorf("bar cell %v, want hunkAdd", k)
		}
	}
}

func TestSwitchingBaseRebuildsSingleView(t *testing.T) {
	a, _ := gitDiffApp(t)
	if dv := openNewFile(t, a); !dv.single {
		t.Fatal("not single against HEAD")
	}
	a.gd.base = basePR
	if dv := a.diffFor(a.curText()); dv.single || !dv.changed[0] {
		t.Errorf("view kept single after switching to PR: single %v", dv.single)
	}
	a.gd.base = baseHead
	if dv := a.diffFor(a.curText()); !dv.single {
		t.Error("view not single after switching back to HEAD")
	}
}

func TestOpenNewFileAgainstPRShowsAllLinesAdded(t *testing.T) {
	a, _ := gitDiffApp(t)
	a.gd.base = basePR
	dv := openNewFile(t, a)
	for i := 0; i < 2; i++ {
		if !dv.changed[i] {
			t.Errorf("line %d of the new file is not marked added", i)
		}
	}
	if g := a.dvGeom(a.layout()); g.leftW == 0 {
		t.Error("new file against a PR is not split")
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
	a.applyChanges(loadChanges(a.gd.gen, a.root, baseHead, nil).(changesLoadedMsg))
	a.handleChangesKey(textKey("s"))
	if a.gd.split {
		t.Errorf("s did not switch to unified")
	}
	a.handleChangesKey(textKey("j"))
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
		a.handleMouse(mouseEvent{X: 2, Y: y, Action: mousePress, Button: tea.MouseLeft})
	}
	h := 1 + a.explorerH() // below the header and the file tree
	click(h + 2)           // second file
	if a.curText() == nil || filepath.Base(a.curText().buf.Path) != "new.txt" {
		t.Errorf("second row opened %v", a.curText().buf.Path)
	}
	click(h + 1) // first file
	if filepath.Base(a.curText().buf.Path) != "a.txt" {
		t.Errorf("first row opened %v", a.curText().buf.Path)
	}
	a.setFocus(focusExplorer)
	a.handleExplorerKey(tea.KeyPressMsg{Code: tea.KeyTab})
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
	a.applyChanges(loadChanges(a.gd.gen, a.root, baseHead, nil).(changesLoadedMsg))

	tb := a.curText()
	tb.preview = previewFull
	if a.diffFile(tb) == nil || a.fullPreview(tb) {
		t.Fatalf("diffFile %v fullPreview %v", a.diffFile(tb), a.fullPreview(tb))
	}
	ver := tb.buf.Version
	for _, k := range []string{"i", "Z"} {
		a.handleHelixKey(textKey(k))
	}
	if tb.buf.Version == ver {
		t.Fatal("editing was blocked while the diff is visible")
	}
}

func TestChangeTreeJoinsSingleChildFolders(t *testing.T) {
	var files []*diff.File
	for _, p := range []string{"internal/diff/inline.go", "README.md", "cmd/shj/main.go", "internal/config/config.go", "internal/ui/a.go", "internal/ui/b/c.go"} {
		files = append(files, &diff.File{NewPath: p})
	}
	ordered, rows := changeTree(files)
	var got []string
	for _, r := range rows {
		s := strings.Repeat("  ", r.depth) + r.name
		if r.file < 0 {
			s += "/"
		} else if ordered[r.file].Path() == "" {
			t.Errorf("row %q has no file", r.name)
		}
		got = append(got, s)
	}
	want := []string{
		"cmd/shj/", "  main.go",
		"internal/", "  config/", "    config.go", "  diff/", "    inline.go", "  ui/", "    b/", "      c.go", "    a.go",
		"README.md",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("rows:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	// files are in the order of the rows, so j/k follows the tree
	var paths []string
	for _, f := range ordered {
		paths = append(paths, f.Path())
	}
	if want := "cmd/shj/main.go,internal/config/config.go,internal/diff/inline.go,internal/ui/b/c.go,internal/ui/a.go,README.md"; strings.Join(paths, ",") != want {
		t.Errorf("order = %v", paths)
	}
}

func TestChangesWheelKeepsSelectionOnScreen(t *testing.T) {
	g := &gitDiff{}
	var files []*diff.File
	for i := 0; i < 10; i++ {
		files = append(files, &diff.File{NewPath: fmt.Sprintf("d%d/f.go", i)})
	}
	g.files, g.rows = changeTree(files) // folder, file, folder, file, ...
	g.scroll = 6
	g.keepSelVisible(3)
	if r := g.selRow(); r < 6 || r > 8 {
		t.Errorf("scrolled down: row %d", r)
	}
	g.selectRow(g.rowOf(9))
	g.scroll = 0
	g.keepSelVisible(3)
	if r := g.selRow(); r > 2 {
		t.Errorf("scrolled up: row %d", r)
	}
}

func TestOpenDiffSwitchesAndCloses(t *testing.T) {
	a, _ := gitDiffApp(t)
	// Space G with HEAD shown closes the mode
	a.openDiff(baseHead)
	if a.gd.on {
		t.Fatal("Space G did not close")
	}
	a.openDiff(baseHead)
	if !a.gd.on || a.gd.base != baseHead {
		t.Fatal("Space G did not open")
	}
	// Space g looks up the pull request first; the mode is unchanged until then
	if cmd := a.openDiff(basePR); cmd == nil || a.gd.base != baseHead {
		t.Fatalf("Space g: cmd=%v base=%s", cmd, a.gd.base)
	}
	pr := &prInfo{Number: 12, Title: "差分モードを 1 つにする", State: "OPEN", BaseRefName: "main", HeadRefName: a.branch}
	pr.Author.Login = "someone"
	a.handlePRFound(prFoundMsg{seq: a.gd.prSeq, pr: pr})
	if a.gd.base != basePR || a.gd.pr != pr {
		t.Fatalf("base=%s pr=%v", a.gd.base, a.gd.pr)
	}
	view := ansi.Strip(a.render())
	for _, s := range []string{"PR #12", "差分モードを 1 つにする", "OPEN", "@someone", "比較元 main（PR の base）↔ 作業ツリー", "Space G:HEAD と比べる  Space g:閉じる"} {
		if !strings.Contains(view, s) {
			t.Errorf("view lacks %q", s)
		}
	}
	// Space G switches back to HEAD, Space g with the PR shown closes
	a.openDiff(baseHead)
	if !a.gd.on || a.gd.base != baseHead || a.gd.pr != nil {
		t.Errorf("Space G did not switch to HEAD: on=%v base=%s", a.gd.on, a.gd.base)
	}
	a.handlePRFound(prFoundMsg{seq: a.gd.prSeq, pr: pr})
	a.openDiff(basePR)
	if a.gd.on {
		t.Error("Space g did not close")
	}
}

func TestBranchPRsPicksThePR(t *testing.T) {
	a, _ := gitDiffApp(t)
	// none: the list of every pull request is asked for
	if cmd := a.handleBranchPRs(branchPRsMsg{seq: a.gd.prSeq}); cmd == nil || a.gd.base != baseHead || a.pal.open {
		t.Errorf("no PR for the branch: cmd=%v base=%s", cmd, a.gd.base)
	}
	// one: it is looked up at once, without a list
	seq := a.gd.prSeq
	if cmd := a.handleBranchPRs(branchPRsMsg{seq: seq, prs: []prSummary{{Number: 4}}}); cmd == nil || a.pal.open || a.gd.prSeq == seq {
		t.Errorf("one PR: cmd=%v pal=%v", cmd, a.pal.open)
	}
	// several: a list of just those
	a.handleBranchPRs(branchPRsMsg{seq: a.gd.prSeq, prs: []prSummary{{Number: 4}, {Number: 5}}})
	if !a.pal.open || len(a.pal.pickAll) != 2 {
		t.Errorf("two PRs: open=%v items=%d", a.pal.open, len(a.pal.pickAll))
	}
	a.pal.close()
	// an answer for an older lookup is dropped
	if cmd := a.handleBranchPRs(branchPRsMsg{seq: a.gd.prSeq - 1, prs: []prSummary{{Number: 4}}}); cmd != nil {
		t.Error("stale answer used")
	}
}

func TestPRLookupFails(t *testing.T) {
	a, _ := gitDiffApp(t)
	if cmd := a.handlePRFound(prFoundMsg{seq: a.gd.prSeq, num: 5, err: errString("not found")}); cmd != nil || !a.msgErr {
		t.Errorf(":pr 5 failing: cmd=%v msgErr=%v", cmd, a.msgErr)
	}
}

func TestChangesTitleShowsBase(t *testing.T) {
	a, _ := gitDiffApp(t)
	a.setFocus(focusChanges)
	if got := ansi.Strip(strings.Join(a.renderChanges(30, 5, true), "\n")); !strings.Contains(got, "CHANGES (2)") || !strings.Contains(got, "DIFF") {
		t.Errorf("HEAD title: %q", got)
	}
	a.gd.base = basePR
	if got := ansi.Strip(a.renderChanges(30, 5, true)[0]); !strings.HasSuffix(strings.TrimSpace(got), "PR") {
		t.Errorf("PR title: %q", got)
	}
}

func TestDiffModeLayout(t *testing.T) {
	a, _ := gitDiffApp(t)
	rows := strings.Split(ansi.Strip(a.render()), "\n")
	if len(rows) != a.h {
		t.Fatalf("%d rows for height %d", len(rows), a.h)
	}
	if !strings.Contains(rows[0], "DIFF") || !strings.Contains(rows[a.h-2], "Space G:閉じる") {
		t.Errorf("header %q / help %q", rows[0], rows[a.h-2])
	}
	l := a.layout()
	if l.top != 1 || l.bodyH != a.h-3 || l.editY != 2 {
		t.Errorf("layout %+v", l)
	}
	// the tab bar moved below the header
	a.handleMouse(mouseEvent{X: l.rightX + 1, Y: 1, Action: mousePress, Button: tea.MouseLeft})
	if a.focus != focusEditor {
		t.Errorf("tab bar click: focus=%v", a.focus)
	}
	a.gd.on = false
	if l := a.layout(); l.top != 0 || l.editY != 1 || l.bodyH != a.h-1 {
		t.Errorf("layout off %+v", l)
	}
}

func TestBarRange(t *testing.T) {
	// rows that fit: one per cell
	if lo, hi := barRange(2, 10, 5); lo != 2 || hi != 3 {
		t.Errorf("fit: %d %d", lo, hi)
	}
	if lo, hi := barRange(7, 10, 5); lo != 5 || hi != 5 {
		t.Errorf("past the end: %d %d", lo, hi)
	}
	// 100 rows on 10 cells: 10 each, covering every row once
	next := 0
	for i := 0; i < 10; i++ {
		lo, hi := barRange(i, 10, 100)
		if lo != next || hi-lo != 10 {
			t.Errorf("cell %d: %d %d", i, lo, hi)
		}
		next = hi
	}
}

func TestChangeBarKinds(t *testing.T) {
	dv := &diffView{rows: []dispRow{
		{hunk: hunkNone}, {hunk: hunkAdd}, {hunk: hunkNone}, {hunk: hunkDel},
		{hunk: hunkMod}, {hunk: hunkNone}, {hunk: hunkAdd}, {hunk: hunkDel},
	}}
	if got := dv.barCells(8); got[1] != hunkAdd || got[3] != hunkDel || got[4] != hunkMod || got[0] != hunkNone {
		t.Errorf("1:1 = %v", got)
	}
	// 2 rows per cell: an add and a delete together count as a rewrite
	if got := dv.barCells(4); got[0] != hunkAdd || got[1] != hunkDel || got[2] != hunkMod || got[3] != hunkMod {
		t.Errorf("2:1 = %v", got)
	}
}

func TestDiffRowsHunkKinds(t *testing.T) {
	a, file := gitDiffApp(t)
	os.WriteFile(file, []byte("a\nB\nc\nd\nx\ne\n"), 0o644) // b rewritten, x added
	a.checkDiskChanges()
	tb := a.curText()
	tb.buf.Lines[1] = []rune("B")
	tb.buf.Version++
	dv := a.diffFor(tb)
	if dv.rows[1].hunk != hunkMod || dv.newIdx[4] < 0 || dv.rows[dv.newIdx[4]].hunk != hunkAdd {
		t.Errorf("rows = %v", dv.rows)
	}
}

func TestChangeBarClickScrolls(t *testing.T) {
	a, file := gitDiffApp(t)
	var sb strings.Builder
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&sb, "line %d\n", i)
	}
	os.WriteFile(file, []byte(sb.String()), 0o644)
	gitIn(t, a.root, "commit", "-qam", "long")
	tb := a.curText()
	tb.buf.Lines = nil
	for i := 0; i < 200; i++ {
		tb.buf.Lines = append(tb.buf.Lines, []rune(fmt.Sprintf("line %d", i)))
	}
	tb.buf.Lines[150] = []rune("changed")
	tb.buf.Version++
	a.applyChanges(loadChanges(a.gd.gen, a.root, baseHead, nil).(changesLoadedMsg))
	a.gd.byAbs[file] = &diff.File{NewPath: "a.txt", Status: "modified"}
	tb.dv = nil
	l := a.layout()
	g := a.dvGeom(l)
	dv := a.diffFor(tb)
	view := ansi.Strip(a.render())
	if !strings.Contains(view, "▐") {
		t.Errorf("no mark on the bar")
	}
	a.handleMouse(mouseEvent{X: g.barX, Y: g.top + g.h - 1, Action: mousePress, Button: tea.MouseLeft})
	if s := dv.start(tb.buf.ScrollRow); s < len(dv.rows)-g.h-1 {
		t.Errorf("click at the bottom of the bar scrolled to row %d of %d", s, len(dv.rows))
	}
	if tb.buf.Cur.Row != 0 {
		t.Errorf("the bar moved the cursor to %d", tb.buf.Cur.Row)
	}
}

func TestInlineSpansFollowEdits(t *testing.T) {
	a, file := gitDiffApp(t)
	os.WriteFile(file, []byte("alpha beta\nc\n"), 0o644)
	gitIn(t, a.root, "commit", "-qam", "two")
	a.applyChanges(loadChanges(a.gd.gen, a.root, baseHead, nil).(changesLoadedMsg))
	tb := a.curText()
	tb.buf.Lines = [][]rune{[]rune("alpha gamma"), []rune("c")}
	tb.buf.Version++
	a.gd.byAbs[file] = &diff.File{NewPath: "a.txt", Status: "modified"}
	tb.dv = nil
	dv := a.diffFor(tb)
	if got := dv.newSpans[0]; len(got) != 1 || got[0] != (diff.Span{Start: 6, End: 10}) {
		t.Errorf("new spans = %v", got)
	}
	if got := dv.oldSpans[0]; len(got) != 1 || got[0] != (diff.Span{Start: 6, End: 9}) {
		t.Errorf("old spans = %v", got)
	}
	// an edit recomputes the spans
	tb.buf.Lines[0] = []rune("alpha betaX")
	tb.buf.Version++
	dv = a.diffFor(tb)
	if got := dv.newSpans[0]; len(got) != 1 || got[0] != (diff.Span{Start: 10, End: 11}) {
		t.Errorf("after edit: new spans = %v", got)
	}
	if _, ok := dv.oldSpans[0]; ok {
		t.Errorf("after edit: old spans = %v (insertion only)", dv.oldSpans[0])
	}
	// an unrelated line is marked as a whole
	tb.buf.Lines[0] = []rune("zzzzzzzzzzzzzz")
	tb.buf.Version++
	dv = a.diffFor(tb)
	if len(dv.newSpans) != 0 || len(dv.oldSpans) != 0 {
		t.Errorf("unrelated: %v %v", dv.oldSpans, dv.newSpans)
	}
}

func TestInlineMarksAreDrawn(t *testing.T) {
	text := highlightText("abcdef", nil, cDelBg, []diff.Span{{Start: 2, End: 4}}, cDelWordBg, 10)
	if ansi.Strip(text) != "abcdef    " {
		t.Errorf("text = %q", ansi.Strip(text))
	}
	if strings.Count(text, "\x1b[") < 3 {
		t.Errorf("marked part is not styled apart: %q", text)
	}
	plain := highlightText("abcdef", nil, cDelBg, nil, cDelWordBg, 10)
	if plain == text {
		t.Errorf("marks did not change the output")
	}
	if got := ansi.Strip(highlightText("\tx", nil, cDelBg, nil, "", 6)); got != "    x " {
		t.Errorf("tab = %q", got)
	}
}

func TestInlineBudgetBoundsOneBuild(t *testing.T) {
	a, file := gitDiffApp(t)
	tb := a.curText()
	var oldLines []string
	tb.buf.Lines = nil
	for i := 0; i < 5000; i++ {
		s := fmt.Sprintf("%03d %s", i, strings.Repeat("abcdefghij", 10))
		oldLines = append(oldLines, s)
		tb.buf.Lines = append(tb.buf.Lines, []rune(s+"X"))
	}
	tb.buf.Version++
	a.gd.byAbs[file] = &diff.File{NewPath: "a.txt", Status: "modified"}
	tb.dv = &diffView{rev: a.gd.rev, old: oldLines}
	dv := a.diffFor(tb)
	if n := len(dv.newSpans); n == 0 || n >= 5000 {
		t.Errorf("first build compared %d of 5000 pairs", n)
	}
	// later builds go on from the pairs already compared
	tb.buf.Version++
	dv = a.diffFor(tb)
	if n := len(dv.newSpans); n <= 0 {
		t.Errorf("second build: %d", n)
	}
}

func TestStalePRLookupIsDropped(t *testing.T) {
	a, _ := gitDiffApp(t)
	cmd := a.findPR(0)
	if cmd == nil {
		t.Fatal("no lookup")
	}
	seq := a.gd.prSeq
	a.closeDiff() // Space G before gh answers
	a.handlePRFound(prFoundMsg{seq: seq, pr: &prInfo{Number: 3, BaseRefName: "main"}})
	if a.gd.on || a.gd.base == basePR {
		t.Errorf("a lookup answered after closing reopened the mode: on=%v base=%s", a.gd.on, a.gd.base)
	}
}

func TestGitDiffCommandComparesWithHead(t *testing.T) {
	a, _ := gitDiffApp(t)
	a.showDiff(basePR, &prInfo{Number: 3, BaseRefName: "main"})
	a.gitDiffCommand("")
	if !a.gd.on || a.gd.base != baseHead {
		t.Errorf(":gitdiff with the PR base shown: on=%v base=%s", a.gd.on, a.gd.base)
	}
	a.showDiff(basePR, &prInfo{Number: 3, BaseRefName: "main"})
	a.gitDiffCommand("on")
	if a.gd.base != baseHead {
		t.Errorf(":gitdiff on kept base %s", a.gd.base)
	}
	a.gitDiffCommand("")
	if a.gd.on {
		t.Error(":gitdiff with HEAD shown did not close")
	}
}

func TestBarClickOutsideTheBarRows(t *testing.T) {
	a, _ := gitDiffApp(t)
	tb := a.curText()
	dv := a.diffFor(tb)
	g := a.dvGeom(a.layout())
	tb.buf.ScrollRow = 2
	a.diffBarClick(tb, dv, g, g.top-1)
	a.diffBarClick(tb, dv, g, g.top+g.h)
	if tb.buf.ScrollRow != 2 {
		t.Errorf("a click outside the bar scrolled to %d", tb.buf.ScrollRow)
	}
}

func TestReloadWithoutGhStillUpdatesList(t *testing.T) {
	a, _ := gitDiffApp(t)
	a.showDiff(basePR, &prInfo{Number: 3, BaseRefName: "main"})
	a.setFocus(focusChanges)
	a.handleChangesKey(textKey("r"))
	gen := a.gd.gen
	if cmd := a.handlePRFound(prFoundMsg{seq: a.gd.prSeq, num: 3, refresh: true, err: errString("gh: offline")}); cmd == nil || a.gd.gen == gen || !a.msgErr {
		t.Errorf("r without gh: cmd=%v gen %d→%d msgErr=%v", cmd, gen, a.gd.gen, a.msgErr)
	}
}

func TestChangesFolderCollapse(t *testing.T) {
	g := &gitDiff{}
	var files []*diff.File
	for _, p := range []string{"a/x.go", "a/y.go", "b/z.go"} {
		files = append(files, &diff.File{NewPath: p})
	}
	g.files, g.rows = changeTree(files) // a, x, y, b, z
	g.selectRow(g.rowOf(1))             // a/y.go
	g.setFolder(g.folderRow("a"), false)
	if g.selDir != "a" || len(g.vis()) != 3 {
		t.Fatalf("selDir = %q, vis = %v", g.selDir, g.vis())
	}
	g.move(1) // skips the files of the closed folder
	if g.selDir != "b" {
		t.Errorf("j from a: selDir = %q, sel = %d", g.selDir, g.sel)
	}
	g.move(1)
	if g.selDir != "" || g.file().Path() != "b/z.go" {
		t.Errorf("j from b: selDir = %q", g.selDir)
	}
	g.setFolder(g.folderRow("a"), true)
	if len(g.vis()) != 5 || g.selDir != "" {
		t.Errorf("open: vis = %v, selDir = %q", g.vis(), g.selDir)
	}
}

func TestChangesReloadKeepsFolderState(t *testing.T) {
	a, _ := gitDiffApp(t)
	g := &a.gd
	load := func(paths ...string) {
		var files []*diff.File
		for _, p := range paths {
			files = append(files, &diff.File{NewPath: p})
		}
		g.gen++
		a.applyChanges(changesLoadedMsg{gen: g.gen, base: g.base, rev: g.rev, files: files})
	}
	load("a/x.go", "b/y.go", "c/z.go")
	g.selectRow(g.rowOf(1)) // b/y.go
	g.setFolder(g.folderRow("c"), false)
	g.selectRow(g.folderRow("c"))
	load("a/x.go", "b/y.go", "c/z.go")
	if g.selDir != "c" || !g.closed["c"] {
		t.Errorf("after reload: selDir = %q, closed = %v", g.selDir, g.closed)
	}
	// the selected file goes away and the next one is in the closed folder
	g.selectRow(g.rowOf(1))
	load("a/x.go", "c/z.go")
	if vis := g.vis(); visPos(vis, g.selRow()) < 0 || vis[visPos(vis, g.selRow())] != g.selRow() {
		t.Errorf("selection hidden: row %d, vis %v", g.selRow(), vis)
	}
}

func TestReloadChangesKeepsEmptyListFromFlickering(t *testing.T) {
	a, _ := gitDiffApp(t)
	a.gd.files, a.gd.rows = nil, nil
	a.gd.loading = false
	view := func() string { return strings.Join(a.renderChanges(40, 6, true), "\n") }
	before := view()
	if !strings.Contains(before, "変更はありません") {
		t.Fatalf("before = %q", before)
	}
	a.reloadChanges()
	if got := view(); got != before || strings.Contains(got, "読み込み中") {
		t.Errorf("reload changed the view: %q", got)
	}
	a.showDiff(baseHead, nil)
	if !strings.Contains(view(), "読み込み中") {
		t.Errorf("showDiff should show loading: %q", view())
	}
}

func TestRefreshPRKeepsEmptyListFromFlickering(t *testing.T) {
	a, _ := gitDiffApp(t)
	pr := &prInfo{Number: 3, BaseRefName: "main"}
	a.gd.base, a.gd.pr = basePR, pr
	a.gd.files, a.gd.rows = nil, nil
	a.gd.loading = false
	a.gd.prSeq++
	if cmd := a.handlePRFound(prFoundMsg{seq: a.gd.prSeq, num: 3, refresh: true, pr: pr}); cmd == nil {
		t.Fatal("no command")
	}
	got := strings.Join(a.renderChanges(40, 6, true), "\n")
	if strings.Contains(got, "読み込み中") || !strings.Contains(got, "変更はありません") {
		t.Errorf("view = %q", got)
	}
}
