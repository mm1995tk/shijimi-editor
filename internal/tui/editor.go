package tui

import (
	"fmt"
	"strings"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/config"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

type tabKind int

const (
	tabText tabKind = iota
	tabImage
)

// tab is one item in the editor area: a text buffer or an image preview.
type tab struct {
	kind tabKind

	buf        *buffer.Buffer
	hl         *highlighter
	lang       *config.Language
	lspVersion int
	autoVer    int  // buffer version an auto-save timer was last scheduled for
	conflict   bool // changed on disk while holding unsaved edits (already reported)

	// a transient tab (VS Code: preview) gives way to the next one until kept
	transient    bool
	transientVer int // buffer version when opened; an edit keeps the tab

	img *imageView
	dv  *diffView // set while the tab is shown as a diff (diff mode)

	// Markdown preview beside the buffer
	preview   previewMode
	prevCache *mdDoc
	prevW     int
	prevVer   int
	prevGen   int

	// mermaid fences and image sources found at scanVer (scanned reports
	// whether that is valid)
	scanned    bool
	scanVer    int
	scanBlocks []string
	scanImages []string
}

func newTab(b *buffer.Buffer) *tab {
	return &tab{kind: tabText, buf: b, hl: newHighlighter(b.Path), lspVersion: -1}
}

func (t *tab) title() string {
	switch t.kind {
	case tabImage:
		return t.img.name()
	}
	return t.buf.Name()
}

func (t *tab) dirty() bool { return t.kind == tabText && t.buf.Dirty }

func (t *tab) path() string {
	switch t.kind {
	case tabText:
		return t.buf.Path
	case tabImage:
		return t.img.path
	}
	return ""
}

// gutterWidthFor is the width of the line numbers column for a file of total lines.
func gutterWidthFor(total int) int { return len(fmt.Sprint(total)) + 4 }

func gutterWidth(b *buffer.Buffer) int { return gutterWidthFor(len(b.Lines)) }

// ensureCursorVisible adjusts scrolling so the cursor is on screen.
func ensureCursorVisible(b *buffer.Buffer, w, h int) {
	if h <= 0 {
		return
	}
	textW := max(1, w-gutterWidth(b))
	margin := min(3, (h-1)/2)
	if b.Cur.Row < b.ScrollRow+margin {
		b.ScrollRow = max(0, b.Cur.Row-margin)
	}
	if b.Cur.Row >= b.ScrollRow+h-margin {
		b.ScrollRow = b.Cur.Row - h + margin + 1
	}
	vis := buffer.VisualCol(b.Lines[b.Cur.Row], b.Cur.Col)
	if vis < b.ScrollCol {
		b.ScrollCol = max(0, vis-textW/4)
	}
	if vis >= b.ScrollCol+textW {
		b.ScrollCol = vis - textW + 1 + textW/4
	}
}

// editorOpts is what App decides about one text pane: its contents (paneOpts,
// for buildTextPane) and how it is drawn (the rest, for drawPane).
type editorOpts struct {
	paneOpts
	focused     bool
	query       string
	cursorColor string
	relative    bool
	gutterMark  func(row int) string // color of a ● before the line number (review comments), "" = none
}

// diagColor is the color of a diagnostic of severity sev.
func diagColor(sev int) string {
	return styleColor(diagStyle(screen.Severity(sevRank(sev))))
}

// renderEditor draws the visible part of a buffer as exactly h lines of
// width w: the Pane of the buffer (buildTextPane), drawn as cells.
func renderEditor(t *tab, w, h int, o editorOpts) []string {
	return drawTextPane(buildTextPane(t, w, h, o.paneOpts), w, h, o)
}

func sevRank(s int) int {
	if s == 0 {
		return 1
	}
	return s
}

func firstLine(s string) string {
	if before, _, ok := strings.Cut(s, "\n"); ok {
		return before
	}
	return s
}

// reload re-reads the file from disk. Reload bumps the buffer version, but it
// is not a user edit, so a transient tab stays transient.
func (t *tab) reload() error {
	if err := t.buf.Reload(); err != nil {
		return err
	}
	t.transientVer = t.buf.Version
	return nil
}
