package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mm1995tk/shijimi-editor/core/ctl"
)

// sidesApp is prApp with work not committed yet: a.txt has its first line
// changed in the working tree (HEAD still has "linea") and u.txt is a file git
// does not track. The diff mode is the pull request's base <- working tree.
func sidesApp(t *testing.T) (*App, string) {
	t.Helper()
	a, file := prApp(t)
	orig, _ := os.ReadFile(file)
	os.WriteFile(file, []byte("local"+strings.TrimPrefix(string(orig), "linea")), 0o644)
	os.WriteFile(filepath.Join(a.Root, "u.txt"), []byte("u\n"), 0o644)
	if err := a.curTab().reload(); err != nil {
		t.Fatal(err)
	}
	loadSides(a)
	return a, file
}

// loadSides reads the list of the sides diff mode has now, as the answer of
// ReloadChanges would, and ends the round (which gives the tabs their HEAD view).
func loadSides(a *App) {
	a.Apply(func() []Effect {
		a.ApplyChanges(LoadSides(a.Gd.Gen, a.Root, a.Gd.Base, a.Gd.Right, a.Gd.Pr).(ChangesLoadedMsg))
		return nil
	})
}

func changedPaths(a *App) string {
	var got []string
	for _, f := range a.Gd.Files {
		got = append(got, f.Path())
	}
	return strings.Join(got, ",")
}

// Space x goes round PR base <- working tree, PR base <- HEAD and HEAD <-
// working tree, and each pair lists its own changes.
func TestCycleSides(t *testing.T) {
	a, _ := sidesApp(t)
	if a.Sides() != (Sides{BasePR, RightWork}) || changedPaths(a) != "a.txt,b.txt,u.txt" {
		t.Fatalf("PR <- work: sides=%v files=%s", a.Sides(), changedPaths(a))
	}
	if a.Cur().ReadOnlyShown() || a.Cur().Buf.Lines[0] == nil || string(a.Cur().Buf.Lines[0]) != "local" {
		t.Fatalf("PR <- work shows %q (read-only %v)", string(a.Cur().Buf.Lines[0]), a.Cur().ReadOnlyShown())
	}

	keys(a, "space", "x")
	if a.Sides() != (Sides{BasePR, RightHead}) || !a.Gd.Loading {
		t.Fatalf("Space x: sides=%v loading=%v", a.Sides(), a.Gd.Loading)
	}
	loadSides(a)
	// what is not committed is not in the list: no u.txt
	if changedPaths(a) != "a.txt,b.txt" {
		t.Errorf("PR <- HEAD: files=%s", changedPaths(a))
	}
	h := a.Cur()
	if !h.ReadOnlyShown() || string(h.Buf.Lines[0]) != "linea" || h == a.curTab() {
		t.Fatalf("PR <- HEAD shows %q (read-only %v)", string(h.Buf.Lines[0]), h.ReadOnlyShown())
	}
	if string(a.curTab().Buf.Lines[0]) != "local" {
		t.Error("the file's own buffer changed")
	}
	dv := a.DiffFor(h)
	if dv == nil || dv.Rows[dv.NewIdx[9]].changed != true || dv.Rows[dv.NewIdx[0]].changed {
		t.Error("the diff of HEAD does not mark line 10 alone")
	}

	// HEAD <- working tree
	keys(a, "space", "x")
	if a.Sides() != (Sides{BaseHead, RightWork}) {
		t.Fatalf("Space x: sides=%v", a.Sides())
	}
	loadSides(a)
	if changedPaths(a) != "a.txt,u.txt" || a.Cur().ReadOnlyShown() || a.curTab().head != nil {
		t.Errorf("HEAD <- work: files=%s read-only=%v", changedPaths(a), a.Cur().ReadOnlyShown())
	}

	// back to the pull request: the branch's pull request is looked up first
	if cmd := a.CycleSides(); cmd == nil || a.Sides() != (Sides{BaseHead, RightWork}) {
		t.Fatalf("Space x from HEAD: cmd=%v sides=%v", cmd, a.Sides())
	}
	a.handleBranchPRs(branchPRsMsg{seq: a.Gd.PrSeq, prs: []prSummary{{Number: 7}}, quiet: true})
	pr := &PrInfo{ID: "PR_1", Number: 7, State: "OPEN", BaseRefName: "main", HeadRefName: "feature", HeadRefOid: gitOut(t, a.Root, "rev-parse", "HEAD")}
	a.HandlePRFound(PrFoundMsg{Seq: a.Gd.PrSeq, Num: 7, Pr: pr})
	if a.Sides() != (Sides{BasePR, RightWork}) || a.Gd.Pr != pr {
		t.Errorf("after the lookup: sides=%v pr=%v", a.Sides(), a.Gd.Pr)
	}
}

// Without a pull request of the branch, Space x stays at HEAD <- working tree
// and says why.
func TestCycleSidesWithoutPR(t *testing.T) {
	a, _ := gitDiffApp(t)
	a.CycleSides()
	if a.handleBranchPRs(branchPRsMsg{seq: a.Gd.PrSeq, quiet: true}) != nil {
		t.Error("the list of every pull request was asked for")
	}
	if a.Sides() != (Sides{BaseHead, RightWork}) || !strings.Contains(tt(a.Msg), "gd.nobranchprhead") {
		t.Errorf("sides=%v msg=%q", a.Sides(), tt(a.Msg))
	}
}

// The right side HEAD cannot be edited or saved; the file stays as it is.
func TestHeadSideIsReadOnly(t *testing.T) {
	a, file := sidesApp(t)
	a.Focus = FocusEditor
	keys(a, "space", "x")
	loadSides(a)
	h := a.Cur()
	keys(a, "i", "z", "esc", "d")
	if string(h.Buf.Lines[0]) != "linea" || h.Buf.Dirty {
		t.Errorf("HEAD view edited: %q dirty=%v", string(h.Buf.Lines[0]), h.Buf.Dirty)
	}
	if !a.MsgErr || !strings.Contains(tt(a.Msg), "gd.readonly") {
		t.Errorf("msg = %q", tt(a.Msg))
	}
	a.Update(Key{Code: 's', Mod: ModCtrl})
	if got, _ := os.ReadFile(file); !strings.HasPrefix(string(got), "local") {
		t.Errorf("the file was written: %q", got)
	}
	// closing closes the file's tab, not the view
	a.CloseTab(true)
	if len(a.Tabs) != 0 || a.Cur() != nil {
		t.Errorf("tabs left: %d", len(a.Tabs))
	}
}

// The GUI is told the right side when it is HEAD.
func TestChangesReportRight(t *testing.T) {
	a, file := sidesApp(t)
	a.SetTreeHidden(true)
	a.SetChangesInGUI(true)
	a.SetChangesPRInGUI(true)
	a.SetLayout(Layout{SideShown: true})
	if d, _ := a.ChangesReport(); d == nil || d.Base != ctl.BasePR || d.Right != "" {
		t.Fatalf("PR <- work: %+v", d)
	}
	a.CycleSides()
	loadSides(a)
	d, _ := a.ChangesReport()
	if d == nil || d.Base != ctl.BasePR || d.Right != ctl.RightHead || d.Selected != file {
		t.Errorf("PR <- HEAD: %+v", d)
	}
}
