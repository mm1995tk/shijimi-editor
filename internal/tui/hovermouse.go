package tui

import (
	"context"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
)

// hoverDelay is how long the pointer must rest on a word before its hover shows.
const hoverDelay = 500 * time.Millisecond

// screenPt is a screen cell.
type screenPt struct{ x, y int }

// wordSpan is the word under the pointer: columns [start, end) of one row. The
// zero value is no word.
type wordSpan struct{ row, start, end int }

// mouseHover says which pointer position a hover reply belongs to.
type mouseHover struct {
	tab  *tab
	seq  int
	pos  buffer.Pos
	word wordSpan
}

// MouseHover reports whether resting the pointer on a word shows its hover.
func (a *App) MouseHover() bool { return a.cfg.Editor.MouseHover }

// hoverBlocked reports whether something that owns the keys is open (palette,
// project search, context menu, command line, resize mode, agent proposal), so
// a popup must not appear over it.
func (a *App) hoverBlocked() bool {
	return a.ctx != nil || a.overlayOpen() || a.cmdOpen || a.resizeMode || a.props.Len() > 0 || a.composer != nil || a.confirm != nil
}

// hoverDwell asks the language server about the word under a resting pointer.
func (a *App) hoverDwell(msg HoverDwell) []Effect {
	if msg.seq != a.hoverSeq || a.focus != focusEditor || a.hoverBlocked() {
		return nil
	}
	pos, word, ok := a.hoverCellWord()
	if !ok {
		return nil
	}
	t := a.cur()
	if t == nil || t.kind != tabText || t.lang == nil {
		return nil
	}
	c := a.lsp.client(t, "hoverProvider")
	if c == nil {
		return nil // no server yet: stay silent, the pointer did not ask for an error
	}
	a.syncLSP()
	path, lp := t.buf.Path, lspPos(t.buf, pos)
	origin := &mouseHover{tab: t, seq: msg.seq, pos: pos, word: word}
	return []Effect{RunLSP{func(ctx context.Context) Event {
		text, err := c.Hover(ctx, path, lp)
		if err != nil {
			return nil
		}
		return hoverMsg{text: text, mouse: origin}
	}}}
}

// popupAnchorPos is the place in the file a popup hangs from.
func (a *App) popupAnchorPos(t *tab) buffer.Pos {
	if a.comp == nil && a.hoverAt != nil {
		// The file may have shrunk (a reload) since the pointer opened it.
		row := clamp(a.hoverAt.Row, 0, len(t.buf.Lines)-1)
		return buffer.Pos{Row: row, Col: clamp(a.hoverAt.Col, 0, len(t.buf.Lines[row]))}
	}
	return t.buf.Cur
}

