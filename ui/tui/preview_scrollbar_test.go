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
	"github.com/mm1995tk/shijimi-editor/core/screen"
)

func previewTestBar(p screen.Pane, h int) []string {
	bar := scrollBar(p.Viewport.Total, p.Viewport.Top, p.Viewport.Rows, nil)
	return append(bar, slices.Repeat([]string{stEmpty.Render(" ")}, h-p.Viewport.Rows)...)
}

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
					bar := previewTestBar(p, l.editH)
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
				for _, i := range []int{p.Viewport.Rows / 2, p.Viewport.Rows - 1, 0} {
					mouse(barX, l.editY+i, core.MouseLeft)
					want := min(core.BarTarget(i, p.Viewport.Rows, p.Viewport.Total), p.Viewport.Total-p.Viewport.Rows)
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
				for i := p.Viewport.Rows; i < l.editH; i++ {
					mouse(barX, l.editY+i, core.MouseLeft)
				}
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
				bar := previewTestBar(p, l.editH)
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
		for _, h := range []int{0, 1, 2, 3, 4, 20} {
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

func TestPreviewScrollBarGraphemeAtRightEdge(t *testing.T) {
	for _, mode := range []core.PreviewMode{core.PreviewSplit, core.PreviewFull} {
		for _, emoji := range []string{"❤️", "👩‍💻", "🇯🇵", "é"} {
			t.Run(fmt.Sprintf("%d/%s", mode, emoji), func(t *testing.T) {
				a := newTestApp(t, "")
				tb := a.CurText()
				tb.Buf.Path = filepath.Join(t.TempDir(), "a.md")
				tb.Preview = mode
				a.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
				a.fit()
				l := a.layout()
				pw := a.previewW(tb, l)
				prefix := strings.Repeat("x", pw-1)
				tb.Buf.Insert(prefix + emoji)
				p := a.BuildPreviewPane(tb, pw, l.editH)
				rows := a.drawPreview(p, pw+1, l.editH)
				bar := previewTestBar(p, l.editH)
				var text strings.Builder
				for i, row := range rows {
					if ansi.StringWidth(row) != pw+1 || !strings.HasSuffix(row, bar[i]) {
						t.Fatalf("row %d: width %d, want %d", i, ansi.StringWidth(row), pw+1)
					}
					text.WriteString(strings.TrimRight(ansi.Strip(strings.TrimSuffix(row, bar[i])), " "))
				}
				if text.String() != prefix+emoji {
					t.Fatalf("wrapped text = %q, want %q", text.String(), prefix+emoji)
				}
				if ansi.StringWidth(emoji) > 1 && p.Viewport.Total != 2 {
					t.Fatalf("wrapped rows = %d, want 2", p.Viewport.Total)
				}
			})
		}
	}
}

func TestPreviewScrollBarPadsTruncatedGrapheme(t *testing.T) {
	a := previewApp(t, t.TempDir(), "")
	p := screen.Pane{
		Lines:    []screen.Line{core.MdRow(core.PlainSpan("x❤️"))},
		Preview:  &screen.PreviewInfo{},
		Viewport: screen.Viewport{Total: 1, Rows: 1},
	}
	rows := a.drawPreview(p, 3, 1)
	bar := scrollBar(1, 0, 1, nil)[0]
	if want := "x " + ansi.Strip(bar); ansi.Strip(rows[0]) != want {
		t.Fatalf("clipped preview = %q, want %q", ansi.Strip(rows[0]), want)
	}
}

func TestPreviewScrollBarWrapsTabAtRightEdge(t *testing.T) {
	const width = 60
	prefix := strings.Repeat("x", width-3)
	a := previewApp(t, t.TempDir(), prefix+"\tZ")
	rows := a.previewLines(a.Tabs[0], width+1, 4)
	bar := scrollBar(2, 0, 2, nil)
	for i, want := range []string{prefix, "Z"} {
		row := ansi.Strip(strings.TrimSuffix(rows[i], bar[i]))
		if ansi.StringWidth(row) != width || strings.TrimRight(row, " ") != want {
			t.Errorf("row %d before the scrollbar = %q, want %q padded to %d columns", i, row, want, width)
		}
	}
}

func TestPreviewBottomMarginInBothLayouts(t *testing.T) {
	for _, mode := range []core.PreviewMode{core.PreviewSplit, core.PreviewFull} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			a := newTestApp(t, "```text\n"+strings.Repeat("body\n", 50)+"```")
			tb := a.CurText()
			tb.Buf.Path = filepath.Join(t.TempDir(), "a.md")
			tb.Preview = mode
			tb.Buf.Cur.Row = 0
			for _, size := range [][2]int{{100, 30}, {76, 12}, {120, 8}, {100, 30}} {
				a.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
				a.fit()
				l := a.layout()
				pw := a.previewW(tb, l)
				for _, end := range []bool{false, true} {
					if end {
						a.WheelPreview(10000)
					} else {
						a.WheelPreview(-10000)
					}
					p := a.BuildPreviewPane(tb, pw, l.editH)
					rows := a.renderText(tb, l, true)
					if len(rows) != l.editH || p.Viewport.Rows != l.editH-2 {
						t.Fatalf("%v: height %d, text rows %d, drawn rows %d", size, l.editH, p.Viewport.Rows, len(rows))
					}
					for i := l.editH - 2; i < l.editH; i++ {
						margin := ansi.Strip(ansi.Cut(rows[i], l.rightW-pw-1, l.rightW))
						if margin != strings.Repeat(" ", pw+1) {
							t.Fatalf("%v, end %v: margin row %d = %q", size, end, i, margin)
						}
					}
					if end && (p.Preview.Top != p.Preview.Rows-p.Viewport.Rows || p.Lines[len(p.Lines)-1].No != p.Preview.Rows-1) {
						t.Fatalf("%v: document end missing: %+v", size, p.Viewport)
					}
				}
			}
		})
	}
}

func TestPreviewWheelReadsEveryRowInShortViewport(t *testing.T) {
	for _, mode := range []core.PreviewMode{core.PreviewSplit, core.PreviewFull} {
		for _, height := range []int{11, 12} {
			t.Run(fmt.Sprintf("mode_%d/height_%d", mode, height), func(t *testing.T) {
				a := newTestApp(t, "```text\n"+strings.Repeat("body\n", 12)+"```")
				tb := a.CurText()
				tb.Buf.Path = filepath.Join(t.TempDir(), "a.md")
				tb.Preview = mode
				tb.Buf.Cur.Row = 0
				a.showTerm = true
				a.Update(tea.WindowSizeMsg{Width: 100, Height: height})
				a.fit()
				l := a.layout()
				pw := a.previewW(tb, l)
				p := a.BuildPreviewPane(tb, pw, l.editH)
				if p.Viewport.Rows != height-10 {
					t.Fatalf("fixture has %d text rows, want %d", p.Viewport.Rows, height-10)
				}
				for _, button := range []core.MouseButton{core.MouseWheelDown, core.MouseWheelUp} {
					seen := make([]bool, p.Viewport.Total)
					for range p.Viewport.Total {
						for _, line := range p.Lines {
							seen[line.No] = true
						}
						before := p.Viewport.Top
						a.HandleMouse(a.locate(l.rightX+l.rightW-1, l.editY, core.Mouse{Action: core.MousePress, Button: button}))
						p = a.BuildPreviewPane(tb, pw, l.editH)
						if delta := p.Viewport.Top - before; delta < -p.Viewport.Rows || delta > p.Viewport.Rows {
							t.Fatalf("wheel %v moved %d rows with only %d visible", button, delta, p.Viewport.Rows)
						}
					}
					for row, visible := range seen {
						if !visible {
							t.Fatalf("wheel %v skipped document row %d", button, row)
						}
					}
				}
			})
		}
	}
}
