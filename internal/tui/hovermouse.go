package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

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

// passiveMotion handles pointer movement with no button held. It never moves
// the cursor: it only starts the dwell timer, or closes a popup the pointer
// left.
func (a *App) passiveMotion(l layout, msg Mouse) tea.Cmd {
	if cmd, ok := a.termMouse(l, msg); ok {
		return a.effectCmd(cmd...)
	}
	if th := a.markerAt(l, msg.X, msg.Y); th != nil && a.cfg.Editor.MouseHover && a.focus == focusEditor && !a.hoverBlocked() && a.resizing == resizeNone {
		// resting on a comment marker shows its thread
		if a.card == nil || a.card.thread != th {
			a.card = &prCard{thread: th, path: a.repoPath(a.curText().buf.Path), byPointer: true, at: &screenPt{msg.X, msg.Y}}
			a.hoverText, a.hoverAt = "", nil
		}
		return nil
	}
	if c := a.card; c != nil && c.byPointer {
		// like a hover, it stays while the pointer reads it, and closes when it leaves
		if box, bx, by := a.cardBox(l); box == nil || msg.X < bx || msg.Y < by || msg.Y >= by+len(box) || msg.X >= bx+ansi.StringWidth(box[0]) {
			a.card = nil
		}
	}
	if !a.cfg.Editor.MouseHover || a.hoverBlocked() || a.resizing != resizeNone {
		return nil
	}
	if a.focus != focusEditor {
		// The keys belong to another pane; a popup must not open over them.
		if a.hoverAt != nil {
			a.hoverText, a.hoverAt = "", nil
		}
		return nil
	}
	pt := screenPt{msg.X, msg.Y}
	if pt == a.hoverCell {
		return nil
	}
	a.hoverCell = pt
	_, word, ok := a.mouseWord(l, pt)
	if a.hoverText != "" && a.inHoverBox(l, pt) {
		return nil // reading it (the wheel scrolls it); whatever hides below is not asked about
	}
	if a.hoverText != "" && a.hoverAt != nil {
		// A pointer popup also stays while the pointer is on the word it describes.
		if ok && word == a.hoverWord {
			return nil
		}
		a.hoverText, a.hoverAt = "", nil
	} else if ok && word == a.hoverWord {
		return nil // the timer for this word is already running
	}
	a.hoverSeq++
	a.hoverWord = wordSpan{}
	if !ok {
		return nil
	}
	a.hoverWord = word
	seq := a.hoverSeq
	return a.effectCmd(StartTimer{After: hoverDelay, Fire: HoverDwell{seq}})
}

// hoverBlocked reports whether something that owns the keys is open (palette,
// project search, context menu, command line, resize mode, agent proposal), so
// a popup must not appear over it.
func (a *App) hoverBlocked() bool {
	return a.ctx != nil || a.overlayOpen() || a.cmdOpen || a.resizeMode || len(a.props) > 0 || a.composer != nil || a.confirm != nil
}

// hoverDwell asks the language server about the word under a resting pointer.
func (a *App) hoverDwell(msg HoverDwell) []Effect {
	if msg.seq != a.hoverSeq || a.focus != focusEditor || a.hoverBlocked() {
		return nil
	}
	pos, word, ok := a.mouseWord(a.layout(), a.hoverCell)
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

// mouseBufPos maps a screen cell to the buffer position of the character under
// it. ok is false outside the text of a plain editor, including past the end
// of a line.
func (a *App) mouseBufPos(l layout, x, y int) (buffer.Pos, bool) {
	t := a.curText()
	if t == nil || a.fullPreview(t) || a.diffFor(t) != nil {
		return buffer.Pos{}, false
	}
	b := t.buf
	gw := gutterWidth(b)
	if y < l.editY || y >= l.editY+l.editH || x < l.rightX+gw || x >= l.rightX+a.editW(t, l) {
		return buffer.Pos{}, false
	}
	row := b.ScrollRow + y - l.editY
	if row >= len(b.Lines) {
		return buffer.Pos{}, false
	}
	line := b.Lines[row]
	vis := x - l.rightX - gw + b.ScrollCol
	if vis >= buffer.VisualCol(line, len(line)) {
		return buffer.Pos{}, false
	}
	return buffer.Pos{Row: row, Col: buffer.ColFromVisual(line, vis)}, true
}

// mouseWord is mouseBufPos plus the word the character belongs to.
func (a *App) mouseWord(l layout, pt screenPt) (buffer.Pos, wordSpan, bool) {
	pos, ok := a.mouseBufPos(l, pt.x, pt.y)
	if !ok {
		return pos, wordSpan{}, false
	}
	s, e := buffer.WordRange(a.curText().buf.Lines[pos.Row], pos.Col)
	return pos, wordSpan{pos.Row, s, e}, true
}

// popupAnchor is the cell a popup hangs from: the pointer for a hover it
// triggered, the cursor otherwise.
func (a *App) popupAnchor(t *tab, l layout) (x, y int) {
	return a.posScreen(t, l, a.popupAnchorPos(t))
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

// inHoverBox reports whether pt is on the open hover popup.
func (a *App) inHoverBox(l layout, pt screenPt) bool {
	x, y, ok := a.hoverPos(l)
	if !ok {
		return false
	}
	box := a.hoverBox(l)
	bw := 0
	for _, s := range box {
		bw = max(bw, ansi.StringWidth(s))
	}
	return pt.x >= x && pt.x < x+bw && pt.y >= y && pt.y < y+len(box)
}
