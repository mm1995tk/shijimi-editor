package core

import (
	"sort"
	"strings"

	"github.com/mm1995tk/shijimi-editor/core/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/core/msg"
)

// diffleft.go is the cursor on the left side of diff mode, the base (HEAD or
// a pull request's base): it moves, selects and copies there as on the right
// side, and cannot change anything.
//
// The left side is a read-only tab of its own (DiffView.Left, made when the
// cursor first goes there), holding the base as the diff has it, so the cursor,
// the selection, the search and the keys of both modes work on it as they do
// on any buffer. While a key is handled with the cursor on the left (keyOnLeft)
// Cur gives that tab in place of the file's, so what the key does acts on it;
// everything else (drawing, saving, the review, the commands of Space) keeps to
// the file's tab. The two sides scroll together: the view is the file's
// buffer scroll (and sub, below), and the left tab's scroll is made from it
// before a key and written back after it.

// DiffLeft returns the left side of the diff of t when the cursor is there, or
// nil: the cursor is on the right side, or t has no left side (it is not a
// diff, the diff is unified, or the file is new).
func (a *Core) DiffLeft(t *Tab) *Tab {
	if !a.Gd.Left || !a.Gd.Split || t == nil {
		return nil
	}
	f := a.DiffFile(t)
	if f == nil || a.DiffSingle(f) || !a.diffBaseFresh(t.Dv, f) {
		return nil
	}
	return t.Dv.Left
}

// newLeftTab is the left side of the diff dv of t: the base as a read-only
// tab, with the cursor on the line the cursor of t is level with.
func newLeftTab(dv *DiffView, t *Tab) *Tab {
	l := NewTab(buffer.NewReadOnly(t.Buf.Path, strings.Join(dv.old, "\n")))
	l.leftOf = t
	l.Buf.UseTabs = t.Buf.UseTabs
	if len(dv.NewIdx) > 0 {
		r := dv.NewIdx[min(t.Buf.Cur.Row, len(dv.NewIdx)-1)]
		l.Buf.Cur = l.Buf.ClampPos(buffer.Pos{Row: dv.NearestOld(r)})
	}
	return l
}

// NearestOld is the line of the base that is on display row r, or the next
// one below it (the one above when there is none).
func (dv *DiffView) NearestOld(r int) int {
	r = Clamp(r, 0, len(dv.Rows)-1)
	for i := r; i < len(dv.Rows); i++ {
		if o := dv.Rows[i].old; o >= 0 {
			return o
		}
	}
	for i := r; i >= 0; i-- {
		if o := dv.Rows[i].old; o >= 0 {
			return o
		}
	}
	return 0
}

// HasNew reports whether display row r has a line on the working tree side.
func (dv *DiffView) HasNew(r int) bool { return r >= 0 && r < len(dv.Rows) && dv.Rows[r].New >= 0 }

// HasOld reports whether display row r has a line on the base side.
func (dv *DiffView) HasOld(r int) bool { return r >= 0 && r < len(dv.Rows) && dv.Rows[r].old >= 0 }

// SetDiffSide moves the cursor to the left side (left) or the right side of
// the diff of the current file, and reports whether it can: there is a left
// side only in the side by side diff of a file that is in the base.
func (a *Core) SetDiffSide(left bool) bool {
	t := a.CurText()
	f := a.DiffFile(t)
	if f == nil {
		return false
	}
	if left && (!a.Gd.Split || a.DiffSingle(f)) {
		a.InfoText(msg.T("gd.noleft"))
		return false
	}
	if a.Gd.Left != left {
		a.Mode = ModeNormal
		a.Comp = nil
		a.Follow = true
	}
	a.Gd.Left = left
	a.DiffFor(t)
	return true
}

// FocusDiffSide is SetDiffSide for the keys: it puts the focus on the editor
// too.
func (a *Core) FocusDiffSide(left bool) []Effect {
	if !a.SetDiffSide(left) {
		return nil
	}
	return a.SetFocus(FocusEditor)
}

// keyOnLeft handles in with the cursor on the left side l of the diff of t.
func (a *Core) keyOnLeft(t, l *Tab, in Key) []Effect {
	dv := t.Dv
	l.Buf.ScrollCol = t.Buf.ScrollCol
	l.Buf.ScrollRow = dv.leftTop(t.Buf.ScrollRow)
	top := l.Buf.ScrollRow
	a.onLeft = true
	effs := a.dispatchEditorKey(in)
	a.onLeft = false
	t.Buf.ScrollCol = l.Buf.ScrollCol
	if l.Buf.ScrollRow != top && len(dv.OldIdx) > 0 { // a key that scrolls (Ctrl+D)
		dv.setTop(t.Buf, dv.OldIdx[Clamp(l.Buf.ScrollRow, 0, len(dv.OldIdx)-1)], diffRows(a.EditRows()))
	}
	return effs
}

// leftTop is the first line of the base on screen when the view starts at
// buffer line scrollRow.
func (dv *DiffView) leftTop(scrollRow int) int {
	for i := dv.Start(scrollRow); i < len(dv.Rows); i++ {
		if o := dv.Rows[i].old; o >= 0 {
			return o
		}
	}
	return max(0, len(dv.old)-1)
}

// scrollAtMost is the last buffer scroll whose first display row is at or
// above display row r.
func (dv *DiffView) scrollAtMost(r int) int {
	return max(0, sort.Search(len(dv.NewIdx), func(i int) bool { return dv.NewIdx[i] > r })-1)
}

// setTop scrolls b, the buffer of the file, so that display row top is the
// first row of a view h rows high. The view starts at a buffer line, so rows
// below it that exist only on the base side (a long deletion) are reached by
// starting sub rows under that line.
func (dv *DiffView) setTop(b *buffer.Buffer, top, h int) {
	top = Clamp(top, 0, max(0, len(dv.Rows)-max(1, h)))
	i := dv.scrollAtMost(top)
	base := 0
	if i > 0 {
		base = dv.NewIdx[i]
	}
	b.ScrollRow = i
	dv.subRow, dv.sub = i, top-base
}

// ensureLeftVisible scrolls the view (the rows of the file's buffer and
// the columns both sides share) so that the cursor of the left side l is on
// screen, h rows high.
func (a *Core) ensureLeftVisible(t *Tab, dv *DiffView, l *Tab, h int) {
	lb := l.Buf
	lb.ScrollCol = t.Buf.ScrollCol
	EnsureCursorVisible(lb, a.DiffBaseCols(), h)
	t.Buf.ScrollCol = lb.ScrollCol
	if h <= 0 || len(dv.OldIdx) == 0 {
		return
	}
	margin := min(3, (h-1)/2)
	r := dv.OldIdx[Clamp(lb.Cur.Row, 0, len(dv.OldIdx)-1)]
	top := dv.Start(t.Buf.ScrollRow)
	switch {
	case r < top+margin:
		top = max(0, r-margin)
	case r >= top+h-margin:
		top = r - h + margin + 1
	default:
		return
	}
	dv.setTop(t.Buf, top, h)
}

// sideTab is the tab the cursor is in: the left side of the diff when the
// cursor is there, otherwise Cur.
func (a *Core) sideTab() *Tab {
	t := a.Cur()
	if l := a.DiffLeft(t); l != nil {
		return l
	}
	return t
}
