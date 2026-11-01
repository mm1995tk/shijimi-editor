package tui

import (
	"fmt"
	"strings"

	"github.com/mm1995tk/shijimi-editor/internal/kitty"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// body draws the image on the w x h cells that are given to it, centred, and
// sends it to the terminal at the size it is shown at. Blank when it could not
// be read.
func (v *imageView) body(w, h int) []string {
	cell := [2]int{cellW, cellH}
	if v.cache != nil && v.cacheW == w && v.cacheH == h && v.cacheCell == cell && v.cacheGen == themeGen {
		return v.cache
	}
	out := make([]string, 0, h)
	blank := stEmpty.Render(strings.Repeat(" ", max(0, w)))
	if v.err != nil || v.img == nil || h <= 0 || w <= 0 {
		for len(out) < h {
			out = append(out, blank)
		}
		return out
	}
	src := v.img.Bounds()
	pw, ph, cols, rows := fitImage(src.Dx(), src.Dy(), w, h)
	v.send(pw, ph, cols, rows)

	padL := (w - cols) / 2
	padT := (h - rows) / 2
	for range padT {
		out = append(out, blank)
	}
	// The cells under a transparent image show the editor background.
	bg := fmt.Sprintf("\x1b[48;2;%sm", rgbParams(cEditorBg))
	for _, line := range kitty.Placeholders(v.id, cols, rows) {
		if len(out) >= h {
			break
		}
		out = append(out, stEmpty.Render(strings.Repeat(" ", padL))+bg+line+"\x1b[0m"+
			stEmpty.Render(strings.Repeat(" ", max(0, w-padL-cols))))
	}
	for len(out) < h {
		out = append(out, blank)
	}
	v.cacheW, v.cacheH, v.cacheCell, v.cacheGen, v.cache = w, h, cell, themeGen, out
	return out
}

// drawImagePane draws the Pane of an image as h lines of w cells: the caption
// line, then the image iv (nil: none) in the terminal (see body).
func drawImagePane(p screen.Pane, iv *imageView, w, h int) []string {
	out := make([]string, 0, h)
	out = append(out, stPanelDim.Render(fit("  "+p.Caption, w)))
	if iv == nil {
		iv = &imageView{} // unreadable or gone: blank
	}
	if h-1 > 0 {
		out = append(out, iv.body(w, h-1)...)
	}
	blank := stEmpty.Render(strings.Repeat(" ", max(0, w)))
	for len(out) < h {
		out = append(out, blank)
	}
	return out
}

// imageLines draws the image tab t in an area w x h.
func (a *App) imageLines(t *tab, w, h int) []string {
	p := a.buildImagePane(t, w, h)
	var iv *imageView
	if p.Image != nil {
		iv = a.blobImage(p.Image.Blob)
	}
	return drawImagePane(p, iv, w, h)
}
