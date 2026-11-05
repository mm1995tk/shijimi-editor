package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/core"
	"github.com/mm1995tk/shijimi-editor/core/config"
	"github.com/mm1995tk/shijimi-editor/core/screen"
)

func previewApp(t *testing.T, dir, src string) *App {
	t.Helper()
	b := core.NewBuffer()
	b.Insert(src)
	b.Path = filepath.Join(dir, "x.md")
	a := appOf(&core.Core{Cfg: &config.Config{}, Tabs: []*core.Tab{core.NewTab(b)}})
	a.Tabs[0].Buf.Cur.Row = 0
	a.Tabs[0].Preview = core.PreviewSplit
	return a
}

func TestBuildPreviewPane(t *testing.T) {
	a := previewApp(t, t.TempDir(), "# 見出し\n\n段落に **太字** と `code`。\n\n- 項目\n\n```go\nfunc main() {}\n```\n")
	p := a.BuildPreviewPane(a.Tabs[0], 30, 5)
	if p.Kind != screen.KindPreview || p.Viewport.Cols != 30 || p.Viewport.Rows != 5 || len(p.Lines) != 5 {
		t.Fatalf("pane = %+v", p)
	}
	if p.Preview == nil || p.Preview.Top != 0 || p.Preview.Rows != p.Viewport.Total || p.Preview.Rows <= 5 {
		t.Fatalf("preview = %+v", p.Preview)
	}
	h := p.Lines[0].Spans[0]
	if h.Text != "見出し" || h.Style != screen.StyleMdH1 || !h.Bold {
		t.Fatalf("heading = %+v", h)
	}
	if p.Lines[1].Spans[0].Style != screen.StyleBorder {
		t.Fatalf("rule under the heading = %+v", p.Lines[1])
	}
	var bold, code bool
	for _, l := range p.Lines {
		for _, sp := range l.Spans {
			bold = bold || (sp.Text == "太字" && sp.Bold)
			code = code || (sp.Text == "code" && sp.Style == screen.StyleMdCode && sp.Bg == screen.StyleMdCodeBg)
		}
	}
	if !bold || !code {
		t.Fatalf("inline styles missing (bold %v, code %v): %+v", bold, code, p.Lines)
	}
	for i, l := range p.Lines {
		if l.No != i {
			t.Fatalf("line %d has No %d", i, l.No)
		}
	}
	// the screen goes to a GUI as JSON
	if _, err := json.Marshal(p); err != nil {
		t.Fatal(err)
	}

	// the lines are the visible ones: the cursor at the end scrolls the pane
	tb := a.Tabs[0]
	tb.Buf.Cur.Row = len(tb.Buf.Lines) - 1
	p = a.BuildPreviewPane(tb, 30, 5)
	if p.Preview.Top != p.Preview.Rows-5 || p.Lines[0].No != p.Preview.Top || len(p.Lines) != 5 {
		t.Fatalf("scrolled pane: top %d rows %d first %d", p.Preview.Top, p.Preview.Rows, p.Lines[0].No)
	}
}

func TestPreviewPaneRendersAgainWhenWidthChanges(t *testing.T) {
	a := previewApp(t, t.TempDir(), strings.Repeat("あ", 30)+"\n")
	tb := a.Tabs[0]
	a.FitPreview(tb, 20)
	narrow := a.BuildPreviewPane(tb, 20, 10)
	a.FitPreview(tb, 80)
	wide := a.BuildPreviewPane(tb, 80, 10)
	if len(narrow.Lines) <= len(wide.Lines) || tb.PrevW != 80 {
		t.Fatalf("lines %d (20 wide) %d (80 wide), cached for %d", len(narrow.Lines), len(wide.Lines), tb.PrevW)
	}
}

// Drawing reads the preview fit cached: it caches nothing itself (#284).
func TestDrawingLeavesPreviewCache(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.md")
	if err := os.WriteFile(p, []byte("# 見出し\n\n本文\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := mustNew(dir, p)
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	tb := a.CurText()
	tb.Preview = core.PreviewSplit
	before := a.renderFrameText()
	if tb.PrevW != 0 {
		t.Fatalf("drawing cached the preview at width %d", tb.PrevW)
	}
	a.fit()
	w := a.previewW(tb, a.layout())
	if w <= 0 || tb.PrevW != w {
		t.Fatalf("fit cached width %d, want %d", tb.PrevW, w)
	}
	if after := a.renderFrameText(); after != before {
		t.Errorf("the frame changed with the cache:\n%s\n---\n%s", before, after)
	}
	if tb.PrevW != w {
		t.Errorf("drawing changed the cached width to %d", tb.PrevW)
	}
}

func TestPreviewPaneImages(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, filepath.Join(dir, "a.png"), 40, 400)
	a := previewApp(t, dir, "![図](a.png)\n\n> ![引用](a.png)\n\n![無い](missing.png)\n")
	load := func() {
		a.SyncPreview()
		for path, job := range a.MdImages {
			a.HandleMDImageDone(core.MdImageDoneMsg{path, job.Stamp, core.NewImageView(path)})
		}
	}
	p := a.BuildPreviewPane(a.Tabs[0], 30, 100)
	if len(p.Preview.Images) != 0 {
		t.Fatalf("an image that is not loaded is text: %+v", p.Preview.Images)
	}
	load()
	p = a.BuildPreviewPane(a.Tabs[0], 30, 100)
	ims := p.Preview.Images
	if len(ims) != 2 {
		t.Fatalf("images = %+v", ims)
	}
	want := screen.ImageRef{Blob: core.BlobFile + filepath.Join(dir, "a.png"), Width: 40, Height: 400}
	for i, im := range ims {
		if im.Image != want || im.Rows < 3 {
			t.Fatalf("image %d = %+v, want %+v", i, im, want)
		}
		for r := im.Row; r < im.Row+im.Rows && r < len(p.Lines); r++ {
			if len(p.Lines[r].Spans) > 1 {
				t.Fatalf("image row %d holds text: %+v", r, p.Lines[r])
			}
		}
	}
	if ims[0].Col != 0 || ims[1].Col != 2 || ims[1].Row <= ims[0].Row+ims[0].Rows {
		t.Fatalf("a quoted image is beside the bar: %+v", ims)
	}
	if p.Lines[ims[1].Row].Spans[0].Style != screen.StyleMdQuote {
		t.Fatalf("quote bar missing: %+v", p.Lines[ims[1].Row])
	}
	if a.PreviewImage(want.Blob) == nil {
		t.Fatal("the Blob does not name the image")
	}

	// only the images in view are in the pane
	p = a.BuildPreviewPane(a.Tabs[0], 30, 2)
	if len(p.Preview.Images) != 1 || p.Preview.Images[0].Row != 0 {
		t.Fatalf("images in view = %+v", p.Preview.Images)
	}
}

func TestPreviewMermaidFallback(t *testing.T) {
	a := previewApp(t, t.TempDir(), "```mermaid\nA-->B\n```\n")
	m := a.MermaidLines("A-->B", 30)
	if m.Image != nil || len(m.Lines) == 0 {
		t.Fatalf("a diagram not drawn yet shows its source: %+v", m)
	}
}

func TestWheelScrollsPreviewNotCursor(t *testing.T) {
	a := previewApp(t, t.TempDir(), strings.Repeat("段落\n\n", 30))
	tb := a.Tabs[0]
	if p := a.BuildPreviewPane(tb, 30, 5); p.Preview.Top != 0 {
		t.Fatalf("top = %d", p.Preview.Top)
	}
	a.WheelPreview(3)
	a.WheelPreview(3)
	p := a.BuildPreviewPane(tb, 30, 5)
	if p.Preview.Top != 6 || tb.Buf.Cur.Row != 0 {
		t.Fatalf("top %d cursor %d", p.Preview.Top, tb.Buf.Cur.Row)
	}
	a.WheelPreview(-100)
	if p = a.BuildPreviewPane(tb, 30, 5); p.Preview.Top != 0 {
		t.Fatalf("top = %d", p.Preview.Top)
	}
	a.WheelPreview(1000)
	a.BuildPreviewPane(tb, 30, 5)
	a.WheelPreview(-3)
	p = a.BuildPreviewPane(tb, 30, 5)
	if p.Preview.Top != p.Preview.Rows-5-3 {
		t.Fatalf("scrolling back from the end: top %d of %d", p.Preview.Top, p.Preview.Rows)
	}
	tb.Buf.Cur.Row = 20 // the cursor moved: the preview follows it
	if p = a.BuildPreviewPane(tb, 30, 5); p.Preview.Top <= 0 {
		t.Fatalf("top = %d", p.Preview.Top)
	}
}

func TestWheelOverSplitPreview(t *testing.T) {
	a := newTestApp(t, strings.Repeat("段落\n\n", 60))
	tb := a.CurText()
	tb.Preview = core.PreviewSplit
	l := a.layout()
	a.BuildPreviewPane(tb, 30, 5) // PrevTop starts at its place
	wheel := func(x int) {
		a.HandleMouse(a.locate(x, l.editY+2, core.Mouse{Action: core.MousePress, Button: core.MouseWheelDown}))
	}
	wheel(l.rightX + a.editW(tb, l) + 2)
	if tb.PrevTop == 0 || tb.Buf.ScrollRow != 0 || tb.Buf.Cur.Row != 0 {
		t.Fatalf("over the preview: top %d scroll %d cursor %d", tb.PrevTop, tb.Buf.ScrollRow, tb.Buf.Cur.Row)
	}
	top := tb.PrevTop
	wheel(l.rightX + a.editW(tb, l) - 1)
	if tb.PrevTop != top || tb.Buf.ScrollRow == 0 {
		t.Fatalf("over the text: top %d scroll %d", tb.PrevTop, tb.Buf.ScrollRow)
	}
}
