package tui

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/kitty"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

var imageExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true, ".webp": true, ".tif": true, ".tiff": true}

func isImage(path string) bool {
	return imageExts[strings.ToLower(filepath.Ext(path))]
}

// imageView shows an image with the Kitty graphics protocol: the pixels are
// sent to the terminal out of band (see queueGraphics) and the view itself
// only holds Unicode placeholder cells that the terminal draws the image on.
type imageView struct {
	path   string
	img    image.Image
	format string
	size   int64
	err    error
	id     uint32 // Kitty image id

	sentCols, sentRows, sentPW, sentPH int // placement the terminal holds

	cacheW, cacheH int
	cacheCell      [2]int // cell size the cache was built for
	cacheGen       int    // themeGen the cache was built for
	cache          []string
}

func newImageView(path string) *imageView {
	v := &imageView{path: path, id: nextImageID()}
	f, err := os.Open(path)
	if err != nil {
		v.err = err
		return v
	}
	defer f.Close()
	if st, err := f.Stat(); err == nil {
		v.size = st.Size()
	}
	v.img, v.format, v.err = image.Decode(f)
	return v
}

// newImageViewFromData shows already-loaded image bytes (for example a
// rendered web page) under the given display name.
func newImageViewFromData(name string, data []byte) *imageView {
	v := &imageView{path: name, size: int64(len(data)), id: nextImageID()}
	v.img, v.format, v.err = image.Decode(bytes.NewReader(data))
	return v
}

func (v *imageView) name() string { return filepath.Base(v.path) }

func (v *imageView) info() string {
	if v.err != nil {
		return i18n.T("image.cantload") + v.err.Error()
	}
	b := v.img.Bounds()
	return fmt.Sprintf("%s  %d×%d  %s", strings.ToUpper(v.format), b.Dx(), b.Dy(), humanSize(v.size))
}

func humanSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

// Cell size in pixels, used to keep an image's aspect ratio and to send it at
// the resolution it is shown at. Terminals that do not report it get a
// typical 1:2 cell.
var cellW, cellH = 10, 20

// updateCellSize records the cell size the terminal reports; zeros (not
// reported) keep the previous value.
func updateCellSize(w, h int) {
	if w > 0 && h > 0 {
		cellW, cellH = w, h
	}
}

var (
	imageIDMu   sync.Mutex
	lastImageID = uint32(rand.IntN(1<<24 - 1))
)

// nextImageID returns a fresh Kitty image id. Ids start at a random point so
// that two shijimi in panes of one terminal do not replace each other's
// images. The id is carried in a 24-bit colour, so it stays below 1<<24.
func nextImageID() uint32 {
	imageIDMu.Lock()
	defer imageIDMu.Unlock()
	lastImageID++
	if lastImageID >= 1<<24 {
		lastImageID = 1
	}
	return lastImageID
}

// graphics holds Kitty graphics sequences (image data, deletions) waiting to
// be written to the terminal. Views queue them while rendering; App.View
// sends them before the frame that shows the placeholders.
var graphics []string

func queueGraphics(s string) { graphics = append(graphics, s) }

// takeGraphics returns the queued sequences and empties the queue.
func takeGraphics() string {
	s := strings.Join(graphics, "")
	graphics = nil
	return s
}

// fitImage returns the size an image of sw x sh pixels is shown at in a box
// of w x h cells: its pixel size (never enlarged) and the cells it spans.
func fitImage(sw, sh, w, h int) (pw, ph, cols, rows int) {
	maxCols, maxRows := min(w, kitty.MaxCells), min(h, kitty.MaxCells)
	scale := min(float64(maxCols*cellW)/float64(sw), float64(maxRows*cellH)/float64(sh), 1)
	pw = max(1, int(math.Round(float64(sw)*scale)))
	ph = max(1, int(math.Round(float64(sh)*scale)))
	cols = clamp((pw+cellW-1)/cellW, 1, max(1, maxCols))
	rows = clamp((ph+cellH-1)/cellH, 1, max(1, maxRows))
	return pw, ph, cols, rows
}

// rowsFor returns the rows the image needs when it is w columns wide at most.
func (v *imageView) rowsFor(w int) int {
	b := v.img.Bounds()
	_, _, _, rows := fitImage(b.Dx(), b.Dy(), w, kitty.MaxCells)
	return rows
}

// send queues the image scaled to pw x ph pixels, placed on cols x rows cells,
// unless the terminal already holds exactly that.
func (v *imageView) send(pw, ph, cols, rows int) {
	if v.sentCols == cols && v.sentRows == rows && v.sentPW == pw && v.sentPH == ph {
		return
	}
	src := v.img.Bounds()
	img := v.img
	if pw != src.Dx() || ph != src.Dy() {
		dst := image.NewRGBA(image.Rect(0, 0, pw, ph))
		draw.CatmullRom.Scale(dst, dst.Bounds(), v.img, src, draw.Src, nil)
		img = dst
	}
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, img); err != nil {
		return
	}
	queueGraphics(kitty.Transmit(v.id, buf.Bytes(), cols, rows))
	v.sentCols, v.sentRows, v.sentPW, v.sentPH = cols, rows, pw, ph
}

// close frees the image in the terminal.
func (v *imageView) close() {
	if v.sentCols != 0 {
		queueGraphics(kitty.Delete(v.id))
		v.sentCols = 0
	}
}

// buildImagePane makes the Pane of the image tab t, w columns wide and h rows
// high: what the image is (Caption, and Image when it could be read; the pixels
// are what Image.Blob names). It changes nothing.
func (a *App) buildImagePane(t *tab, w, h int) screen.Pane {
	v := t.img
	p := screen.Pane{
		ID:       v.path,
		Kind:     screen.KindImage,
		Title:    screen.Title{Text: v.name()},
		Viewport: screen.Viewport{Rows: h, Cols: w},
		Caption:  v.info(),
	}
	if v.err == nil {
		b := v.img.Bounds()
		p.Image = &screen.ImageRef{Blob: blobFile + v.path, Width: b.Dx(), Height: b.Dy(), Format: v.format, Size: v.size}
	}
	return p
}

// imageLines draws the image tab t in an area w x h.
func (a *App) imageLines(t *tab, w, h int) []string {
	return a.drawImagePane(a.buildImagePane(t, w, h), w, h)
}

// drawImagePane draws the Pane of an image as h lines of w cells: the caption
// line, then the image in the terminal (see body).
func (a *App) drawImagePane(p screen.Pane, w, h int) []string {
	out := make([]string, 0, h)
	out = append(out, stPanelDim.Render(fit("  "+p.Caption, w)))
	iv := &imageView{} // unreadable or gone: blank
	if p.Image != nil {
		if got := a.blobImage(p.Image.Blob); got != nil {
			iv = got
		}
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

// blobImage is the image that a Blob names (blob.get): a local file, which may
// be open in a tab, or a mermaid diagram of the preview. nil when it is gone.
func (a *App) blobImage(blob string) *imageView {
	if iv := a.previewImage(blob); iv != nil {
		return iv
	}
	if path, ok := strings.CutPrefix(blob, blobFile); ok {
		for _, t := range a.tabs {
			if t.kind == tabImage && t.img.path == path {
				return t.img
			}
		}
	}
	return nil
}

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

func rgbParams(hex string) string {
	var r, g, b int
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	return fmt.Sprintf("%d;%d;%d", r, g, b)
}
