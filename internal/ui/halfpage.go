package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
)

// Ctrl+D / Ctrl+U move half a page. A jump makes it easy to lose the line one
// was reading, so:
//   - When the terminal reports key releases (kitty keyboard protocol), the
//     keys scroll like the mouse wheel: line by line while held, stopping as
//     soon as the key is let go.
//   - Otherwise the cursor jumps half a page and its line is put in the middle
//     of the screen, so the eye always looks at the same place.

const holdScrollFrame = 16 * time.Millisecond // one line per frame

type holdScroll struct {
	buf  *buffer.Buffer // nil when not scrolling
	dir  int            // +1 down, -1 up
	ext  bool           // extend the selection (select mode)
	code rune           // the held key: 'd' or 'u'
}

type holdScrollMsg struct{ seq int }

// halfPage handles Ctrl+D (dir = 1) and Ctrl+U (dir = -1).
func (a *App) halfPage(b *buffer.Buffer, dir int, ext bool, msg tea.KeyPressMsg) tea.Cmd {
	if !a.keyReleases {
		b.MoveLines(dir*a.pageSize()/2, ext)
		b.ScrollRow = max(0, b.Cur.Row-a.layout().editH/2)
		return nil
	}
	if msg.IsRepeat || a.hold.buf != nil && a.hold.dir == dir {
		return nil // already scrolling until the release
	}
	a.hold = holdScroll{buf: b, dir: dir, ext: ext, code: msg.Code}
	a.holdSeq++
	return a.holdScrollTick(a.holdSeq)
}

// holdScrollTick moves the cursor and the view one line together, so the
// cursor stays on its screen row while the text flows past it.
func (a *App) holdScrollTick(seq int) tea.Cmd {
	h := a.hold
	if h.buf == nil || seq != a.holdSeq {
		return nil
	}
	row := h.buf.Cur.Row
	h.buf.MoveLines(h.dir, h.ext)
	if h.buf.Cur.Row == row { // the start or end of the file
		a.stopHoldScroll()
		return nil
	}
	h.buf.ScrollRow = clamp(h.buf.ScrollRow+h.dir, 0, max(0, len(h.buf.Lines)-1))
	a.follow = true
	return tea.Tick(holdScrollFrame, func(time.Time) tea.Msg { return holdScrollMsg{seq} })
}

func (a *App) stopHoldScroll() { a.hold = holdScroll{} }

// holdScrollRelease stops the scroll when the held letter is let go. The
// release of D or U ends it even if Ctrl was let go first.
func (a *App) holdScrollRelease(msg tea.KeyReleaseMsg) {
	if a.hold.buf != nil && msg.Code == a.hold.code {
		a.stopHoldScroll()
	}
}

// wantKeyReleases reports whether Ctrl+D / Ctrl+U would scroll now: the Helix
// normal or select mode in the editor, with nothing open on top. Text is not
// typed there, so losing IME input to key-release reporting does no harm.
func (a *App) wantKeyReleases() bool {
	return a.helix && a.mode != modeInsert && a.focus == focusEditor && a.curText() != nil &&
		!a.overlayOpen() && !a.cmdOpen && !a.findOpen && len(a.props) == 0 && a.ctx == nil
}
