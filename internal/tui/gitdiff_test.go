package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/diff"
	"github.com/mm1995tk/shijimi-editor/internal/screen"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

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
	if got, want := a.ExplorerH(), (a.h-3)*7/10; got != want {
		t.Errorf("explorerH = %d, want %d", got, want)
	}
	a.gd.on = false
	if got := a.ExplorerH(); got != a.h-1 {
		t.Errorf("explorerH off = %d", got)
	}
}

func TestOpenNewFileAgainstHeadIsOnePane(t *testing.T) {
	a, _ := gitDiffApp(t)
	dv := openNewFile(t, a)
	for i := range dv.changed {
		if dv.changed[i] {
			t.Errorf("line %d of the new file is marked", i)
		}
	}
	if g := a.dvGeom(a.layout()); g.LeftW != 0 || g.RightX != a.layout().rightX {
		t.Errorf("new file is split: %+v", g)
	}
	for _, k := range BarKinds(len(dv.Rows), len(dv.Rows), dv.hunks) {
		if k != screen.MarkAdded {
			t.Errorf("bar cell %v, want added", k)
		}
	}
}

func TestOnePaneHidesSplitHintAndIgnoresHeaderClick(t *testing.T) {
	a, _ := gitDiffApp(t)
	dv := openNewFile(t, a)
	head := func() string {
		return ansi.Strip(a.renderDiffText(a.CurText(), a.DiffFor(a.CurText()), a.layout(), false)[0])
	}
	if strings.Contains(head(), "分割 ⇄ 統合") {
		t.Errorf("one pane shows the split hint: %q", head())
	}
	split := a.gd.split
	l := a.layout()
	a.HandleMouse(a.locate(l.rightX+2, l.editY, Mouse{Action: MousePress, Button: MouseLeft}))
	if a.gd.split != split || !dv.Single {
		t.Error("clicking the header of a one-pane file switched split")
	}
	a.gd.base = basePR
	if strings.Contains(head(), "分割 ⇄ 統合") {
		t.Errorf("one pane against a PR shows the split hint: %q", head())
	}
}

func TestOpenNewFileAgainstPRIsOnePane(t *testing.T) {
	a, _ := gitDiffApp(t)
	a.gd.base = basePR
	dv := openNewFile(t, a)
	if !dv.Single {
		t.Error("new file against a PR is not one pane")
	}
	if g := a.dvGeom(a.layout()); g.LeftW != 0 {
		t.Errorf("new file against a PR is split: %+v", g)
	}
}

func TestChangesListClickAndTab(t *testing.T) {
	a, _ := gitDiffApp(t)
	click := func(y int) {
		a.HandleMouse(a.locate(2, y, Mouse{Action: MousePress, Button: MouseLeft}))
	}
	h := 1 + a.ExplorerH() // below the header and the file tree
	click(h + 2)           // second file
	if a.CurText() == nil || filepath.Base(a.CurText().Buf.Path) != "new.txt" {
		t.Errorf("second row opened %v", a.CurText().Buf.Path)
	}
	click(h + 1) // first file
	if filepath.Base(a.CurText().Buf.Path) != "a.txt" {
		t.Errorf("first row opened %v", a.CurText().Buf.Path)
	}
	a.SetFocus(FocusExplorer)
	a.handleExplorerKey(Key{Code: KeyTab})
	if a.focus != FocusChanges {
		t.Errorf("Tab from the tree: focus=%v", a.focus)
	}
	a.toggleSide()
	if a.focus == FocusChanges {
		t.Errorf("focus stays on the hidden list")
	}
}

func TestSpaceEFocusesChanges(t *testing.T) {
	a, _ := gitDiffApp(t)
	a.SetFocus(FocusEditor)
	a.showSide = false
	a.spaceMode("E")
	if a.focus != FocusChanges || !a.showSide {
		t.Errorf("Space E in diff mode: focus=%v showSide=%v", a.focus, a.showSide)
	}
	a.spaceMode("e")
	if a.focus != FocusExplorer {
		t.Errorf("Space e in diff mode: focus=%v", a.focus)
	}
	a.toggleGitDiff()
	a.spaceMode("E")
	if a.focus != FocusExplorer {
		t.Errorf("Space E outside diff mode: focus=%v", a.focus)
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
	pr := &PrInfo{Number: 12, Title: "差分モードを 1 つにする", State: "OPEN", BaseRefName: "main", HeadRefName: a.branch}
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
	a.gd.on = false
	if l := a.layout(); l.top != 0 || l.editY != 1 || l.bodyH != a.h-1 {
		t.Errorf("layout off %+v", l)
	}
}

func TestChangeBarClickScrolls(t *testing.T) {
	a, file := gitDiffApp(t)
	var sb strings.Builder
	for i := range 200 {
		fmt.Fprintf(&sb, "line %d\n", i)
	}
	os.WriteFile(file, []byte(sb.String()), 0o644)
	gitIn(t, a.root, "commit", "-qam", "long")
	tb := a.CurText()
	tb.Buf.Lines = nil
	for i := range 200 {
		tb.Buf.Lines = append(tb.Buf.Lines, []rune(fmt.Sprintf("line %d", i)))
	}
	tb.Buf.Lines[150] = []rune("changed")
	tb.Buf.Version++
	a.applyChanges(loadChanges(a.gd.gen, a.root, baseHead, nil).(changesLoadedMsg))
	a.gd.byAbs[file] = &diff.File{NewPath: "a.txt", Status: "modified"}
	tb.dv = nil
	l := a.layout()
	g := a.dvGeom(l)
	dv := a.DiffFor(tb)
	view := ansi.Strip(a.render())
	if !strings.Contains(view, "▐") {
		t.Errorf("no mark on the bar")
	}
	a.HandleMouse(a.locate(g.BarX, g.Top+g.H-1, Mouse{Action: MousePress, Button: MouseLeft}))
	if s := dv.Start(tb.Buf.ScrollRow); s < len(dv.Rows)-g.H-1 {
		t.Errorf("click at the bottom of the bar scrolled to row %d of %d", s, len(dv.Rows))
	}
	if tb.Buf.Cur.Row != 0 {
		t.Errorf("the bar moved the cursor to %d", tb.Buf.Cur.Row)
	}
}

func TestInlineMarksAreDrawn(t *testing.T) {
	text := highlightText("abcdef", nil, cDelBg, []diff.Span{{Start: 2, End: 4}}, cDelWordBg, 10, 0)
	if ansi.Strip(text) != "abcdef    " {
		t.Errorf("text = %q", ansi.Strip(text))
	}
	if strings.Count(text, "\x1b[") < 3 {
		t.Errorf("marked part is not styled apart: %q", text)
	}
	plain := highlightText("abcdef", nil, cDelBg, nil, cDelWordBg, 10, 0)
	if plain == text {
		t.Errorf("marks did not change the output")
	}
	if got := ansi.Strip(highlightText("\tx", nil, cDelBg, nil, "", 6, 0)); got != "    x " {
		t.Errorf("tab = %q", got)
	}
}

func TestBarClickOutsideTheBarRows(t *testing.T) {
	a, _ := gitDiffApp(t)
	tb := a.CurText()
	dv := a.DiffFor(tb)
	g := a.dvGeom(a.layout())
	tb.Buf.ScrollRow = 2
	a.DiffBarClick(tb, dv, g, g.Top-1)
	a.DiffBarClick(tb, dv, g, g.Top+g.H)
	if tb.Buf.ScrollRow != 2 {
		t.Errorf("a click outside the bar scrolled to %d", tb.Buf.ScrollRow)
	}
}

func TestSidebarSplitDrag(t *testing.T) {
	a, _ := gitDiffApp(t)
	l := a.layout()
	body := a.BodyH()

	if got := a.resizeHit(l, 1, l.top+a.ExplorerH()); got != ResizeExplorer {
		t.Fatalf("hit = %v", got)
	}
	a.focus = FocusEditor
	a.HandleMouse(a.locate(1, l.top+a.ExplorerH(), Mouse{Action: MousePress, Button: MouseLeft}))
	if a.focus != FocusChanges || a.resizing != ResizeExplorer {
		t.Fatalf("press: focus %v resizing %v", a.focus, a.resizing)
	}
	a.dragResize(l, 1, l.top+body/2)
	if got := a.ExplorerH(); got != body/2 {
		t.Fatalf("drag to half: %d of %d", got, body)
	}

	a.dragResize(l, 1, l.top)
	if got := a.ExplorerH(); got != MinSideSectionH {
		t.Fatalf("tree min: %d", got)
	}
	a.dragResize(l, 1, l.top+body)
	if got := body - a.ExplorerH(); got != MinSideSectionH {
		t.Fatalf("changes min: %d", got)
	}

	a.dragResize(l, 1, l.top+body/2)
	before := float64(a.ExplorerH()) / float64(body)
	a.h += 20
	if got := float64(a.ExplorerH()) / float64(a.BodyH()); got < before-0.05 || got > before+0.05 {
		t.Fatalf("ratio %v -> %v", before, got)
	}

	a.resizing = ResizeNone
	a.StartResize()
	press(a, "=")
	if got, want := a.ExplorerH(), a.BodyH()*7/10; got != want {
		t.Fatalf("reset %d, want %d", got, want)
	}
}

func TestOpenChangePutsCursorOnFirstChange(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git がありません")
	}
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	var lines []string
	for i := range 40 {
		lines = append(lines, fmt.Sprint(i))
	}
	write := func(name string, ls []string) {
		os.WriteFile(filepath.Join(dir, name), []byte(strings.Join(ls, "\n")+"\n"), 0o644)
	}
	write("mod.txt", lines)
	write("del.txt", lines)
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-q", "-m", "init")
	mod := slices.Clone(lines)
	mod[30], mod[35] = "x", "y"
	write("mod.txt", mod)
	write("del.txt", slices.Delete(slices.Clone(lines), 20, 22))
	a := newApp(dir, "")
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	a.gd.on = true
	a.gd.base = baseHead
	a.applyChanges(loadChanges(a.gd.gen, a.root, baseHead, nil).(changesLoadedMsg))
	open := func(name string) *Tab {
		t.Helper()
		f := a.gd.byAbs[filepath.Join(dir, name)]
		if f == nil {
			t.Fatalf("%s is not in the changes", name)
		}
		a.openChange(f)
		return a.CurText()
	}

	// the cursor is on the first changed line, a few rows below the top
	tb := open("mod.txt")
	a.render() // the scroll stays where it was put
	if tb.Buf.Cur.Row != 30 || tb.Buf.ScrollRow != 27 {
		t.Errorf("mod.txt: cur=%d scroll=%d", tb.Buf.Cur.Row, tb.Buf.ScrollRow)
	}
	// a change that only deletes puts the cursor on the line after it
	if tb := open("del.txt"); tb.Buf.Cur.Row != 20 {
		t.Errorf("del.txt: cur=%d", tb.Buf.Cur.Row)
	}
	// a file already open keeps its cursor
	tb.Buf.Cur.Row = 5
	if open("mod.txt"); tb.Buf.Cur.Row != 5 {
		t.Errorf("reopened mod.txt: cur=%d", tb.Buf.Cur.Row)
	}
}

func TestAddedLineKeepsGreenUnderCursor(t *testing.T) {
	a, _ := gitDiffApp(t)
	tb := a.CurText()
	dv := a.DiffFor(tb)
	if dv == nil {
		t.Fatal("no diff view")
	}
	// row 3 of the new file is the added line "x"
	tb.Buf.SetCursor(3, 0, false)
	lines := a.renderDiffText(tb, dv, a.layout(), true)
	rgb := hexRGB(cAddBg)
	want := fmt.Sprintf("48;2;%d;%d;%d", rgb[0], rgb[1], rgb[2])
	var line string
	for _, l := range lines {
		if strings.Contains(ansi.Strip(l), "x") {
			line = l
		}
	}
	if line == "" {
		t.Fatal("added line not rendered")
	}
	if !strings.Contains(line, want) {
		t.Errorf("added line under the cursor lost its background: %q", line)
	}
}

func TestDiffPanes(t *testing.T) {
	a, _ := gitDiffApp(t)
	tb := a.CurText()
	a.gd.split = true
	dv := a.DiffFor(tb)
	g := a.dvGeom(a.layout())
	panes := a.BuildDiffPanes(tb, dv, g, a.EditorOpts(tb, true).paneOpts)
	if len(panes) != 2 {
		t.Fatalf("split diff has %d panes", len(panes))
	}
	left, right := panes[0], panes[1]
	if left.Kind != screen.KindDiff || right.Kind != screen.KindDiff ||
		left.Diff.Side != screen.SideOld || right.Diff.Side != screen.SideNew {
		t.Errorf("kinds/sides: %+v %+v", left.Diff, right.Diff)
	}
	if len(left.Lines) != len(dv.Rows) || len(right.Lines) != len(dv.Rows) {
		t.Fatalf("rows: %d %d, want %d", len(left.Lines), len(right.Lines), len(dv.Rows))
	}
	// row 1: b is deleted (nothing on the right); row 4: x is added (nothing on the left)
	if l := left.Lines[1]; l.Blank || l.No != 1 || !isDeletedLine(l) || !right.Lines[1].Blank {
		t.Errorf("deleted row: left %+v right %+v", l, right.Lines[1])
	}
	if r := right.Lines[4]; r.Blank || r.No != 3 || !changedLine(r) || !left.Lines[4].Blank {
		t.Errorf("added row: left %+v right %+v", left.Lines[4], r)
	}
	if l := left.Lines[0]; isDeletedLine(l) || changedLine(right.Lines[0]) {
		t.Errorf("context row is marked: %+v %+v", l, right.Lines[0])
	}
	if want := []screen.Hunk{{1, 2, screen.MarkDeleted}, {4, 5, screen.MarkAdded}}; !reflect.DeepEqual(right.Diff.Hunks, want) {
		t.Errorf("hunks = %v", right.Diff.Hunks)
	}
	if right.Title.Text != "a.txt" || right.Title.Added != 1 || right.Title.Deleted != 1 {
		t.Errorf("title = %+v", right.Title)
	}

	a.gd.split = false
	dv = a.DiffFor(tb)
	panes = a.BuildDiffPanes(tb, dv, a.dvGeom(a.layout()), a.EditorOpts(tb, true).paneOpts)
	if len(panes) != 1 || panes[0].Diff.Side != screen.SideUnified {
		t.Fatalf("unified diff: %+v", panes)
	}
	if l := panes[0].Lines[1]; !isDeletedLine(l) || l.No != 1 || l.Spans[0].Text != "b" {
		t.Errorf("deleted line = %+v", l)
	}
}

// renderChanges fits the list to h rows and draws it, the way View does.
func (a *App) renderChanges(w, h int, focused bool) []string {
	a.fitChanges(h - 1)
	return drawChanges(a.ChangesScreen(w, h), w, h, focused)
}


func TestChangesTitleShowsBase(t *testing.T) {
	a, _ := gitDiffApp(t)
	a.SetFocus(FocusChanges)
	if got := ansi.Strip(strings.Join(a.renderChanges(30, 5, true), "\n")); !strings.Contains(got, "CHANGES (2)") || !strings.Contains(got, "DIFF") {
		t.Errorf("HEAD title: %q", got)
	}
	a.gd.base = basePR
	if got := ansi.Strip(a.renderChanges(30, 5, true)[0]); !strings.HasSuffix(strings.TrimSpace(got), "PR") {
		t.Errorf("PR title: %q", got)
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
	pr := &PrInfo{Number: 3, BaseRefName: "main"}
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
