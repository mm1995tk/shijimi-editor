package core

import (
	"path/filepath"
	"testing"

	"github.com/mm1995tk/shijimi-editor/core/ctl"
)

// Without the GUI showing the list there is nothing to tell.
func TestChangesReportNeedsGUI(t *testing.T) {
	a, _ := gitDiffApp(t)
	if d, loading := a.ChangesReport(); d != nil || loading {
		t.Errorf("report = %v, %v", d, loading)
	}
}

// With the GUI showing the list, the report is the list, and the shj draws none.
func TestChangesReportInGUI(t *testing.T) {
	a, file := gitDiffApp(t)
	a.SetTreeHidden(true)
	a.SetChangesInGUI(true)
	a.SetLayout(Layout{SideShown: true})
	d, loading := a.ChangesReport()
	if loading || d == nil || len(d.Files) != 2 {
		t.Fatalf("report = %+v, %v", d, loading)
	}
	want := ctl.DiffFile{Path: file, Status: "modified", Added: 1, Deleted: 1}
	if d.Files[0] != want {
		t.Errorf("file 0 = %+v, want %+v", d.Files[0], want)
	}
	if d.Files[1].Path != filepath.Join(a.GitTopDir(), "new.txt") || d.Files[1].Status != "added" {
		t.Errorf("file 1 = %+v", d.Files[1])
	}
	if d.Selected != file {
		t.Errorf("selected = %q, want %q", d.Selected, file)
	}
	if a.ChangesListShown() || a.SideVisible() {
		t.Errorf("shown=%v side=%v: the shj must draw no list", a.ChangesListShown(), a.SideVisible())
	}
	// The diff mode against a pull request keeps the shj's own list.
	a.Gd.Base = BasePR
	if d, _ := a.ChangesReport(); d != nil || !a.ChangesListShown() || !a.SideVisible() {
		t.Errorf("pr: report=%v shown=%v side=%v", d, a.ChangesListShown(), a.SideVisible())
	}
	a.Gd.Base = BaseHead
	a.Gd.On = false
	if d, _ := a.ChangesReport(); d != nil {
		t.Errorf("off: report = %v", d)
	}
}

func TestChangesReportWaitsWhileLoading(t *testing.T) {
	a, _ := gitDiffApp(t)
	a.SetChangesInGUI(true)
	a.Gd.Loading = true
	if d, loading := a.ChangesReport(); d != nil || !loading {
		t.Errorf("report = %v, %v", d, loading)
	}
}

// The GUI asks for a file of the list with "diff": it opens, also when deleted.
func TestCtlDiffOpensFileOfList(t *testing.T) {
	a, _ := gitDiffApp(t)
	a.SetChangesInGUI(true)
	other := filepath.Join(a.GitTopDir(), "new.txt")
	if r := ctlCall(a, ctl.Request{Cmd: "diff", Path: other}); !r.OK {
		t.Fatalf("diff: %+v", r)
	}
	if a.CurText() == nil || a.CurText().Buf.Path != other || a.Focus != FocusEditor {
		t.Errorf("cur = %v focus = %v", a.CurText(), a.Focus)
	}
	a.Gd.Files[0].Status = "deleted" // as if a.txt were gone
	if r := ctlCall(a, ctl.Request{Cmd: "diff", Path: filepath.Join(a.GitTopDir(), "a.txt")}); !r.OK {
		t.Errorf("deleted: %+v", r)
	}
}
