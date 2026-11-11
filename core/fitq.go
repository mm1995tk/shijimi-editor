package core

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/core/internal/buffer"
)

// fitq.go is the part of fitting a frame (App.fit, app_draw.go) that changes
// what core holds: the scroll that follows the cursor, the palette's list,
// the widths of the inputs, the side bar and the proposal. App.fit works out
// the cells and hands these the numbers.

// FitCursor scrolls t, the current text tab, to the cursor if something asked
// to follow it (FollowCursor), and keeps the scroll inside the file. editW and
// editH are the cells of its text.
func (a *Core) FitCursor(t *Tab, editW, editH int) {
	if a.Follow {
		if dv := a.DiffFor(t); dv != nil {
			a.ensureDiffVisible(t, dv, editW, diffRows(editH))
		} else if !a.FullPreview(t) { // the hidden editor is 0 wide: following it would scroll to the cursor column
			EnsureCursorVisible(t.Buf, editW, editH)
		}
		a.Follow = false
	}
	t.Buf.ScrollRow = Clamp(t.Buf.ScrollRow, 0, a.maxScrollRow(t, editH))
}

// diffRows is the rows of text the diff view shows in an editor editH rows
// high: its heading takes one.
func diffRows(editH int) int { return max(0, editH-1) }

// maxScrollRow is the last ScrollRow of t, a text tab whose editor is editH
// rows high. It is the one place that says how far a text tab may scroll, and
// every way of scrolling (the wheel, the cursor, half a page, a held key, the
// first change, the change bar) is clamped by it.
//
// The scroll is the buffer's row at the top, but the diff view counts in its
// own rows (the deleted lines are rows of it), so there the last scroll is the
// one that puts the last display row on the bottom row: see
// DiffView.maxScroll.
func (a *Core) maxScrollRow(t *Tab, editH int) int {
	if dv := a.DiffFor(t); dv != nil {
		return dv.maxScroll(diffRows(editH))
	}
	return MaxScrollRow(len(t.Buf.Lines), editH)
}

// maxScrollRowOf is maxScrollRow for the buffer b of the current tab, with
// the editor's height the core was given.
func (a *Core) maxScrollRowOf(b *buffer.Buffer) int {
	if t := a.CurText(); t != nil && t.Buf == b {
		return a.maxScrollRow(t, a.EditRows())
	}
	return MaxScrollRow(len(b.Lines), a.EditRows())
}

// MaxScrollRow is the last first row of a view h rows high over n lines: the
// one that puts the last line on the bottom row, so the view never shows the
// empty space below the end of the file. A view with no rows (a hidden
// editor) may still start at the last line.
func MaxScrollRow(n, h int) int { return max(0, n-max(1, h)) }

// FitPalette filters the palette's list and sizes its input, and fits the
// project search, while they are open.
func (a *Core) FitPalette() {
	if a.Pal.Open {
		a.Pal.filter(a.Commands)
		pw := a.PaletteWidth()
		a.Pal.Input.SetWidth(max(1, pw-ansi.StringWidth(a.Pal.label())-2))
	}
	if a.Search.Open {
		a.Search.fit(a)
	}
}

// FitFields sizes the find bar's input (findCells is the cells the bar has)
// and the command line's (cmdCells), while they are open.
func (a *Core) FitFields(findCells, cmdCells int) {
	if a.FindOpen {
		a.FindInput.SetWidth(max(1, findCells-1))
	}
	if a.CmdOpen {
		a.cmdInput.SetWidth(max(1, cmdCells-3))
	}
}

// FitSide keeps the file tree's selection on screen, and fits the list of
// changes below it in the diff mode; bodyH is the height of the side bar.
func (a *Core) FitSide(bodyH int) {
	if !a.treeHidden {
		a.Explorer.EnsureVisible(a.ExplorerH() - 1)
	}
	if a.ChangesListShown() {
		a.FitChanges(bodyH - a.ExplorerH() - 1)
	}
}

// FitProposals fits the agent's proposal while there is one.
func (a *Core) FitProposals() {
	if a.props.Len() > 0 {
		a.PropView().fit(a)
	}
}

// ContextBodyRows is how many rows of text the context preview shows on a
// screen sh rows high: the box takes at most sh-4 rows (at least 4), of which a
// title row, a position row and a border are not text.
func ContextBodyRows(sh int) int { return max(1, max(4, sh-4)-3) }

// FitContext keeps the scroll of the context preview inside its text, for a
// screen sh rows high.
func (a *Core) FitContext(sh int) {
	if a.Ctx == nil {
		return
	}
	body := ContextBodyRows(sh)
	lines := len(strings.Split(strings.TrimRight(a.Ctx.Text, "\n"), "\n"))
	a.Ctx.Scroll = Clamp(a.Ctx.Scroll, 0, max(0, lines-body))
}
