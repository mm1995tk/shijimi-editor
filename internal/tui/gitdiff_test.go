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

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/diff"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

func TestRenderDiffTextShowsWholeFile(t *testing.T) {
	a, _ := gitDiffApp(t)
	for _, split := range []bool{true, false} {
		a.Gd.Split = split
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
	a.Gd.On = false
	if got := a.ExplorerH(); got != a.h-1 {
		t.Errorf("explorerH off = %d", got)
	}
}

func TestOpenNewFileAgainstHeadIsOnePane(t *testing.T) {
	a, _ := gitDiffApp(t)
	dv := openNewFile(t, a)
	for i := range dv.Changed {
		if dv.Changed[i] {
			t.Errorf("line %d of the new file is marked", i)
		}
	}
	if g := a.dvGeom(a.layout()); g.LeftW != 0 || g.RightX != a.layout().rightX {
		t.Errorf("new file is split: %+v", g)
	}
	for _, k := range core.BarKinds(len(dv.Rows), len(dv.Rows), dv.Hunks) {
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
	split := a.Gd.Split
	l := a.layout()
	a.HandleMouse(a.locate(l.rightX+2, l.editY, core.Mouse{Action: core.MousePress, Button: core.MouseLeft}))
	if a.Gd.Split != split || !dv.Single {
		t.Error("clicking the header of a one-pane file switched split")
	}
	a.Gd.Base = core.BasePR
	if strings.Contains(head(), "分割 ⇄ 統合") {
		t.Errorf("one pane against a PR shows the split hint: %q", head())
	}
}

func TestOpenNewFileAgainstPRIsOnePane(t *testing.T) {
	a, _ := gitDiffApp(t)
	a.Gd.Base = core.BasePR
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
		a.HandleMouse(a.locate(2, y, core.Mouse{Action: core.MousePress, Button: core.MouseLeft}))
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
	a.SetFocus(core.FocusExplorer)
	a.HandleExplorerKey(core.Key{Code: KeyTab})
	if a.Focus != core.FocusChanges {
		t.Errorf("Tab from the tree: focus=%v", a.Focus)
	}
	a.ToggleSide()
	if a.Focus == core.FocusChanges {
		t.Errorf("focus stays on the hidden list")
	}
}

func TestSpaceEFocusesChanges(t *testing.T) {
	a, _ := gitDiffApp(t)
	a.SetFocus(core.FocusEditor)
	a.showSide = false
	a.SpaceMode("E")
	if a.Focus != core.FocusChanges || !a.showSide {
		t.Errorf("Space E in diff mode: focus=%v showSide=%v", a.Focus, a.showSide)
	}
	a.SpaceMode("e")
	if a.Focus != core.FocusExplorer {
		t.Errorf("Space e in diff mode: focus=%v", a.Focus)
	}
	a.ToggleGitDiff()
	a.SpaceMode("E")
	if a.Focus != core.FocusExplorer {
		t.Errorf("Space E outside diff mode: focus=%v", a.Focus)
	}
}

func TestOpenDiffSwitchesAndCloses(t *testing.T) {
	a, _ := gitDiffApp(t)
	// Space G with HEAD shown closes the mode
	a.OpenDiff(core.BaseHead)
	if a.Gd.On {
		t.Fatal("Space G did not close")
	}
	a.OpenDiff(core.BaseHead)
	if !a.Gd.On || a.Gd.Base != core.BaseHead {
		t.Fatal("Space G did not open")
	}
	// Space g looks up the pull request first; the mode is unchanged until then
	if cmd := a.OpenDiff(core.BasePR); cmd == nil || a.Gd.Base != core.BaseHead {
		t.Fatalf("Space g: cmd=%v base=%s", cmd, a.Gd.Base)
	}
	pr := &core.PrInfo{Number: 12, Title: "差分モードを 1 つにする", State: "OPEN", BaseRefName: "main", HeadRefName: a.Branch}
	pr.Author.Login = "someone"
	a.HandlePRFound(core.PrFoundMsg{Seq: a.Gd.PrSeq, Pr: pr})
	if a.Gd.Base != core.BasePR || a.Gd.Pr != pr {
		t.Fatalf("base=%s pr=%v", a.Gd.Base, a.Gd.Pr)
	}
	view := ansi.Strip(a.render())
	for _, s := range []string{"PR #12", "差分モードを 1 つにする", "OPEN", "@someone", "比較元 main（PR の base）↔ 作業ツリー", "Space G:HEAD と比べる  Space g:閉じる"} {
		if !strings.Contains(view, s) {
			t.Errorf("view lacks %q", s)
		}
	}
	// Space G switches back to HEAD, Space g with the PR shown closes
	a.OpenDiff(core.BaseHead)
	if !a.Gd.On || a.Gd.Base != core.BaseHead || a.Gd.Pr != nil {
		t.Errorf("Space G did not switch to HEAD: on=%v base=%s", a.Gd.On, a.Gd.Base)
	}
	a.HandlePRFound(core.PrFoundMsg{Seq: a.Gd.PrSeq, Pr: pr})
	a.OpenDiff(core.BasePR)
	if a.Gd.On {
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
	a.Gd.On = false
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
	gitIn(t, a.Root, "commit", "-qam", "long")
	tb := a.CurText()
	tb.Buf.Lines = nil
	for i := range 200 {
		tb.Buf.Lines = append(tb.Buf.Lines, []rune(fmt.Sprintf("line %d", i)))
	}
	tb.Buf.Lines[150] = []rune("changed")
	tb.Buf.Version++
	a.ApplyChanges(core.LoadChanges(a.Gd.Gen, a.Root, core.BaseHead, nil).(core.ChangesLoadedMsg))
	a.Gd.ByAbs[file] = &diff.File{NewPath: "a.txt", Status: "modified"}
	tb.Dv = nil
	l := a.layout()
	g := a.dvGeom(l)
	dv := a.DiffFor(tb)
	view := ansi.Strip(a.render())
	if !strings.Contains(view, "▐") {
		t.Errorf("no mark on the bar")
	}
	a.HandleMouse(a.locate(g.BarX, g.Top+g.H-1, core.Mouse{Action: core.MousePress, Button: core.MouseLeft}))
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

	if got := a.resizeHit(l, 1, l.top+a.ExplorerH()); got != core.ResizeExplorer {
		t.Fatalf("hit = %v", got)
	}
	a.Focus = core.FocusEditor
	a.HandleMouse(a.locate(1, l.top+a.ExplorerH(), core.Mouse{Action: core.MousePress, Button: core.MouseLeft}))
	if a.Focus != core.FocusChanges || a.resizing != core.ResizeExplorer {
		t.Fatalf("press: focus %v resizing %v", a.Focus, a.resizing)
	}
	a.dragResize(l, 1, l.top+body/2)
	if got := a.ExplorerH(); got != body/2 {
		t.Fatalf("drag to half: %d of %d", got, body)
	}

	a.dragResize(l, 1, l.top)
	if got := a.ExplorerH(); got != core.MinSideSectionH {
		t.Fatalf("tree min: %d", got)
	}
	a.dragResize(l, 1, l.top+body)
	if got := body - a.ExplorerH(); got != core.MinSideSectionH {
		t.Fatalf("changes min: %d", got)
	}

	a.dragResize(l, 1, l.top+body/2)
	before := float64(a.ExplorerH()) / float64(body)
	a.h += 20
	if got := float64(a.ExplorerH()) / float64(a.BodyH()); got < before-0.05 || got > before+0.05 {
		t.Fatalf("ratio %v -> %v", before, got)
	}

	a.resizing = core.ResizeNone
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
	a := New(dir, "")
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	a.Gd.On = true
	a.Gd.Base = core.BaseHead
	a.ApplyChanges(core.LoadChanges(a.Gd.Gen, a.Root, core.BaseHead, nil).(core.ChangesLoadedMsg))
	open := func(name string) *core.Tab {
		t.Helper()
		f := a.Gd.ByAbs[filepath.Join(dir, name)]
		if f == nil {
			t.Fatalf("%s is not in the changes", name)
		}
		a.OpenChange(f)
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
	a.Gd.Split = true
	dv := a.DiffFor(tb)
	g := a.dvGeom(a.layout())
	panes := a.BuildDiffPanes(tb, dv, g, a.EditorOpts(tb, true).PaneOpts)
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

	a.Gd.Split = false
	dv = a.DiffFor(tb)
	panes = a.BuildDiffPanes(tb, dv, a.dvGeom(a.layout()), a.EditorOpts(tb, true).PaneOpts)
	if len(panes) != 1 || panes[0].Diff.Side != screen.SideUnified {
		t.Fatalf("unified diff: %+v", panes)
	}
	if l := panes[0].Lines[1]; !isDeletedLine(l) || l.No != 1 || l.Spans[0].Text != "b" {
		t.Errorf("deleted line = %+v", l)
	}
}

// renderChanges fits the list to h rows and draws it, the way View does.
func (a *App) renderChanges(w, h int, focused bool) []string {
	a.FitChanges(h - 1)
	return drawChanges(a.ChangesScreen(w, h), w, h, focused)
}

func TestChangesTitleShowsBase(t *testing.T) {
	a, _ := gitDiffApp(t)
	a.SetFocus(core.FocusChanges)
	if got := ansi.Strip(strings.Join(a.renderChanges(30, 5, true), "\n")); !strings.Contains(got, "CHANGES (2)") || !strings.Contains(got, "DIFF") {
		t.Errorf("HEAD title: %q", got)
	}
	a.Gd.Base = core.BasePR
	if got := ansi.Strip(a.renderChanges(30, 5, true)[0]); !strings.HasSuffix(strings.TrimSpace(got), "PR") {
		t.Errorf("PR title: %q", got)
	}
}

func TestReloadChangesKeepsEmptyListFromFlickering(t *testing.T) {
	a, _ := gitDiffApp(t)
	a.Gd.Files, a.Gd.Rows = nil, nil
	a.Gd.Loading = false
	view := func() string { return strings.Join(a.renderChanges(40, 6, true), "\n") }
	before := view()
	if !strings.Contains(before, "変更はありません") {
		t.Fatalf("before = %q", before)
	}
	a.ReloadChanges()
	if got := view(); got != before || strings.Contains(got, "読み込み中") {
		t.Errorf("reload changed the view: %q", got)
	}
	a.ShowDiff(core.BaseHead, nil)
	if !strings.Contains(view(), "読み込み中") {
		t.Errorf("showDiff should show loading: %q", view())
	}
}

func TestRefreshPRKeepsEmptyListFromFlickering(t *testing.T) {
	a, _ := gitDiffApp(t)
	pr := &core.PrInfo{Number: 3, BaseRefName: "main"}
	a.Gd.Base, a.Gd.Pr = core.BasePR, pr
	a.Gd.Files, a.Gd.Rows = nil, nil
	a.Gd.Loading = false
	a.Gd.PrSeq++
	if cmd := a.HandlePRFound(core.PrFoundMsg{Seq: a.Gd.PrSeq, Num: 3, Refresh: true, Pr: pr}); cmd == nil {
		t.Fatal("no command")
	}
	got := strings.Join(a.renderChanges(40, 6, true), "\n")
	if strings.Contains(got, "読み込み中") || !strings.Contains(got, "変更はありません") {
		t.Errorf("view = %q", got)
	}
}

// Drawing reads the diff view fit made: it keeps none itself (#284).
func TestDrawingLeavesDiffView(t *testing.T) {
	a, _ := gitDiffApp(t)
	tb := a.CurText()
	tb.Dv = nil
	before := a.renderFrameText()
	if tb.Dv != nil {
		t.Fatal("drawing kept a diff view")
	}
	a.fit()
	dv := tb.Dv
	if dv == nil {
		t.Fatal("fit made no diff view")
	}
	if after := a.renderFrameText(); after != before {
		t.Errorf("the frame changed with the kept view:\n%s\n---\n%s", before, after)
	}
	if tb.Dv != dv {
		t.Error("drawing replaced the diff view")
	}
}
