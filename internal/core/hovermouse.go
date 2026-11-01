package core

import (
	"context"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
)

// hoverDelay is how long the pointer must rest on a word before its hover shows.
const hoverDelay = 500 * time.Millisecond

// screenPt is a screen cell.
type ScreenPt struct{ X, Y int }

// wordSpan is the word under the pointer: columns [start, end) of one row. The
// zero value is no word.
type WordSpan struct{ row, start, end int }

// mouseHover says which pointer position a hover reply belongs to.
type mouseHover struct {
	tab  *Tab
	seq  int
	pos  buffer.Pos
	word WordSpan
}

// MouseHover reports whether resting the pointer on a word shows its hover.
func (a *Core) MouseHover() bool { return a.cfg.Editor.MouseHover }

// hoverBlocked reports whether something that owns the keys is open (palette,
// project search, context menu, command line, resize mode, agent proposal), so
// a popup must not appear over it.
func (a *Core) HoverBlocked() bool {
	return a.ctx != nil || a.overlayOpen() || a.cmdOpen || a.InResizeMode() || a.props.Len() > 0 || a.composer != nil || a.confirm != nil
}

// hoverDwell asks the language server about the word under a resting pointer.
func (a *Core) hoverDwell(msg HoverDwell) []Effect {
	if msg.seq != a.hoverSeq || a.focus != FocusEditor || a.HoverBlocked() {
		return nil
	}
	pos, word, ok := a.HoverCellWord()
	if !ok {
		return nil
	}
	t := a.Cur()
	if t == nil || t.Kind != TabText || t.lang == nil {
		return nil
	}
	c := a.lsp.client(t, "hoverProvider")
	if c == nil {
		return nil // no server yet: stay silent, the pointer did not ask for an error
	}
	a.SyncLSP()
	path, lp := t.Buf.Path, lspPos(t.Buf, pos)
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
func (a *Core) PopupAnchorPos(t *Tab) buffer.Pos {
	if a.comp == nil && a.hoverAt != nil {
		// The file may have shrunk (a reload) since the pointer opened it.
		row := Clamp(a.hoverAt.Row, 0, len(t.Buf.Lines)-1)
		return buffer.Pos{Row: row, Col: Clamp(a.hoverAt.Col, 0, len(t.Buf.Lines[row]))}
	}
	return t.Buf.Cur
}

// NewWordSpan is the word of line row from column start up to end.
func NewWordSpan(row, start, end int) WordSpan { return WordSpan{row, start, end} }
