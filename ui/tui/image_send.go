package tui

import (
	"bytes"
	"image"
	"image/png"
	"strings"

	"golang.org/x/image/draw"

	"github.com/mm1995tk/shijimi-editor/core"
	"github.com/mm1995tk/shijimi-editor/ui/tui/kitty"
)

// The pixels of an image go to the terminal with the Kitty graphics protocol,
// out of band, and the frame only holds the Unicode placeholder cells the
// terminal draws the image on. core knows none of this: it holds the image
// (core.ImageView) and what limit a picture has (core.MaxImageCells).

func init() {
	core.MaxImageCells = kitty.MaxCells
}

// sentImage is the placement of an image the terminal holds.
type sentImage struct{ cols, rows, pw, ph int }

var (
	// graphics holds Kitty graphics sequences (image data, deletions) waiting
	// to be written to the terminal. fit queues them; View sends them before
	// the frame that shows the placeholders.
	graphics []string
	sent     = map[uint32]sentImage{}
)

// takeGraphics returns the queued sequences and empties the queue.
func takeGraphics() string {
	s := strings.Join(graphics, "")
	graphics = nil
	return s
}

// sendPixels queues the image scaled to pw x ph pixels, placed on cols x rows
// cells, unless the terminal already holds exactly that.
func sendPixels(v *core.ImageView, pw, ph, cols, rows int) {
	want := sentImage{cols, rows, pw, ph}
	if sent[v.Id] == want {
		return
	}
	src := v.Img.Bounds()
	img := v.Img
	if pw != src.Dx() || ph != src.Dy() {
		dst := image.NewRGBA(image.Rect(0, 0, pw, ph))
		draw.CatmullRom.Scale(dst, dst.Bounds(), v.Img, src, draw.Src, nil)
		img = dst
	}
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, img); err != nil {
		return
	}
	graphics = append(graphics, kitty.Transmit(v.Id, buf.Bytes(), cols, rows))
	sent[v.Id] = want
}

// ImageClosed (core.Port) frees in the terminal the image id, which is not shown any more.
func (a *App) ImageClosed(id uint32) {
	if _, ok := sent[id]; ok {
		graphics = append(graphics, kitty.Delete(id))
		delete(sent, id)
	}
}
