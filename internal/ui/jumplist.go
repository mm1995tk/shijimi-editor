package ui

import (
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
)

// maxJumps caps the jump list.
const maxJumps = 100

// jumpLoc is a file and cursor position in the jump list.
type jumpLoc struct {
	path     string
	row, col int
}

// curLoc returns where the cursor is now; ok is false when the active tab has
// no file position (no tab, an image, a review, an unsaved scratch buffer).
func (a *App) curLoc() (jumpLoc, bool) {
	t := a.curText()
	if t == nil || t.buf.Path == "" {
		return jumpLoc{}, false
	}
	return jumpLoc{t.buf.Path, t.buf.Cur.Row, t.buf.Cur.Col}, true
}

// pushJump records the current position before a far jump. Going somewhere
// new drops the "forward" part of the list; a position equal to the last one
// is not added twice.
func (a *App) pushJump() {
	l, ok := a.curLoc()
	if !ok {
		return
	}
	a.jumps = a.jumps[:min(a.jumpIdx, len(a.jumps))]
	if n := len(a.jumps); n == 0 || a.jumps[n-1] != l {
		a.jumps = append(a.jumps, l)
	}
	if len(a.jumps) > maxJumps {
		a.jumps = a.jumps[len(a.jumps)-maxJumps:]
	}
	a.jumpIdx = len(a.jumps)
}

// jumpable reports whether the file of l can still be shown.
func (a *App) jumpable(l jumpLoc) bool {
	for _, t := range a.tabs {
		if t.kind == tabText && t.buf.Path == l.path {
			return true
		}
	}
	st, err := os.Stat(l.path)
	return err == nil && !st.IsDir()
}

// jumpMove goes back (dir -1) or forward (dir +1) in the jump list, reopening
// closed files and skipping entries whose file is gone.
func (a *App) jumpMove(dir int) tea.Cmd {
	i := a.jumpIdx
	if dir < 0 && i >= len(a.jumps) {
		// Leaving the newest position: remember it so that "forward" returns.
		if _, ok := a.curLoc(); ok {
			a.pushJump()
			i = len(a.jumps) - 1
		} else {
			i = len(a.jumps)
		}
	}
	for {
		i += dir
		if i < 0 || i >= len(a.jumps) {
			if dir < 0 {
				a.info("これ以上戻れません")
			} else {
				a.info("これ以上進めません")
			}
			return nil
		}
		l := a.jumps[i]
		if !a.jumpable(l) || !a.openFile(l.path) {
			a.jumps = append(a.jumps[:i], a.jumps[i+1:]...)
			if dir > 0 {
				i--
			}
			continue
		}
		t := a.curText()
		if t == nil {
			return nil
		}
		b := t.buf
		b.Anchor = nil
		row := min(max(l.row, 0), len(b.Lines)-1)
		b.SetCursor(row, buffer.VisualCol(b.Lines[row], l.col), false)
		a.jumpIdx = i
		a.follow = true
		return a.setFocus(focusEditor)
	}
}
