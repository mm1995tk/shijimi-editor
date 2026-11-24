package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/core"
)

// driftDiffApp is gitDiffApp with its branch pushed to a remote of its own and
// one commit made after, so HEAD is ahead of its upstream. The diff mode is
// HEAD <- working tree.
func driftDiffApp(t *testing.T) *App {
	t.Helper()
	a, _ := gitDiffApp(t)
	remote := filepath.Join(t.TempDir(), "remote.git")
	gitIn(t, a.Root, "init", "-q", "--bare", remote)
	gitIn(t, a.Root, "remote", "add", "origin", remote)
	gitIn(t, a.Root, "push", "-q", "-u", "origin", "HEAD")
	os.WriteFile(filepath.Join(a.Root, "c.txt"), []byte("c\n"), 0o644)
	gitIn(t, a.Root, "add", "c.txt")
	gitIn(t, a.Root, "commit", "-q", "-m", "c")
	reloadSides(a)
	return a
}

func reloadSides(a *App) {
	a.Apply(func() []core.Effect {
		a.ApplyChanges(core.LoadSides(a.Gd.Gen, a.Root, a.Gd.Base, a.Gd.Right, a.Gd.Pr).(core.ChangesLoadedMsg))
		return nil
	})
}

func TestHeadDriftGraysTheHeadSide(t *testing.T) {
	a := driftDiffApp(t)
	a.Gd.Split = true
	tb := a.CurText()
	panes := a.BuildDiffPanes(tb, a.DiffFor(tb), a.dvGeom(a.layout()), a.EditorOpts(tb, true).PaneOpts)
	left, right := panes[0], panes[1]
	if !left.Dim || right.Dim {
		t.Fatalf("pane dim: left=%v right=%v", left.Dim, right.Dim)
	}
	for i, l := range left.Lines {
		if l.Dim == l.Blank {
			t.Errorf("left line %d: dim=%v blank=%v", i, l.Dim, l.Blank)
		}
	}
	for i, l := range right.Lines {
		if l.Dim {
			t.Errorf("right line %d is gray", i)
		}
	}

	view := ansi.Strip(a.render())
	if !strings.Contains(view, "HEAD が origin/") || !strings.Contains(view, "1 commit 先・未 push") || !strings.Contains(view, "HEAD の側を灰色にしています") {
		t.Errorf("no band\n%s", view)
	}

	// unified: only what comes from HEAD (the deleted line) is gray
	a.Gd.Split = false
	tb = a.CurText()
	panes = a.BuildDiffPanes(tb, a.DiffFor(tb), a.dvGeom(a.layout()), a.EditorOpts(tb, true).PaneOpts)
	for i, l := range panes[0].Lines {
		if l.Dim != isDeletedLine(l) {
			t.Errorf("unified line %d: dim=%v deleted=%v", i, l.Dim, isDeletedLine(l))
		}
	}

	// the gray rows have no color of their own: no green background is drawn
	a.Gd.Split = true
	raw := a.render()
	if strings.Count(raw, cDelBg) != 0 {
		t.Errorf("the HEAD side still draws the deleted-line background %s", cDelBg)
	}
}

func TestHeadDriftGraysTheRightWhenHeadIsOnTheRight(t *testing.T) {
	a := driftDiffApp(t)
	a.Gd.Base, a.Gd.Right = core.BasePR, core.RightHead
	a.Gd.Pr = &core.PrInfo{Number: 7, BaseRefName: "main", HeadRefName: "elsewhere", HeadRefOid: gitOut(t, a.Root, "rev-parse", "HEAD~1")}
	gitIn(t, a.Root, "branch", "-f", "main", "HEAD~1")
	reloadSides(a)
	a.Gd.Split = true
	a.OpenChange(a.Gd.File())
	tb := a.CurText()
	panes := a.BuildDiffPanes(tb, a.DiffFor(tb), a.dvGeom(a.layout()), a.EditorOpts(tb, true).PaneOpts)
	if len(panes) == 2 && (panes[0].Dim || !panes[1].Dim) {
		t.Errorf("pane dim: left=%v right=%v", panes[0].Dim, panes[1].Dim)
	}
	if len(panes) == 1 && !panes[0].Dim {
		t.Errorf("one pane is not gray")
	}
	if view := ansi.Strip(a.render()); !strings.Contains(view, "HEAD が PR #7 と一致していません") {
		t.Errorf("no band\n%s", view)
	}
}

func TestHeadDriftBandGoesWhenPushed(t *testing.T) {
	a := driftDiffApp(t)
	body := a.BodyH()
	gitIn(t, a.Root, "push", "-q")
	reloadSides(a)
	view := ansi.Strip(a.render())
	if strings.Contains(view, "一致していません") {
		t.Errorf("the band stays after the push\n%s", view)
	}
	if a.BodyH() != body+1 {
		t.Errorf("the body is %d rows, want %d without the band", a.BodyH(), body+1)
	}
}
