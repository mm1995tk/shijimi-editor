package core

import (
	"fmt"
	"strings"
	"testing"
)

func TestPreviewPaneMarginAndScrollLimits(t *testing.T) {
	for _, src := range []string{"", "short", strings.Repeat("paragraph\n\n", 12)} {
		t.Run(fmt.Sprintf("source_rows_%d", strings.Count(src, "\n")+1), func(t *testing.T) {
			a := newTestApp(t, src)
			tb := a.CurText()
			tb.Preview = PreviewSplit
			tb.Buf.Cur.Row = 0
			// Reuse the tab across shrinking and growing viewports.
			for _, size := range [][2]int{{30, 8}, {12, 4}, {30, 3}, {12, 2}, {30, 1}, {12, 0}, {30, -1}, {12, 40}, {30, 8}} {
				w, h := size[0], size[1]
				a.FitPreview(tb, w)
				p := a.BuildPreviewPane(tb, w, h)
				rows := 0
				if h > 0 {
					rows = 1
					if h > 3 {
						rows = h - 2
					}
				}
				if p.Viewport.Rows != rows || p.Preview.Top != 0 {
					t.Fatalf("%dx%d: viewport = %+v, want %d text rows from top", w, h, p.Viewport, rows)
				}
				a.WheelPreview(1000)
				p = a.BuildPreviewPane(tb, w, h)
				wantTop := max(0, p.Preview.Rows-rows)
				if p.Preview.Top != wantTop || len(p.Lines) != min(rows, p.Preview.Rows) {
					t.Fatalf("%dx%d: end viewport = %+v, %d lines, want top %d", w, h, p.Viewport, len(p.Lines), wantTop)
				}
				if rows > 0 && len(p.Lines) > 0 && p.Lines[len(p.Lines)-1].No != p.Preview.Rows-1 {
					t.Fatalf("%dx%d: last document row missing", w, h)
				}
				a.WheelPreview(1000)
				if again := a.BuildPreviewPane(tb, w, h); again.Preview.Top != wantTop {
					t.Fatalf("%dx%d: scrolled past the end to %d", w, h, again.Preview.Top)
				}
				a.WheelPreview(-1000)
				a.BuildPreviewPane(tb, w, h)
				tb.Buf.Cur.Row = len(tb.Buf.Lines) - 1
				if p = a.BuildPreviewPane(tb, w, h); len(tb.Buf.Lines) > 1 && p.Preview.Top != wantTop {
					t.Fatalf("%dx%d: cursor follow stopped at %d, want %d", w, h, p.Preview.Top, wantTop)
				}
				tb.Buf.Cur.Row = 0
				tb.PrevTop = 0
				a.BuildPreviewPane(tb, w, h)
			}
		})
	}
}

func TestPreviewOneRowViewportCanReadEveryRow(t *testing.T) {
	a := newTestApp(t, "```\none\ntwo\nthree\n```\n")
	tb := a.CurText()
	tb.Preview = PreviewFull
	tb.Buf.Cur.Row = 0
	for _, h := range []int{1, 2, 3} {
		tb.PrevTop = 0
		p := a.BuildPreviewPane(tb, 20, h)
		for row := range p.Preview.Rows {
			p = a.BuildPreviewPane(tb, 20, h)
			if len(p.Lines) != 1 || p.Lines[0].No != row {
				t.Fatalf("height %d: document row %d missing: %+v", h, row, p.Lines)
			}
			a.WheelPreview(1)
		}
	}
}
