package core

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
type ImageView struct {
	Path   string
	Img    image.Image
	Format string
	Size   int64
	Err    error
	Id     uint32 // Kitty image id

	sentCols, sentRows, sentPW, sentPH int // placement the terminal holds

	CacheW, CacheH int
	CacheCell      [2]int // cell size the cache was built for
	CacheGen       int    // themeGen the cache was built for
	Cache          []string
}

func NewImageView(path string) *ImageView {
	v := &ImageView{Path: path, Id: nextImageID()}
	f, err := os.Open(path)
	if err != nil {
		v.Err = err
		return v
	}
	defer f.Close()
	if st, err := f.Stat(); err == nil {
		v.Size = st.Size()
	}
	v.Img, v.Format, v.Err = image.Decode(f)
	return v
}

// newImageViewFromData shows already-loaded image bytes (for example a
// rendered web page) under the given display name.
func NewImageViewFromData(name string, data []byte) *ImageView {
	v := &ImageView{Path: name, Size: int64(len(data)), Id: nextImageID()}
	v.Img, v.Format, v.Err = image.Decode(bytes.NewReader(data))
	return v
}

func (v *ImageView) Name() string { return filepath.Base(v.Path) }

func (v *ImageView) Info() string {
	if v.Err != nil {
		return i18n.T("image.cantload") + v.Err.Error()
	}
	b := v.Img.Bounds()
	return fmt.Sprintf("%s  %d×%d  %s", strings.ToUpper(v.Format), b.Dx(), b.Dy(), humanSize(v.Size))
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
var CellW, CellH = 10, 20

// updateCellSize records the cell size the terminal reports; zeros (not
// reported) keep the previous value.
func UpdateCellSize(w, h int) {
	if w > 0 && h > 0 {
		CellW, CellH = w, h
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
func TakeGraphics() string {
	s := strings.Join(graphics, "")
	graphics = nil
	return s
}

// fitImage returns the size an image of sw x sh pixels is shown at in a box
// of w x h cells: its pixel size (never enlarged) and the cells it spans.
func FitImage(sw, sh, w, h int) (pw, ph, cols, rows int) {
	maxCols, maxRows := min(w, kitty.MaxCells), min(h, kitty.MaxCells)
	scale := min(float64(maxCols*CellW)/float64(sw), float64(maxRows*CellH)/float64(sh), 1)
	pw = max(1, int(math.Round(float64(sw)*scale)))
	ph = max(1, int(math.Round(float64(sh)*scale)))
	cols = Clamp((pw+CellW-1)/CellW, 1, max(1, maxCols))
	rows = Clamp((ph+CellH-1)/CellH, 1, max(1, maxRows))
	return pw, ph, cols, rows
}

// rowsFor returns the rows the image needs when it is w columns wide at most.
func (v *ImageView) RowsFor(w int) int {
	b := v.Img.Bounds()
	_, _, _, rows := FitImage(b.Dx(), b.Dy(), w, kitty.MaxCells)
	return rows
}

// send queues the image scaled to pw x ph pixels, placed on cols x rows cells,
// unless the terminal already holds exactly that.
func (v *ImageView) Send(pw, ph, cols, rows int) {
	if v.sentCols == cols && v.sentRows == rows && v.sentPW == pw && v.sentPH == ph {
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
	queueGraphics(kitty.Transmit(v.Id, buf.Bytes(), cols, rows))
	v.sentCols, v.sentRows, v.sentPW, v.sentPH = cols, rows, pw, ph
}

// close frees the image in the terminal.
func (v *ImageView) Close() {
	if v.sentCols != 0 {
		queueGraphics(kitty.Delete(v.Id))
		v.sentCols = 0
	}
}

// buildImagePane makes the Pane of the image tab t, w columns wide and h rows
// high: what the image is (Caption, and Image when it could be read; the pixels
// are what Image.Blob names). It changes nothing.
func (a *Core) BuildImagePane(t *Tab, w, h int) screen.Pane {
	v := t.Img
	p := screen.Pane{
		ID:       v.Path,
		Kind:     screen.KindImage,
		Title:    screen.Title{Text: v.Name()},
		Viewport: screen.Viewport{Rows: h, Cols: w},
		Caption:  v.Info(),
	}
	if v.Err == nil {
		b := v.Img.Bounds()
		p.Image = &screen.ImageRef{Blob: BlobFile + v.Path, Width: b.Dx(), Height: b.Dy(), Format: v.Format, Size: v.Size}
	}
	return p
}

// blobImage is the image that a Blob names (blob.get): a local file, which may
// be open in a tab, or a mermaid diagram of the preview. nil when it is gone.
func (a *Core) BlobImage(blob string) *ImageView {
	if iv := a.PreviewImage(blob); iv != nil {
		return iv
	}
	if path, ok := strings.CutPrefix(blob, BlobFile); ok {
		for _, t := range a.Tabs {
			if t.Kind == TabImage && t.Img.Path == path {
				return t.Img
			}
		}
	}
	return nil
}

func RgbParams(hex string) string {
	var r, g, b int
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	return fmt.Sprintf("%d;%d;%d", r, g, b)
}
