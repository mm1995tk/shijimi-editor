// Package kitty implements the parts of the Kitty graphics protocol that
// gh-editor needs: detecting terminals that support it, sending an image, and
// drawing it with Unicode placeholders so that the TUI's own cell-based
// redraws (bubbletea) keep working. The image data is sent out of band; the
// screen only ever contains placeholder characters.
package kitty

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"image/jpeg"
	"os"
	"strings"
	"sync"

	"golang.org/x/sys/unix"
)

// EnvGraphics overrides the terminal detection: "kitty" forces the Kitty
// graphics protocol, "halfblock" (or "off") forces the half-block fallback.
const EnvGraphics = "GH_EDITOR_GRAPHICS"

// Supported reports whether the terminal draws Kitty graphics, judging by
// environment variables (getenv is os.Getenv in production). Kitty, Ghostty
// and WezTerm are recognised. Herdr is deliberately not: it redraws its panes
// without resending the images, so the page vanishes (#102). It inherits the
// outer terminal's variables, so it is checked first; set GH_EDITOR_GRAPHICS=kitty
// to use the protocol there anyway.
func Supported(getenv func(string) string) bool {
	switch strings.ToLower(getenv(EnvGraphics)) {
	case "kitty", "on", "1":
		return true
	case "halfblock", "off", "0":
		return false
	}
	if strings.EqualFold(getenv("TERM_PROGRAM"), "herdr") {
		return false
	}
	if getenv("KITTY_WINDOW_ID") != "" || getenv("GHOSTTY_RESOURCES_DIR") != "" || getenv("WEZTERM_EXECUTABLE") != "" {
		return true
	}
	switch strings.ToLower(getenv("TERM_PROGRAM")) {
	case "ghostty", "wezterm":
		return true
	}
	switch getenv("TERM") {
	case "xterm-kitty", "xterm-ghostty", "wezterm":
		return true
	}
	return false
}

// MaxCells is the largest number of rows or columns a placeholder image can
// span (the size of the diacritics table).
var MaxCells = len(diacritics)

const placeholder = '\U0010EEEE'

// Placeholders returns rows lines of cols placeholder cells for the image id.
// The image id is carried in the foreground colour (24-bit), so every line
// starts with that colour and ends by resetting it.
func Placeholders(id uint32, cols, rows int) []string {
	cols = min(max(cols, 0), MaxCells)
	rows = min(max(rows, 0), MaxCells)
	out := make([]string, rows)
	fg := fmt.Sprintf("\x1b[38;2;%d;%d;%dm", id>>16&0xff, id>>8&0xff, id&0xff)
	for r := range out {
		var sb strings.Builder
		sb.WriteString(fg)
		for c := 0; c < cols; c++ {
			sb.WriteRune(placeholder)
			sb.WriteRune(diacritics[r])
			sb.WriteRune(diacritics[c])
		}
		sb.WriteString("\x1b[39m")
		out[r] = sb.String()
	}
	return out
}

// chunkSize is the largest base64 payload per escape sequence the protocol
// allows.
const chunkSize = 4096

// Transmit returns the escape sequences that store the PNG data as image id
// and create a virtual placement of cols x rows cells, so that placeholder
// cells for id show it. Sending it again with the same id replaces the image
// in place, which is how video frames are shown. Replies are suppressed
// (q=2): the terminal would otherwise answer on stdin.
func Transmit(id uint32, png []byte, cols, rows int) string {
	return transmit(id, base64.StdEncoding.EncodeToString(png), "f=100", cols, rows)
}

// TransmitJPEG is Transmit for a JPEG frame. Kitty graphics cannot take JPEG,
// so the frame is decoded and sent as zlib-compressed RGB, which is far
// smaller than the PNG of the same video frame.
func TransmitJPEG(id uint32, data []byte, cols, rows int) (string, error) {
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	b := img.Bounds()
	var raw bytes.Buffer
	zw, _ := zlib.NewWriterLevel(&raw, zlib.BestSpeed)
	row := make([]byte, 0, b.Dx()*3)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row = row[:0]
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			row = append(row, byte(r>>8), byte(g>>8), byte(bl>>8))
		}
		_, _ = zw.Write(row)
	}
	if err := zw.Close(); err != nil {
		return "", err
	}
	enc := base64.StdEncoding.EncodeToString(raw.Bytes())
	return transmit(id, enc, fmt.Sprintf("f=24,o=z,s=%d,v=%d", b.Dx(), b.Dy()), cols, rows), nil
}

// transmit splits the base64 payload into chunks; format holds the f= (and
// size) keys of the first chunk.
func transmit(id uint32, enc, format string, cols, rows int) string {
	var sb strings.Builder
	for first := true; len(enc) > 0; first = false {
		n := min(len(enc), chunkSize)
		more := 0
		if n < len(enc) {
			more = 1
		}
		if first {
			fmt.Fprintf(&sb, "\x1b_Ga=T,U=1,%s,i=%d,c=%d,r=%d,C=1,q=2,m=%d;", format, id, cols, rows, more)
		} else {
			fmt.Fprintf(&sb, "\x1b_Gm=%d;", more)
		}
		sb.WriteString(enc[:n])
		sb.WriteString("\x1b\\")
		enc = enc[n:]
	}
	return sb.String()
}

// Delete returns the escape sequence that removes image id and frees its data.
func Delete(id uint32) string {
	return fmt.Sprintf("\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", id)
}

// Writer serialises writes to the terminal. bubbletea's renderer and the
// goroutine that sends images share one Writer, so an image's escape
// sequences are never interleaved with a frame of the TUI. It wraps a file
// and forwards Read, Close and Fd, which bubbletea needs to recognise the
// output as a terminal and to read its size.
type Writer struct {
	mu sync.Mutex
	f  *os.File
}

// NewWriter wraps f.
func NewWriter(f *os.File) *Writer { return &Writer{f: f} }

// Write implements io.Writer.
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.f.Write(p)
}

// WriteString writes s as one uninterrupted unit.
func (w *Writer) WriteString(s string) error {
	_, err := w.Write([]byte(s))
	return err
}

func (w *Writer) Read(p []byte) (int, error) { return w.f.Read(p) }
func (w *Writer) Close() error               { return w.f.Close() }
func (w *Writer) Fd() uintptr                { return w.f.Fd() }

// CellSize returns the size of one terminal cell in pixels, or 0, 0 when the
// terminal does not report it.
func CellSize(f *os.File) (w, h int) {
	ws, err := unix.IoctlGetWinsize(int(f.Fd()), unix.TIOCGWINSZ)
	if err != nil || ws.Col == 0 || ws.Row == 0 {
		return 0, 0
	}
	return int(ws.Xpixel) / int(ws.Col), int(ws.Ypixel) / int(ws.Row)
}
