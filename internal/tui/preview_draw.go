package tui

import (
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// previewLines draws the Markdown preview of t in an area w x h.
func (a *App) previewLines(t *tab, w, h int) []string {
	return a.drawPreview(a.buildPreviewPane(t, w, h), w, h)
}

// drawPreview draws the Pane of a Markdown preview as h lines of w cells.
func (a *App) drawPreview(p screen.Pane, w, h int) []string {
	out := a.drawMdRows(p.Lines, p.Preview.Top, p.Preview.Images, w)
	for len(out) < h {
		out = append(out, stEmpty.Render(strings.Repeat(" ", w)))
	}
	return out
}

// drawMdDoc draws a rendered Markdown document as lines of w cells.
func (a *App) drawMdDoc(doc mdDoc, w int) []string {
	return a.drawMdRows(doc.Lines, 0, doc.Images, w)
}

// markdownLines renders src as lines of w cells.
func (a *App) markdownLines(src string, w int, o mdOpts) []string {
	return a.drawMdDoc(renderMarkdown(src, w, o), w)
}

// drawMdRows draws lines, which are the rows of a document from row top, as
// lines of w cells. The rows of images are drawn from the images in the
// terminal; images are the ones that may be in view.
func (a *App) drawMdRows(lines []screen.Line, top int, images []screen.PreviewImage, w int) []string {
	out := make([]string, 0, len(lines))
	rows := map[int][]string{} // the rows of an image, by its index in images
	for i, l := range lines {
		row := top + i
		idx := slices.IndexFunc(images, func(im screen.PreviewImage) bool { return row >= im.Row && row < im.Row+im.Rows })
		if idx < 0 {
			out = append(out, drawMdLine(l, w))
			continue
		}
		im := images[idx]
		if rows[idx] == nil {
			iw := max(0, w-im.Col)
			if iv := a.previewImage(im.Image.Blob); iv != nil {
				rows[idx] = iv.body(iw, im.Rows)
			} else { // gone: drawn again when the preview is made again
				blank := stEmpty.Render(strings.Repeat(" ", iw))
				rows[idx] = slices.Repeat([]string{blank}, im.Rows)
			}
		}
		out = append(out, drawMdLine(l, im.Col)+rows[idx][row-im.Row])
	}
	return out
}

// mdSpanKey is the look of a span as the terminal draws it.
type mdSpanKey struct {
	fg, bg                      string
	bold, italic, under, strike bool
}

var mdSpanStyles = map[mdSpanKey]lipgloss.Style{}

// drawMdLine draws l, w cells wide: the rest of the line after its spans is
// the editor background.
func drawMdLine(l screen.Line, w int) string {
	var sb strings.Builder
	used := 0
	for _, sp := range l.Spans {
		if sp.Text == "" {
			continue
		}
		k := mdSpanKey{styleColor(sp.Style), styleColor(sp.Bg), sp.Bold, sp.Italic, sp.Underline, sp.Strike}
		if k.fg == "" {
			k.fg = cEditorFg
		}
		if k.bg == "" {
			k.bg = cEditorBg
		}
		st, ok := mdSpanStyles[k]
		if !ok {
			st = bgfg(k.bg, k.fg).Bold(k.bold).Italic(k.italic).Underline(k.under).Strikethrough(k.strike)
			mdSpanStyles[k] = st
		}
		sb.WriteString(st.Render(sp.Text))
		used += ansi.StringWidth(sp.Text)
	}
	if used < w {
		sb.WriteString(stEmpty.Render(strings.Repeat(" ", w-used)))
	}
	return sb.String()
}
