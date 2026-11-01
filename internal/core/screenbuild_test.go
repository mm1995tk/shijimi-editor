package core

import (
	"strings"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

func view(cols, rows int) screen.View {
	return screen.View{
		Files:   screen.Size{Cols: cols, Rows: rows},
		Status:  screen.Size{Cols: cols},
		Cmdline: screen.Size{Cols: cols},
		Overlay: screen.Size{Cols: cols, Rows: rows},
	}
}

// The screen of a text file: one pane of the size the GUI gave, the status
// line, no command line, and the regions only the TUI shows left out.
func TestBuildScreenText(t *testing.T) {
	a := newTestApp(t, "one\ntwo\n")
	a.Update(Key{Code: KeyEscape})
	a.Focus = FocusEditor
	sc := a.BuildScreen(view(40, 10))
	if sc.Focus != screen.RegionFiles || sc.Files == nil || len(sc.Files.Panes) != 1 || sc.Files.Active != 0 {
		t.Fatalf("files: %+v", sc.Files)
	}
	p := sc.Files.Panes[0]
	if p.Kind != screen.KindText || p.Viewport.Rows != 10 || len(p.Lines) != 3 || p.Lines[1].Spans[0].Text != "two" {
		t.Errorf("pane: %+v", p)
	}
	if sc.Status.Mode == "" || sc.Status.Position != "1:1" {
		t.Errorf("status: %+v", sc.Status)
	}
	if sc.Cmdline != nil || sc.Overlay == nil || len(sc.Overlay) != 0 || sc.Explorer != nil || sc.Changes != nil || sc.Agent != nil {
		t.Errorf("other regions: %+v", sc)
	}
	if a.ScreenW() != 40 || a.ScreenH() != 11 {
		t.Errorf("the core's screen is %dx%d", a.ScreenW(), a.ScreenH())
	}

	keys(a, ":")
	sc = a.BuildScreen(view(40, 10))
	if sc.Focus != screen.RegionCmdline || sc.Cmdline == nil || sc.Cmdline.Prefix != ":" {
		t.Errorf("command line: %v %+v", sc.Focus, sc.Cmdline)
	}
}

// The scroll follows the cursor in the rows the GUI has.
func TestBuildScreenFollowsCursor(t *testing.T) {
	a := newTestApp(t, strings.Repeat("line\n", 100))
	a.Update(Key{Code: KeyEscape})
	a.Focus = FocusEditor
	keys(a, "6", "0", "g")
	sc := a.BuildScreen(view(40, 10))
	p := sc.Files.Panes[0]
	if row := a.CurText().Buf.Cur.Row; row < p.Viewport.Top || row >= p.Viewport.Top+10 {
		t.Fatalf("cursor row %d outside rows %d..%d", row, p.Viewport.Top, p.Viewport.Top+10)
	}
	if len(p.Lines) != 10 || p.Lines[0].No != p.Viewport.Top {
		t.Errorf("lines: %d from %d", len(p.Lines), p.Lines[0].No)
	}
}

// A split editor gives two panes side by side; the focused one is active.
func TestBuildScreenSplit(t *testing.T) {
	a, _ := splitApp(t)
	if !a.Split {
		t.Fatal("not split")
	}
	sc := a.BuildScreen(view(81, 10))
	ps := sc.Files.Panes
	if len(ps) != 2 || ps[0].ID == ps[1].ID {
		t.Fatalf("panes: %+v", ps)
	}
	if sc.Files.Panes[sc.Files.Active].ID != a.Cur().Path() {
		t.Errorf("active %d is not the current file", sc.Files.Active)
	}
	w := 0
	for _, p := range ps {
		w += p.Viewport.Cols + GutterWidth(a.Tabs[0].Buf)
	}
	if w != 80 {
		t.Errorf("the panes are %d columns wide with their gutters, want 80", w)
	}
}

// With no file open the files region has the welcome pane.
func TestBuildScreenWelcome(t *testing.T) {
	a := newApp(t.TempDir(), "")
	sc := a.BuildScreen(view(40, 10))
	if len(sc.Files.Panes) != 1 || sc.Files.Panes[0].Kind != screen.KindWelcome {
		t.Fatalf("panes: %+v", sc.Files.Panes)
	}
}

// Beside its preview, the text is the active pane: it has the cursor and takes
// the keys.
func TestBuildScreenPreviewSplitActive(t *testing.T) {
	a := newTestApp(t, "# title\n\nbody\n")
	a.Update(Key{Code: KeyEscape})
	a.Focus = FocusEditor
	a.CurText().Preview = PreviewSplit
	sc := a.BuildScreen(view(81, 10))
	ps := sc.Files.Panes
	if len(ps) != 2 || ps[0].Kind != screen.KindText || ps[1].Kind != screen.KindPreview {
		t.Fatalf("panes: %+v", ps)
	}
	if sc.Files.Active != 0 {
		t.Errorf("active %d, want the text (0)", sc.Files.Active)
	}
	if got := ps[0].Viewport.Cols + GutterWidth(a.CurText().Buf); got != previewSplitW(81) {
		t.Errorf("text is %d columns, want %d", got, previewSplitW(81))
	}
}
