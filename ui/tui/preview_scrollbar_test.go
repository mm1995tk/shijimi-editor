package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/core"
)

func TestPreviewScrollBarByMouse(t *testing.T) {
	for _, mode := range []core.PreviewMode{core.PreviewSplit, core.PreviewFull} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			a := newTestApp(t, "```text\n"+strings.Repeat(strings.Repeat("long word ", 24)+"\n", 80)+"```\n")
			tb := a.CurText()
			tb.Buf.Path = filepath.Join(t.TempDir(), "a.md")
			tb.Preview = mode
			tb.Buf.SetCursor(7, 3, false)
			cursor, scroll := tb.Buf.Cur, tb.Buf.ScrollRow
			for _, width := range []int{100, 76, 120} {
				a.Update(tea.WindowSizeMsg{Width: width, Height: 30})
				a.fit()
				l := a.layout()
				pw := a.previewW(tb, l)
				if tb.PrevW != pw || pw+1 != a.previewAreaW(tb, l) {
					t.Fatalf("cached width %d, text width %d, area %d", tb.PrevW, pw, a.previewAreaW(tb, l))
				}
				p := a.BuildPreviewPane(tb, pw, l.editH)
				if p.Viewport.Total <= len(tb.Buf.Lines) {
					t.Fatal("fixture must wrap into more display rows than source rows")
				}
				check := func() []string {
					t.Helper()
					a.fit()
					p := a.BuildPreviewPane(tb, pw, l.editH)
					bar := scrollBar(p.Viewport.Total, p.Viewport.Top, l.editH, nil)
					rows := a.renderText(tb, l, true)
					for i, row := range rows {
						if ansi.StringWidth(row) != l.rightW || !strings.HasSuffix(row, bar[i]) {
							t.Fatalf("width %d row %d: width %d, scrollbar missing or misplaced", width, i, ansi.StringWidth(row))
						}
					}
					if tb.Buf.Cur != cursor || tb.Buf.ScrollRow != scroll || a.IsDragging() {
						t.Fatalf("preview changed editor: cursor %v, scroll %d, dragging %v", tb.Buf.Cur, tb.Buf.ScrollRow, a.IsDragging())
					}
					return bar
				}
				mouse := func(x, y int, button core.MouseButton) {
					t.Helper()
					a.HandleMouse(a.locate(x, y, core.Mouse{Action: core.MousePress, Button: button}))
					a.HandleMouse(a.locate(x, y, core.Mouse{Action: core.MouseRelease, Button: button}))
				}
				barX := l.rightX + l.rightW - 1
				mouse(barX, l.editY, core.MouseLeft)
				before := check()
				for range 10 {
					mouse(barX, l.editY+2, core.MouseWheelDown)
				}
				after := check()
				if tb.PrevTop != 30 || slices.Equal(before, after) {
					t.Fatalf("wheel did not move preview and scrollbar: top %d", tb.PrevTop)
				}
				for _, i := range []int{l.editH / 2, l.editH - 1, 0} {
					mouse(barX, l.editY+i, core.MouseLeft)
					want := min(core.BarTarget(i, l.editH, p.Viewport.Total), p.Viewport.Total-l.editH)
					if tb.PrevTop != want {
						t.Fatalf("bar cell %d: top %d, want %d", i, tb.PrevTop, want)
					}
					check()
				}
				mouse(barX, l.editY+l.editH/2, core.MouseLeft)
				top := tb.PrevTop
				mouse(barX-1, l.editY+2, core.MouseLeft)
				mouse(barX, l.editY-1, core.MouseLeft)
				mouse(barX, l.editY+l.editH, core.MouseLeft)
				if tb.PrevTop != top {
					t.Fatal("click beside or outside the bar scrolled the preview")
				}
				check()
			}
		})
	}
}

func TestPreviewScrollBarImages(t *testing.T) {
	for _, mode := range []core.PreviewMode{core.PreviewSplit, core.PreviewFull} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			dir := t.TempDir()
			writePNG(t, filepath.Join(dir, "a.png"), 1200, 600)
			path := filepath.Join(dir, "a.md")
			if err := os.WriteFile(path, []byte("> ![image](a.png)\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			a := mustNew(dir, path)
			tb := a.CurText()
			tb.Preview = mode
			a.SyncPreview()
			for path, job := range a.MdImages {
				a.HandleMDImageDone(core.MdImageDoneMsg{Path: path, Stamp: job.Stamp, Img: core.NewImageView(path)})
			}
			for _, width := range []int{100, 76, 120} {
				a.Update(tea.WindowSizeMsg{Width: width, Height: 30})
				a.fit()
				l := a.layout()
				pw := a.previewW(tb, l)
				p := a.BuildPreviewPane(tb, pw, l.editH)
				if len(p.Preview.Images) != 1 {
					t.Fatalf("preview images: %+v", p.Preview.Images)
				}
				im := p.Preview.Images[0]
				iv := a.PreviewImage(im.Image.Blob)
				rows := a.renderText(tb, l, true)
				if iv.CacheW != pw-im.Col {
					t.Fatalf("image drawn at width %d, want %d", iv.CacheW, pw-im.Col)
				}
				b := iv.Img.Bounds()
				_, _, cols, imageRows := core.FitImage(b.Dx(), b.Dy(), pw-im.Col, im.Rows)
				if got := sent[iv.Id]; got.cols != cols || got.rows != imageRows {
					t.Fatalf("image sent at %+v, want %dx%d cells", got, cols, imageRows)
				}
				bar := scrollBar(p.Viewport.Total, p.Viewport.Top, l.editH, nil)
				for i, row := range rows {
					if ansi.StringWidth(row) != l.rightW || !strings.HasSuffix(row, bar[i]) {
						t.Fatalf("image overlaps bar at row %d (width %d)", i, ansi.StringWidth(row))
					}
				}
			}
		})
	}
}

func TestPreviewScrollBarSmallArea(t *testing.T) {
	a := previewApp(t, t.TempDir(), "# text\n\n> quoted\n")
	for _, w := range []int{0, 1, 2, 20} {
		for _, h := range []int{0, 1, 20} {
			rows := a.previewLines(a.Tabs[0], w, h)
			if len(rows) != h {
				t.Fatalf("area %dx%d: %d rows", w, h, len(rows))
			}
			for _, row := range rows {
				if ansi.StringWidth(row) != w {
					t.Fatalf("area %dx%d: row width %d", w, h, ansi.StringWidth(row))
				}
			}
		}
	}
}
