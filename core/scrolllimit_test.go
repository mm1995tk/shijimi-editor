package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// How far the editor may scroll is decided in one place (maxScrollRow), and
// every way of scrolling goes through it. This test runs each of those ways
// in the plain view and in the diff view over the same file, and checks that
// each stops where the last row is on the bottom row of the screen. A way of
// scrolling that is added later and forgets the limit in one of the views
// fails here: add it to scrollRoutes.

// scrollLimitApp is a repository with a committed 100-line file whose working
// copy has lines deleted and changed near the end (the first change is there),
// so that the diff view has more rows than the file has lines. With diff set
// it is in the diff mode; otherwise the same file is shown plain.
func scrollLimitApp(t *testing.T, diff bool) (*App, *Tab) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git がありません")
	}
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	var lines []string
	for i := range 100 {
		lines = append(lines, fmt.Sprint("line ", i))
	}
	file := filepath.Join(dir, "a.txt")
	write := func(ls []string) {
		if err := os.WriteFile(file, []byte(strings.Join(ls, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(lines)
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-q", "-m", "init")
	mod := slices.Clone(lines)
	mod[97] = "changed"
	mod = slices.Delete(mod, 90, 94) // four rows only the base side has
	write(mod)

	a, f := newTestCore(t, dir, file, 100, 30)
	a.Helix = true
	if diff {
		a.Gd.On = true
		a.Gd.Base = BaseHead
		a.ApplyChanges(LoadChanges(a.Gd.Gen, a.Root, BaseHead, nil).(ChangesLoadedMsg))
	}
	f.push() // the sizes of the editor in the diff mode
	tb := a.CurText()
	if (a.DiffFor(tb) != nil) != diff {
		t.Fatalf("diff view = %v, want %v", a.DiffFor(tb) != nil, diff)
	}
	return a, tb
}

// lastRowOnBottom reports whether the scroll s of tb is the first at which
// the last row of the view is on screen, which is the lowest the view may go.
func lastRowOnBottom(a *App, tb *Tab, s int) (ok bool, detail string) {
	rows, h, start := len(tb.Buf.Lines), a.EditRows(), func(s int) int { return s }
	if dv := a.DiffFor(tb); dv != nil {
		rows, h, start = len(dv.Rows), diffRows(a.EditRows()), dv.Start
	}
	shown := func(s int) bool { return start(s)+h >= rows }
	ok = shown(s) && (s == 0 || !shown(s-1))
	return ok, fmt.Sprintf("scroll %d: %d rows in a view of %d, which starts at row %d", s, rows, h, start(s))
}

// scrollRoutes are the ways of scrolling. Each tries to go as far down as it
// can. only is "" for a route of both views, or the view it exists in.
var scrollRoutes = []struct {
	name string
	only string // "", "plain" or "diff"
	run  func(t *testing.T, a *App, tb *Tab)
}{
	{"wheel", "", func(t *testing.T, a *App, tb *Tab) { a.WheelEditor(10000) }},
	{"cursor", "", func(t *testing.T, a *App, tb *Tab) {
		tb.Buf.SetCursor(len(tb.Buf.Lines)-1, 0, false)
		a.Follow = true
		a.FitCursor(tb, a.TextCols(), a.EditRows())
	}},
	{"half page", "", func(t *testing.T, a *App, tb *Tab) {
		for range 40 {
			a.Update(ctrlD)
		}
	}},
	{"half page, view only", "", func(t *testing.T, a *App, tb *Tab) {
		for range 40 {
			a.Update(ctrlAltD)
		}
	}},
	{"held half page", "", func(t *testing.T, a *App, tb *Tab) {
		a.Update(ReleasesReported{On: true})
		a.Update(ctrlD)
		for range 1000 {
			a.Update(HoldScrollTick{Seq: a.HoldSeq})
		}
	}},
	{"held half page, view only", "", func(t *testing.T, a *App, tb *Tab) {
		a.Update(ReleasesReported{On: true})
		a.Update(ctrlAltD)
		for range 1000 {
			a.Update(HoldScrollTick{Seq: a.HoldSeq})
		}
	}},
	{"open a change", "diff", func(t *testing.T, a *App, tb *Tab) {
		a.showFirstChange(tb)
		a.FitCursor(tb, a.TextCols(), a.EditRows())
	}},
	{"scroll bar", "", func(t *testing.T, a *App, tb *Tab) {
		if dv := a.DiffFor(tb); dv != nil {
			g := DiffGeom{Top: 2, H: diffRows(a.EditRows())}
			a.DiffBarClick(tb, dv, g, g.Top+g.H-1)
			return
		}
		a.TextBarClick(tb, a.EditRows(), a.EditRows()-1)
	}},
}

func TestScrollStopsWithLastRowOnBottom(t *testing.T) {
	for _, mode := range []string{"plain", "diff"} {
		for _, r := range scrollRoutes {
			if r.only != "" && r.only != mode {
				continue
			}
			t.Run(mode+"/"+r.name, func(t *testing.T) {
				a, tb := scrollLimitApp(t, mode == "diff")
				r.run(t, a, tb)
				if ok, detail := lastRowOnBottom(a, tb, tb.Buf.ScrollRow); !ok {
					t.Errorf("not stopped with the last row on the bottom: %s", detail)
				}
			})
		}
	}
}

// A route that goes only to one view is a leak waiting to happen: the plain
// view must have every route the diff view has, except those of the diff view
// itself (the first change, the change bar).
func TestScrollRoutesCoverBothViews(t *testing.T) {
	for _, r := range scrollRoutes {
		if r.only == "plain" {
			t.Errorf("%s: a route of the plain view only; give it to the diff view too", r.name)
		}
	}
}
