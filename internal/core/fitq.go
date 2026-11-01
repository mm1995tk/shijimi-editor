package core

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// fitq.go is the part of fitting a frame (App.fit, app_draw.go) that changes
// what core holds: the scroll that follows the cursor, the palette's list,
// the widths of the inputs, the side bar and the proposal. App.fit works out
// the cells and hands these the numbers.

// fitCursor scrolls t, the current text tab, to the cursor if something asked
// to follow it (followCursor), and keeps the scroll inside the file. editW and
// editH are the cells of its text; dvH is the rows of its diff.
func (a *Core) FitCursor(t *Tab, editW, editH, dvH int) {
	if a.follow {
		if dv := a.DiffFor(t); dv != nil {
			a.ensureDiffVisible(t, dv, editW, dvH)
		} else if !a.FullPreview(t) { // the hidden editor is 0 wide: following it would scroll to the cursor column
			EnsureCursorVisible(t.Buf, editW, editH)
		}
		a.follow = false
	}
	t.Buf.ScrollRow = Clamp(t.Buf.ScrollRow, 0, max(0, len(t.Buf.Lines)-1))
}

// fitPalette filters the palette's list and sizes its input, and fits the
// project search, while they are open.
func (a *Core) FitPalette() {
	if a.pal.open {
		a.pal.filter(a.commands)
		pw := a.PaletteWidth()
		a.pal.input.SetWidth(max(1, pw-ansi.StringWidth(a.pal.label())-2))
	}
	if a.search.open {
		a.search.fit(a)
	}
}

// fitFields sizes the find bar's input (findCells is the cells the bar has)
// and the command line's (cmdCells), while they are open.
func (a *Core) FitFields(findCells, cmdCells int) {
	if a.findOpen {
		a.findInput.SetWidth(max(1, findCells-1))
	}
	if a.cmdOpen {
		a.cmdInput.SetWidth(max(1, cmdCells-3))
	}
}

// fitSide keeps the file tree's selection on screen, and fits the list of
// changes below it in the diff mode; bodyH is the height of the side bar.
func (a *Core) FitSide(bodyH int) {
	a.explorer.ensureVisible(a.ExplorerH() - 1)
	if a.gd.on {
		a.fitChanges(bodyH - a.ExplorerH() - 1)
	}
}

// fitProposals fits the agent's proposal while there is one.
func (a *Core) FitProposals() {
	if a.props.Len() > 0 {
		a.PropView().fit(a)
	}
}

// fitContext keeps the scroll of the context preview inside its text, for a
// screen sh rows high (the box has a title row, a position row and a border).
func (a *Core) FitContext(sh int) {
	if a.ctx == nil {
		return
	}
	body := max(1, max(4, sh-4)-3)
	lines := len(strings.Split(strings.TrimRight(a.ctx.Text, "\n"), "\n"))
	a.ctx.Scroll = Clamp(a.ctx.Scroll, 0, max(0, lines-body))
}
