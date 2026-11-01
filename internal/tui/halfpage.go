package tui

import (
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
)

// Ctrl+D / Ctrl+U move half a page. A jump makes it easy to lose the line one
// was reading, so:
//   - When the terminal reports key releases (kitty keyboard protocol), the
//     keys scroll like the mouse wheel: line by line while held, stopping as
//     soon as the key is let go.
//   - Otherwise the cursor jumps half a page and its line is put in the middle
//     of the screen, so the eye always looks at the same place.
//
// Ctrl+Alt+D / Ctrl+Alt+U scroll the same way, but only the view: the cursor
// stays on its line until the screen edge pushes it.

const holdScrollFrame = 16 * time.Millisecond // one line per frame

type holdScroll struct {
	buf  *buffer.Buffer // nil when not scrolling
	dir  int            // +1 down, -1 up
	ext  bool           // extend the selection (select mode)
	code rune           // the held key: 'd' or 'u'
	keep bool           // Ctrl+Alt: scroll the view only, the cursor stays on its line
}

// continues reports whether a key press is the held key itself (or its repeat),
// so the running scroll goes on. Ctrl+D and Ctrl+Alt+D share a code but are
// different scrolls.
func (h holdScroll) continues(msg Key) bool {
	return h.buf != nil && msg.Code == h.code && msg.Mod.Has(ModCtrl) && msg.Mod.Has(ModAlt) == h.keep
}

// halfPage handles Ctrl+D (dir = 1) and Ctrl+U (dir = -1), and with Alt added
// (Ctrl+Alt+D / Ctrl+Alt+U) the same scroll that leaves the cursor on its line.
func (a *App) halfPage(b *buffer.Buffer, dir int, ext bool, msg Key) []Effect {
	keep := msg.Mod.Has(ModAlt)
	if !a.keyReleases { // one press, half a page
		if keep {
			a.scrollKeepCursor(b, dir*a.pageSize()/2, ext)
		} else {
			b.MoveLines(dir*a.pageSize()/2, ext)
			b.ScrollRow = max(0, b.Cur.Row-a.layout().editH/2)
		}
		return nil
	}
	if msg.Repeat || a.hold.continues(msg) {
		return nil // already scrolling until the release
	}
	a.hold = holdScroll{buf: b, dir: dir, ext: ext, code: msg.Code, keep: keep}
	a.holdSeq++
	return a.holdScrollTick(a.holdSeq)
}

// holdScrollTick moves the cursor and the view one line together, so the
// cursor stays on its screen row while the text flows past it.
func (a *App) holdScrollTick(seq int) []Effect {
	h := a.hold
	if h.buf == nil || seq != a.holdSeq {
		return nil
	}
	if h.keep {
		if !a.scrollKeepCursor(h.buf, h.dir, h.ext) { // the start or end of the file
			a.stopHoldScroll()
			return nil
		}
		return []Effect{StartTimer{After: holdScrollFrame, Fire: HoldScrollTick{seq}}}
	}
	row := h.buf.Cur.Row
	h.buf.MoveLines(h.dir, h.ext)
	if h.buf.Cur.Row == row { // the start or end of the file
		a.stopHoldScroll()
		return nil
	}
	h.buf.ScrollRow = clamp(h.buf.ScrollRow+h.dir, 0, max(0, len(h.buf.Lines)-1))
	a.follow = true
	return []Effect{StartTimer{After: holdScrollFrame, Fire: HoldScrollTick{seq}}}
}

func (a *App) stopHoldScroll() { a.hold = holdScroll{} }

// holdScrollRelease stops the scroll when the held letter is let go. The
// release of D or U ends it even if Ctrl was let go first.
func (a *App) holdScrollRelease(msg Key) {
	if a.hold.buf != nil && msg.Code == a.hold.code {
		a.stopHoldScroll()
	}
}

// wantKeyReleases reports whether Ctrl+D / Ctrl+U would scroll now: the Helix
// normal or select mode in the editor, with nothing open on top. Text is not
// typed there, so losing IME input to key-release reporting does no harm.
func (a *App) wantKeyReleases() bool {
	return a.helix && a.mode != modeInsert && a.focus == focusEditor && a.curText() != nil &&
		!a.overlayOpen() && !a.cmdOpen && !a.findOpen && a.props.Len() == 0 && a.ctx == nil
}

// scrollKeepCursor scrolls the view by n lines (Ctrl+Alt+D / Ctrl+Alt+U) and
// leaves the cursor on its line. Once the line would leave the screen (within
// the same margin that keeps the cursor in view) the cursor is pushed along
// the edge. It reports whether the view moved; the view stops when the first
// or the last line is at the edge of the screen.
func (a *App) scrollKeepCursor(b *buffer.Buffer, n int, ext bool) bool {
	h := a.layout().editH
	if h <= 0 {
		return false
	}
	old := b.ScrollRow
	b.ScrollRow = clamp(old+n, 0, max(0, len(b.Lines)-h))
	moved := b.ScrollRow != old
	margin := min(3, (h-1)/2)
	lo, hi := b.ScrollRow+margin, b.ScrollRow+h-1-margin
	if b.ScrollRow == 0 {
		lo = 0 // the top of the file has no line above to keep in view
	}
	if b.ScrollRow == max(0, len(b.Lines)-h) {
		hi = len(b.Lines) - 1 // nor does the bottom have a line below
	}
	row := clamp(b.Cur.Row, lo, max(lo, hi))
	if row != b.Cur.Row {
		b.MoveLines(row-b.Cur.Row, ext)
	}
	a.follow = false // the view must not be pulled back to the cursor
	return moved
}
