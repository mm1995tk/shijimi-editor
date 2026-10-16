package ui

import (
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

var imageExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true, ".webp": true, ".tif": true, ".tiff": true}

func isImage(path string) bool {
	return imageExts[strings.ToLower(filepath.Ext(path))]
}

// imageView renders an image with "▀" half blocks: each cell shows two
// vertically stacked pixels (foreground = top, background = bottom).
type imageView struct {
	path   string
	img    image.Image
	format string
	size   int64
	err    error

	cacheW, cacheH int
	cacheGen       int // themeGen the cache was built for
	cache          []string
}

func newImageView(path string) *imageView {
	v := &imageView{path: path}
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

func (v *imageView) name() string { return filepath.Base(v.path) }

func (v *imageView) info() string {
	if v.err != nil {
		return "画像を読み込めません: " + v.err.Error()
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

func (v *imageView) render(w, h int) []string {
	if v.cache != nil && v.cacheW == w && v.cacheH == h && v.cacheGen == themeGen {
		return v.cache
	}
	out := make([]string, 0, h)
	out = append(out, stPanelDim.Render(fit("  "+v.info(), w)))
	body := h - 1
	if v.err != nil || body <= 0 {
		for len(out) < h {
			out = append(out, stEmpty.Render(strings.Repeat(" ", w)))
		}
		return out
	}
	src := v.img.Bounds()
	// target size in pixels: w columns, 2*body rows; keep the aspect ratio.
	pw, ph := w, body*2
	scale := min(float64(pw)/float64(src.Dx()), float64(ph)/float64(src.Dy()))
	tw := max(1, int(float64(src.Dx())*scale))
	th := max(2, int(float64(src.Dy())*scale))
	dst := image.NewRGBA(image.Rect(0, 0, tw, th))
	draw.CatmullRom.Scale(dst, dst.Bounds(), v.img, src, draw.Src, nil)

	padL := (w - tw) / 2
	padT := (body - (th+1)/2) / 2
	blank := stEmpty.Render(strings.Repeat(" ", w))
	for i := 0; i < padT; i++ {
		out = append(out, blank)
	}
	for y := 0; y < th && len(out) < h; y += 2 {
		var sb strings.Builder
		sb.WriteString(stEmpty.Render(strings.Repeat(" ", padL)))
		for x := 0; x < tw; x++ {
			top := blend(dst.At(x, y))
			bot := cEditorBg
			if y+1 < th {
				bot = blend(dst.At(x, y+1))
			}
			sb.WriteString(fmt.Sprintf("\x1b[38;2;%sm\x1b[48;2;%sm▀", rgbParams(top), rgbParams(bot)))
		}
		sb.WriteString("\x1b[0m")
		sb.WriteString(stEmpty.Render(strings.Repeat(" ", max(0, w-padL-tw))))
		out = append(out, sb.String())
	}
	for len(out) < h {
		out = append(out, blank)
	}
	v.cacheW, v.cacheH, v.cacheGen, v.cache = w, h, themeGen, out
	return out
}

// blend composites a pixel over the editor background and returns "#rrggbb".
func blend(c color.Color) string {
	r, g, b, a := c.RGBA()
	const bg = 0x1e
	mix := func(v uint32) uint32 {
		// v is premultiplied 16-bit
		return (v + bg*257*(0xffff-a)/0xffff) >> 8
	}
	return fmt.Sprintf("#%02x%02x%02x", mix(r), mix(g), mix(b))
}

func rgbParams(hex string) string {
	var r, g, b int
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	return fmt.Sprintf("%d;%d;%d", r, g, b)
}
