package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/config"
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

func previewApp(t *testing.T, dir, src string) *App {
	t.Helper()
	b := buffer.New()
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
	a := New(dir, p)
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
