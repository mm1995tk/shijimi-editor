package core

import (
	"strings"
	"testing"

	"github.com/mm1995tk/shijimi-editor/core/screen"
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
	if sc.Status.Mode == "" || tt(sc.Status.Position) != "1:1" {
		t.Errorf("status: %+v", sc.Status)
	}
	if sc.Cmdline != nil || sc.Overlay == nil || len(sc.Overlay) != 0 {
		t.Errorf("other regions: %+v", sc)
	}
	if a.ScreenW() != 40 || a.ScreenH() != 11 {
		t.Errorf("the core's screen is %dx%d", a.ScreenW(), a.ScreenH())
	}

	keys(a, ":")
	sc = a.BuildScreen(view(40, 10))
	if sc.Focus != screen.RegionCmdline || sc.Cmdline == nil || tt(sc.Cmdline.Prefix) != ":" {
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
	if got := ps[0].Viewport.Cols + GutterWidth(a.CurText().Buf); got != a.previewSplitW(81) {
		t.Errorf("text is %d columns, want %d", got, a.previewSplitW(81))
	}
}

func TestSplitLeftW(t *testing.T) {
	for _, c := range []struct{ w, pct, want int }{
		{81, 0, 40},   // half when nothing was dragged
		{81, 25, 20},  // a quarter of the 80 columns beside the divider
		{81, 1, 10},   // never narrower than minSplitW
		{81, 99, 70},  // nor the other side
		{15, 70, 7},   // too narrow to keep both sides: half
		{101, -3, 50}, // a negative share is none
	} {
		if got := splitLeftW(c.w, c.pct); got != c.want {
			t.Errorf("splitLeftW(%d, %d) = %d, want %d", c.w, c.pct, got, c.want)
		}
	}
}

// The share the GUI dragged the preview's border to moves the panes.
func TestBuildScreenPreviewPct(t *testing.T) {
	a := newTestApp(t, "# title\n\nbody\n")
	a.Update(Key{Code: KeyEscape})
	a.Focus = FocusEditor
	a.CurText().Preview = PreviewSplit
	v := view(81, 10)
	v.PreviewPct = 25
	ps := a.BuildScreen(v).Files.Panes
	if len(ps) != 2 {
		t.Fatalf("panes: %+v", ps)
	}
	if got := ps[0].Viewport.Cols + GutterWidth(a.CurText().Buf); got != 20 {
		t.Errorf("text is %d columns, want 20", got)
	}
	if got := ps[1].Viewport.Cols; got != 60 {
		t.Errorf("preview is %d columns, want 60", got)
	}
}

// Leaving the resize mode with a key clears it in the core, and the hint with it.
func TestResizeModeLeaves(t *testing.T) {
	a := newTestApp(t, "x\n")
	a.StartResize()
	if !a.InResizeMode() || a.Msg.IsZero() {
		t.Fatalf("resize mode %v, message %v", a.InResizeMode(), a.Msg)
	}
	a.HandleKey(Key{Code: 'h', Text: "h"})
	if !a.InResizeMode() {
		t.Fatal("h left the mode")
	}
	a.HandleKey(Key{Code: KeyEscape})
	if a.InResizeMode() || !a.Msg.IsZero() {
		t.Errorf("after Esc: resize mode %v, message %v", a.InResizeMode(), a.Msg)
	}
}

// The search window is an overlay of the screen: its fields, the matched
// lines (the matches carry a background) and the preview of the selected one.
func TestBuildScreenSearch(t *testing.T) {
	a := newTestApp(t, "alpha\nneedle here\nomega\n")
	a.OpenSearch(FQuery)
	for _, r := range "needle" {
		a.HandleSearchKey(textKey(string(r)))
	}
	a.HandleEvent(SearchTick{seq: a.Search.seq.Load()})
	effs := a.HandleEvent(SearchTick{seq: a.Search.seq.Load()})
	a.HandleEvent(effs[0].(RunSearch).Run())

	sc := a.BuildScreen(view(80, 24))
	if len(sc.Overlay) != 1 || sc.Overlay[0].Kind != screen.OverlaySearch || sc.Overlay[0].Search == nil {
		t.Fatalf("overlays: %+v", sc.Overlay)
	}
	s := sc.Overlay[0].Search
	if s.Fields[FQuery].Text != "needle" || s.Field != FQuery || s.Total != 1 || len(s.Hits) != 1 || s.BodyRows < 1 {
		t.Fatalf("search: %+v", s)
	}
	if h := s.Hits[0]; h.Label != "a.txt:2" || len(h.Line.Spans) != 2 || h.Line.Spans[0].Bg != screen.StyleMatch || h.Line.Spans[0].Text != "needle" {
		t.Errorf("hit: %+v", h)
	}
	if p := s.Preview; p == nil || p.Hit != 1 || len(p.Lines) != 4 || p.Del != nil {
		t.Errorf("preview: %+v", p)
	}

	a.HandleSearchKey(Key{Code: KeyEscape})
	if sc := a.BuildScreen(view(80, 24)); len(sc.Overlay) != 0 {
		t.Errorf("Esc leaves overlays: %+v", sc.Overlay)
	}
}

// A long label of a matched line keeps its end (file name and line number),
// as the TUI does.
func TestSearchHitLabelKeepsEnd(t *testing.T) {
	a := newTestApp(t, "x\n")
	h := &SearchHit{Path: a.Root + "/" + strings.Repeat("dir/", 20) + "file.go", Row: 41, Line: "needle", Spans: [][2]int{{0, 6}}}
	got := a.searchHitRow(h, 40).Label
	if !strings.HasPrefix(got, "…") || !strings.HasSuffix(got, "file.go:42") || len([]rune(got)) > 24 {
		t.Errorf("label = %q", got)
	}
}

// The lines before and after a replacement show a tab as one space, like the
// line above them.
func TestWholeSpansTab(t *testing.T) {
	sp := wholeSpans([]rune("\tfoo"), 0, screen.StyleDiffDelBg)
	if len(sp) != 1 || sp[0].Text != " foo" {
		t.Errorf("spans: %+v", sp)
	}
}

// A change the agent proposes is an overlay of the screen: the title, the rows
// of the diff in view (added and removed rows carry a band), and Enter answers
// the agent.
func TestBuildScreenProposal(t *testing.T) {
	a, f := ideApp(t)
	a.OpenFile(f)
	args := map[string]any{"old_file_path": f, "new_file_path": f, "new_file_contents": "one\n2\nthree\n", "tab_name": "edit"}
	ch, _, _ := ideCallRaw(a, "openDiff", args)

	sc := a.BuildScreen(view(100, 24))
	if len(sc.Overlay) != 1 || sc.Overlay[0].Kind != screen.OverlayProposal {
		t.Fatalf("overlays: %+v", sc.Overlay)
	}
	ov := sc.Overlay[0]
	if ov.View == nil || ov.View.Cols != ProposalWidth(100)-2 || ov.View.Total != len(ov.Lines) || len(ov.Lines) < 4 {
		t.Fatalf("overlay: %+v", ov)
	}
	var add, del bool
	for _, l := range ov.Lines {
		add = add || (l.Band == screen.StyleDiffAddBg && l.Spans[0].Text == " + 2")
		del = del || (l.Band == screen.StyleDiffDelBg && l.Spans[0].Text == " - two")
	}
	if !add || !del {
		t.Errorf("lines: %+v", ov.Lines)
	}

	a.Update(Key{Code: KeyEnter})
	if r := <-ch; r.Content[0].Text != "FILE_SAVED" {
		t.Errorf("reply = %+v", r)
	}
	if sc := a.BuildScreen(view(100, 24)); len(sc.Overlay) != 0 {
		t.Errorf("overlays remain: %+v", sc.Overlay)
	}
}
