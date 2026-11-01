package tui

import (
	"fmt"
	"strings"

	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/kitty"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// sendImage sends v (nil: none) to the terminal at the size imageBody shows it
// in w x h cells, unless the terminal already holds it at that size. fit calls
// it for the images about to be drawn, so drawing sends nothing.
func sendImage(v *core.ImageView, w, h int) {
	if v == nil || v.Err != nil || v.Img == nil || h <= 0 || w <= 0 {
		return
	}
	src := v.Img.Bounds()
	v.Send(core.FitImage(src.Dx(), src.Dy(), w, h))
}

// sendMdImages sends the images of a Markdown document drawn w cells wide
// (drawMdRows) to the terminal.
func (a *App) sendMdImages(images []screen.PreviewImage, w int) {
	for _, im := range images {
		sendImage(a.PreviewImage(im.Image.Blob), max(0, w-im.Col), im.Rows)
	}
}

// sendImagePane sends the image of the image tab t, drawn in w x h cells
// (imageLines), to the terminal.
func (a *App) sendImagePane(t *core.Tab, w, h int) {
	if p := a.BuildImagePane(t, w, h); p.Image != nil {
		sendImage(a.BlobImage(p.Image.Blob), w, h-1)
	}
}

// imageBody draws the image on the w x h cells that are given to it, centred,
// as the placeholders of the image sendImage sent. Blank when it could not be
// read.
func imageBody(v *core.ImageView, w, h int) []string {
	cell := [2]int{core.CellW, core.CellH}
	if v.Cache != nil && v.CacheW == w && v.CacheH == h && v.CacheCell == cell && v.CacheGen == core.ThemeGen {
		return v.Cache
	}
	out := make([]string, 0, h)
	blank := stEmpty.Render(strings.Repeat(" ", max(0, w)))
	if v.Err != nil || v.Img == nil || h <= 0 || w <= 0 {
		for len(out) < h {
			out = append(out, blank)
		}
		return out
	}
	src := v.Img.Bounds()
	_, _, cols, rows := core.FitImage(src.Dx(), src.Dy(), w, h)

	padL := (w - cols) / 2
	padT := (h - rows) / 2
	for range padT {
		out = append(out, blank)
	}
	// The cells under a transparent image show the editor background.
	bg := fmt.Sprintf("\x1b[48;2;%sm", core.RgbParams(cEditorBg))
	for _, line := range kitty.Placeholders(v.Id, cols, rows) {
		if len(out) >= h {
			break
		}
		out = append(out, stEmpty.Render(strings.Repeat(" ", padL))+bg+line+"\x1b[0m"+
			stEmpty.Render(strings.Repeat(" ", max(0, w-padL-cols))))
	}
	for len(out) < h {
		out = append(out, blank)
	}
	v.CacheW, v.CacheH, v.CacheCell, v.CacheGen, v.Cache = w, h, cell, core.ThemeGen, out
	return out
}

// drawImagePane draws the Pane of an image as h lines of w cells: the caption
// line, then the image iv (nil: none) in the terminal (see body).
func drawImagePane(p screen.Pane, iv *core.ImageView, w, h int) []string {
	out := make([]string, 0, h)
	out = append(out, stPanelDim.Render(core.Fit("  "+p.Caption, w)))
	if iv == nil {
		iv = &core.ImageView{} // unreadable or gone: blank
	}
	if h-1 > 0 {
		out = append(out, imageBody(iv, w, h-1)...)
	}
	blank := stEmpty.Render(strings.Repeat(" ", max(0, w)))
	for len(out) < h {
		out = append(out, blank)
	}
	return out
}

// imageLines draws the image tab t in an area w x h.
func (a *App) imageLines(t *core.Tab, w, h int) []string {
	p := a.BuildImagePane(t, w, h)
	var iv *core.ImageView
	if p.Image != nil {
		iv = a.BlobImage(p.Image.Blob)
	}
	return drawImagePane(p, iv, w, h)
}
