package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/diff"
	"github.com/mm1995tk/shijimi-editor/internal/screen"

	tea "charm.land/bubbletea/v2"
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
	a := newApp(dir, file)
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
	tb := a.CurText()
	dv := a.DiffFor(tb)
	if dv == nil {
		t.Fatal("no diff view for a changed file")
	}
	// split: a | (b -> -) | c d | (- -> x) | e
	want := []dispRow{{0, 0, false, hunkNone}, {1, -1, true, hunkDel}, {2, 1, false, hunkNone}, {3, 2, false, hunkNone}, {-1, 3, true, hunkAdd}, {4, 4, false, hunkNone}, {5, 5, false, hunkNone}}
	if len(dv.Rows) != len(want) {
		t.Fatalf("split rows = %v", dv.Rows)
	}
	for i := range want {
		if dv.Rows[i] != want[i] {
			t.Errorf("split row %d = %v, want %v", i, dv.Rows[i], want[i])
		}
	}
	a.gd.split = false
	dv = a.DiffFor(tb)
	if len(dv.Rows) != 7 || dv.Rows[1] != (dispRow{1, -1, true, hunkDel}) || dv.Rows[4] != (dispRow{-1, 3, true, hunkAdd}) {
		t.Errorf("unified rows = %v", dv.Rows)
	}
	if !dv.changed[3] || dv.changed[0] {
		t.Errorf("changed = %v", dv.changed)
	}
}

func TestDiffViewFollowsEdits(t *testing.T) {
	a, _ := gitDiffApp(t)
	tb := a.CurText()
	if dv := a.DiffFor(tb); dv.changed[1] {
		t.Fatalf("line c should be unchanged")
	}
	tb.Buf.SetCursor(1, 0, false)
	tb.Buf.Insert("Z")
	dv := a.DiffFor(tb)
	if !dv.changed[1] {
		t.Errorf("edited line is not marked changed: %v", dv.changed)
	}
}

func openNewFile(t *testing.T, a *App) *DiffView {
	t.Helper()
	for i, f := range a.gd.files {
		if f.Path() == "new.txt" {
			a.gd.sel = i
		}
	}
	a.openChange(a.gd.file())
	dv := a.DiffFor(a.CurText())
	if dv == nil {
		t.Fatal("no diff view")
	}
	return dv
}

func TestSwitchingBaseRebuildsSingleView(t *testing.T) {
	a, _ := gitDiffApp(t)
	if dv := openNewFile(t, a); !dv.Single {
		t.Fatal("not single against HEAD")
	}
	a.gd.base = basePR
	a.gd.rev = "other"
	if dv := a.DiffFor(a.CurText()); !dv.Single || dv.rev != "other" {
		t.Errorf("view not rebuilt after switching to PR: %+v", dv)
	}
	a.gd.base = baseHead
	if dv := a.DiffFor(a.CurText()); !dv.Single {
		t.Error("view not single after switching back to HEAD")
	}
}

func TestToggleGitDiffAndKeys(t *testing.T) {
	a, _ := gitDiffApp(t)
	a.gd.on = false
	a.toggleGitDiff()
	if !a.gd.on {
		t.Fatal("not on")
	}
	a.SetFocus(FocusChanges)
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
	if a.gd.on || a.focus == FocusChanges {
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
	a := newApp(dir, file)
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	a.gd.on = true
	a.gd.base = baseHead
	a.applyChanges(loadChanges(a.gd.gen, a.root, baseHead, nil).(changesLoadedMsg))

	tb := a.CurText()
	tb.Preview = PreviewFull
	if a.DiffFile(tb) == nil || a.FullPreview(tb) {
		t.Fatalf("diffFile %v fullPreview %v", a.DiffFile(tb), a.FullPreview(tb))
	}
	ver := tb.Buf.Version
	for _, k := range []string{"i", "Z"} {
		a.handleHelixKey(textKey(k))
	}
	if tb.Buf.Version == ver {
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
	for i := range 10 {
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

func TestBarRange(t *testing.T) {
	// rows that fit: one per cell
	if lo, hi := BarRange(2, 10, 5); lo != 2 || hi != 3 {
		t.Errorf("fit: %d %d", lo, hi)
	}
	if lo, hi := BarRange(7, 10, 5); lo != 5 || hi != 5 {
		t.Errorf("past the end: %d %d", lo, hi)
	}
	// 100 rows on 10 cells: 10 each, covering every row once
	next := 0
	for i := range 10 {
		lo, hi := BarRange(i, 10, 100)
		if lo != next || hi-lo != 10 {
			t.Errorf("cell %d: %d %d", i, lo, hi)
		}
		next = hi
	}
}

func TestChangeBarKinds(t *testing.T) {
	dv := &DiffView{Rows: []dispRow{
		{hunk: hunkNone}, {hunk: hunkAdd}, {hunk: hunkNone}, {hunk: hunkDel},
		{hunk: hunkMod}, {hunk: hunkNone}, {hunk: hunkAdd}, {hunk: hunkDel},
	}}
	if got := BarKinds(len(dv.Rows), 8, hunksOf(dv.Rows)); got[1] != screen.MarkAdded || got[3] != screen.MarkDeleted || got[4] != screen.MarkModified || got[0] != "" {
		t.Errorf("1:1 = %v", got)
	}
	// 2 rows per cell: an add and a delete together count as a rewrite
	if got := BarKinds(len(dv.Rows), 4, hunksOf(dv.Rows)); got[0] != screen.MarkAdded || got[1] != screen.MarkDeleted || got[2] != screen.MarkModified || got[3] != screen.MarkModified {
		t.Errorf("2:1 = %v", got)
	}
}

func TestDiffRowsHunkKinds(t *testing.T) {
	a, file := gitDiffApp(t)
	os.WriteFile(file, []byte("a\nB\nc\nd\nx\ne\n"), 0o644) // b rewritten, x added
	a.checkDiskChanges()
	tb := a.CurText()
	tb.Buf.Lines[1] = []rune("B")
	tb.Buf.Version++
	dv := a.DiffFor(tb)
	if dv.Rows[1].hunk != hunkMod || dv.NewIdx[4] < 0 || dv.Rows[dv.NewIdx[4]].hunk != hunkAdd {
		t.Errorf("rows = %v", dv.Rows)
	}
}

func TestInlineSpansFollowEdits(t *testing.T) {
	a, file := gitDiffApp(t)
	os.WriteFile(file, []byte("alpha beta\nc\n"), 0o644)
	gitIn(t, a.root, "commit", "-qam", "two")
	a.applyChanges(loadChanges(a.gd.gen, a.root, baseHead, nil).(changesLoadedMsg))
	tb := a.CurText()
	tb.Buf.Lines = [][]rune{[]rune("alpha gamma"), []rune("c")}
	tb.Buf.Version++
	a.gd.byAbs[file] = &diff.File{NewPath: "a.txt", Status: "modified"}
	tb.dv = nil
	dv := a.DiffFor(tb)
	if got := dv.newSpans[0]; len(got) != 1 || got[0] != (diff.Span{Start: 6, End: 10}) {
		t.Errorf("new spans = %v", got)
	}
	if got := dv.oldSpans[0]; len(got) != 1 || got[0] != (diff.Span{Start: 6, End: 9}) {
		t.Errorf("old spans = %v", got)
	}
	// an edit recomputes the spans
	tb.Buf.Lines[0] = []rune("alpha betaX")
	tb.Buf.Version++
	dv = a.DiffFor(tb)
	if got := dv.newSpans[0]; len(got) != 1 || got[0] != (diff.Span{Start: 10, End: 11}) {
		t.Errorf("after edit: new spans = %v", got)
	}
	if _, ok := dv.oldSpans[0]; ok {
		t.Errorf("after edit: old spans = %v (insertion only)", dv.oldSpans[0])
	}
	// an unrelated line is marked as a whole
	tb.Buf.Lines[0] = []rune("zzzzzzzzzzzzzz")
	tb.Buf.Version++
	dv = a.DiffFor(tb)
	if len(dv.newSpans) != 0 || len(dv.oldSpans) != 0 {
		t.Errorf("unrelated: %v %v", dv.oldSpans, dv.newSpans)
	}
}

func TestInlineBudgetBoundsOneBuild(t *testing.T) {
	a, file := gitDiffApp(t)
	tb := a.CurText()
	var oldLines []string
	tb.Buf.Lines = nil
	for i := range 5000 {
		s := fmt.Sprintf("%03d %s", i, strings.Repeat("abcdefghij", 10))
		oldLines = append(oldLines, s)
		tb.Buf.Lines = append(tb.Buf.Lines, []rune(s+"X"))
	}
	tb.Buf.Version++
	a.gd.byAbs[file] = &diff.File{NewPath: "a.txt", Status: "modified"}
	tb.dv = &DiffView{rev: a.gd.rev, old: oldLines}
	dv := a.DiffFor(tb)
	if n := len(dv.newSpans); n == 0 || n >= 5000 {
		t.Errorf("first build compared %d of 5000 pairs", n)
	}
	// later builds go on from the pairs already compared
	tb.Buf.Version++
	dv = a.DiffFor(tb)
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
	a.handlePRFound(prFoundMsg{seq: seq, pr: &PrInfo{Number: 3, BaseRefName: "main"}})
	if a.gd.on || a.gd.base == basePR {
		t.Errorf("a lookup answered after closing reopened the mode: on=%v base=%s", a.gd.on, a.gd.base)
	}
}

func TestGitDiffCommandComparesWithHead(t *testing.T) {
	a, _ := gitDiffApp(t)
	a.showDiff(basePR, &PrInfo{Number: 3, BaseRefName: "main"})
	a.gitDiffCommand("")
	if !a.gd.on || a.gd.base != baseHead {
		t.Errorf(":gitdiff with the PR base shown: on=%v base=%s", a.gd.on, a.gd.base)
	}
	a.showDiff(basePR, &PrInfo{Number: 3, BaseRefName: "main"})
	a.gitDiffCommand("on")
	if a.gd.base != baseHead {
		t.Errorf(":gitdiff on kept base %s", a.gd.base)
	}
	a.gitDiffCommand("")
	if a.gd.on {
		t.Error(":gitdiff with HEAD shown did not close")
	}
}

func TestReloadWithoutGhStillUpdatesList(t *testing.T) {
	a, _ := gitDiffApp(t)
	a.showDiff(basePR, &PrInfo{Number: 3, BaseRefName: "main"})
	a.SetFocus(FocusChanges)
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
