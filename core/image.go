package core

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"github.com/mm1995tk/shijimi-editor/core/msg"
	"github.com/mm1995tk/shijimi-editor/core/screen"
)

var imageExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true, ".webp": true, ".tif": true, ".tiff": true}

func isImage(path string) bool {
	return imageExts[strings.ToLower(filepath.Ext(path))]
}

// ImageView is an image that is shown: what was read of the file, and Id, the
// number a UI uses to name the picture. How it is shown is the UI's business:
// the TUI sends the pixels to the terminal out of band (ui/tui).
type ImageView struct {
	Path   string
	Img    image.Image
	Format string
	Size   int64
	Err    error
	Id     uint32 // the picture's number, unique in the process (the TUI names its terminal graphics by it)

	CacheW, CacheH int
	CacheCell      [2]int // cell size the cache was built for
	CacheGen       int    // ThemeGen the cache was built for
	Cache          []string
}

// NewImageView loads the image at path.
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

// NewImageViewFromData shows already-loaded image bytes (for example a
// rendered web page) under the given display name.
func NewImageViewFromData(name string, data []byte) *ImageView {
	v := &ImageView{Path: name, Size: int64(len(data)), Id: nextImageID()}
	v.Img, v.Format, v.Err = image.Decode(bytes.NewReader(data))
	return v
}

// Name is the image's file name.
func (v *ImageView) Name() string { return filepath.Base(v.Path) }

// Info is the line about the image shown under it: its size, or why it could
// not be loaded.
func (v *ImageView) Info() msg.Msg {
	if v.Err != nil {
		return msg.T("image.cantload").Add(v.Err)
	}
	b := v.Img.Bounds()
	return msg.S(fmt.Sprintf("%s  %d×%d  %s", strings.ToUpper(v.Format), b.Dx(), b.Dy(), humanSize(v.Size)))
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

// UpdateCellSize records the cell size the terminal reports; zeros (not
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

// FitImage returns the size an image of sw x sh pixels is shown at in a box
// of w x h cells: its pixel size (never enlarged) and the cells it spans.
func FitImage(sw, sh, w, h int) (pw, ph, cols, rows int) {
	maxCols, maxRows := min(w, MaxImageCells), min(h, MaxImageCells)
	scale := min(float64(maxCols*CellW)/float64(sw), float64(maxRows*CellH)/float64(sh), 1)
	pw = max(1, int(math.Round(float64(sw)*scale)))
	ph = max(1, int(math.Round(float64(sh)*scale)))
	cols = Clamp((pw+CellW-1)/CellW, 1, max(1, maxCols))
	rows = Clamp((ph+CellH-1)/CellH, 1, max(1, maxRows))
	return pw, ph, cols, rows
}

// RowsFor returns the rows the image needs when it is w columns wide at most.
func (v *ImageView) RowsFor(w int) int {
	b := v.Img.Bounds()
	_, _, _, rows := FitImage(b.Dx(), b.Dy(), w, MaxImageCells)
	return rows
}

// BuildImagePane makes the Pane of the image tab t, w columns wide and h rows
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

// BlobImage is the image that a Blob names (the TUI draws it, image_draw.go): a local file, which may
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

// RgbParams is the color #rrggbb as the "r;g;b" of an SGR sequence.
func RgbParams(hex string) string {
	var r, g, b int
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	return fmt.Sprintf("%d;%d;%d", r, g, b)
}

// MaxImageCells is the largest number of rows or columns an image may span.
// A UI whose pictures have such a limit (the TUI's terminal graphics) sets it
// before it draws; until then there is none.
var MaxImageCells = 1 << 20
