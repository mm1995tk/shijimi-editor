package core

import (
	"fmt"
	"strings"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/config"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

type tabKind int

const (
	TabText tabKind = iota
	TabImage
)

// Tab is one item in the editor area: a text buffer or an image preview.
type Tab struct {
	Kind tabKind

	Buf        *buffer.Buffer
	Hl         *highlighter
	Lang       *config.Language
	lspVersion int
	AutoVer    int  // buffer version an auto-save timer was last scheduled for
	conflict   bool // changed on disk while holding unsaved edits (already reported)

	// a transient tab (VS Code: preview) gives way to the next one until kept
	Transient    bool
	transientVer int // buffer version when opened; an edit keeps the tab

	Img *ImageView
	Dv  *DiffView // set while the tab is shown as a diff (diff mode)

	// Markdown preview beside the buffer
	Preview   PreviewMode
	prevCache *MdDoc
	PrevW     int
	prevVer   int
	prevGen   int

	// mermaid fences and image sources found at scanVer (scanned reports
	// whether that is valid)
	scanned    bool
	scanVer    int
	scanBlocks []string
	scanImages []string
}

// NewTab is a text tab holding b.
func NewTab(b *buffer.Buffer) *Tab {
	return &Tab{Kind: TabText, Buf: b, Hl: NewHighlighter(b.Path), lspVersion: -1}
}

// Title is the name the tab bar shows for t.
func (t *Tab) Title() string {
	switch t.Kind {
	case TabImage:
		return t.Img.Name()
	}
	return t.Buf.Name()
}

func (t *Tab) dirty() bool { return t.Kind == TabText && t.Buf.Dirty }

// Path is the file t shows, or "" when it has none.
func (t *Tab) Path() string {
	switch t.Kind {
	case TabText:
		return t.Buf.Path
	case TabImage:
		return t.Img.Path
	}
	return ""
}

// GutterWidthFor is the width of the line numbers column for a file of total lines.
func GutterWidthFor(total int) int { return len(fmt.Sprint(total)) + 4 }

// GutterWidth is the width of the line number gutter for b.
func GutterWidth(b *buffer.Buffer) int { return GutterWidthFor(len(b.Lines)) }

// EnsureCursorVisible adjusts scrolling so the cursor is on screen.
func EnsureCursorVisible(b *buffer.Buffer, w, h int) {
	if h <= 0 {
		return
	}
	textW := max(1, w-GutterWidth(b))
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

// EditorOpts is what App decides about one text pane: its contents (PaneOpts,
// for BuildTextPane) and how it is drawn (the rest, for drawPane).
type EditorOpts struct {
	PaneOpts
	Focused     bool
	Query       string
	CursorColor string
	Relative    bool
	GutterMark  func(row int) string // color of a ● before the line number (review comments), "" = none
}

// DiagColor is the color of a diagnostic of severity sev.
func DiagColor(sev int) string {
	return StyleColor(DiagStyle(screen.Severity(sevRank(sev))))
}

func sevRank(s int) int {
	if s == 0 {
		return 1
	}
	return s
}

// FirstLine is s up to its first line break.
func FirstLine(s string) string {
	if before, _, ok := strings.Cut(s, "\n"); ok {
		return before
	}
	return s
}

// reload re-reads the file from disk. Reload bumps the buffer version, but it
// is not a user edit, so a transient tab stays transient.
func (t *Tab) reload() error {
	if err := t.Buf.Reload(); err != nil {
		return err
	}
	t.transientVer = t.Buf.Version
	return nil
}
