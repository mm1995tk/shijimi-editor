package tui

import (
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// mouse.go is where a pointer on the screen is worked out: which cell it is
// over, what is drawn there (a popup, a border, the terminal, a text
// column), and what that means for the state. The hit tests need the layout,
// so they stay in the TUI; the state they change (the cursor, a hover, a
// selection) is the core's, reached through the same methods the keys use.
// Mouse arrives as a Region plus a row and column in it (see Mouse).

// termMouse passes presses, drags and releases over the terminal body to a
// program that asked for mouse input (vim, htop, tmux). Once a press went to
// the program, its drag and release do too, wherever the pointer is. Otherwise
// (or with Shift held) a left drag selects text and copies it on release. ok
// is false when the event is not for the terminal.
func (a *App) termMouse(l layout, msg core.Mouse) (cmd []core.Effect, ok bool) {
	t := a.term
	if !a.showTerm {
		return nil, false
	}
	x, y := msg.Col, msg.Row
	if t.sel.dragging {
		switch msg.Action {
		case core.MouseMotion:
			t.dragTo(x, y)
		case core.MouseRelease:
			t.dragTo(x, y)
			if s := t.endSelect(); s != "" {
				a.CopyText(s)
				a.Info(i18n.T("app.termcopied"), utf8.RuneCountInString(s))
			}
		default:
			t.sel.dragging = false // the release was lost
			return nil, false
		}
		return nil, true
	}
	inBody := msg.Region == core.RegionTerminal && x >= 0 && x < l.fullW && y >= 0
	grabbed := t.grab
	if !grabbed && !inBody {
		return nil, false
	}
	sent := false
	if grabbed || !msg.Shift {
		sent = t.mouse(msg, x, y)
	}
	switch msg.Action {
	case core.MousePress:
		if sent {
			t.sel = termSel{}
			t.grab = true
			return a.SetFocus(core.FocusTerminal), true
		}
		if msg.Button == core.MouseLeft {
			t.selectAt(x, y)
			return a.SetFocus(core.FocusTerminal), true
		}
	case core.MouseRelease:
		t.grab = false
	}
	return nil, sent || grabbed
}

// HandleMouse handles a pointer event.
func (a *App) HandleMouse(msg core.Mouse) []core.Effect {
	if a.SearchOpen() {
		return a.searchMouse(msg)
	}
	if a.PaletteOpen() {
		if msg.Action == core.MousePress && msg.Button == core.MouseLeft {
			a.ClosePalette()
		}
		return nil
	}
	l := a.layout()
	// A press or the wheel over the other pane first moves the focus there.
	if msg.Region == core.RegionOther && msg.Action == core.MousePress && (msg.Button == core.MouseLeft || msg.Button == core.MouseWheelUp || msg.Button == core.MouseWheelDown) {
		a.FocusPane(a.OtherPane())
		l = a.layout()
		// The pane under the pointer is the focused one now. Its corner has not
		// moved, so Row and Col stay; only the name of the region changes.
		msg.Region = core.RegionEditor
		if a.capture == core.RegionOther {
			a.capture = core.RegionEditor
		}
	}
	if a.AgentDragging() {
		if cmd, ok := a.AgentDrag(msg); ok {
			return cmd
		}
	}
	if a.resizing == core.ResizeNone && !a.IsDragging() && !a.term.sel.dragging {
		if cmd, ok := a.AgentMouse(msg); ok {
			return cmd
		}
	}
	// Some terminals report a drag as motion with no button; while a border
	// or a selection is being dragged, motion is the drag whatever it says.
	if msg.Action == core.MouseMotion && msg.Button == core.MouseNone && a.resizing == core.ResizeNone && !a.IsDragging() && !a.term.sel.dragging {
		return a.passiveMotion(l, msg)
	}
	x, y := a.screenCell(l, msg)
	wheel := 0
	switch msg.Button {
	case core.MouseWheelUp:
		wheel = -3
	case core.MouseWheelDown:
		wheel = 3
	}
	// Sideways: the horizontal wheel, or the vertical one with Shift held.
	hwheel := 0
	switch {
	case msg.Button == core.MouseWheelLeft:
		hwheel = -core.HscrollWheel
	case msg.Button == core.MouseWheelRight:
		hwheel = core.HscrollWheel
	}
	// A popup the pointer opened only owns the wheel while the pointer is on it.
	if wheel != 0 && a.Hovering() && (!a.HoverByPointer() || a.inHoverBox(l, core.ScreenPt{x, y})) {
		if msg.Action == core.MousePress {
			a.ScrollHover(wheel)
		}
		return nil
	}
	if a.Hovering() && msg.Button == core.MouseLeft {
		// The button copies on press; the matching release must not close the hover.
		if bx, by, ok := a.hoverPos(l); ok && a.hoverView(l).CopyHit(x-bx, y-by) {
			if msg.Action == core.MousePress {
				a.CopyHover()
			}
			return nil
		}
	}
	if a.OpenCard() != nil && msg.Action == core.MousePress {
		if box, bx, by := a.cardBox(l); box != nil && x >= bx && y >= by && y < by+len(box) && x < bx+ansi.StringWidth(box[0]) {
			if wheel != 0 {
				a.ScrollCard(wheel)
			}
			return nil // reading the card
		}
		if wheel == 0 {
			a.CloseCard()
		}
	}
	// Past the branches that keep the popup open: the pointer's hover is over.
	a.PointerMoved()
	if msg.Action == core.MousePress && hwheel == 0 && wheel != 0 && msg.Shift && a.inEditorText(l, msg) {
		hwheel = wheel / 3 * core.HscrollWheel // Shift turns the vertical wheel sideways, over the text only
	}
	if hwheel != 0 && msg.Action == core.MousePress {
		if a.inEditorText(l, msg) {
			a.ScrollCols(a.Cur(), hwheel)
			return nil
		}
	}
	if wheel != 0 && msg.Action == core.MousePress {
		switch {
		case msg.Region == core.RegionNone:
			// the diff mode header and help lines
		case msg.Region == core.RegionExplorer:
			a.WheelSide(msg.Row, l.bodyH, wheel)
		case msg.Region == core.RegionTerminal:
			a.term.wheel(msg, msg.Col, msg.Row, wheel)
		default:
			a.WheelEditor(wheel)
		}
		return nil
	}

	if a.resizing != core.ResizeNone {
		switch msg.Action {
		case core.MouseMotion:
			a.dragResize(l, x, y)
			return nil
		case core.MouseRelease:
			a.resizing = core.ResizeNone
			return nil
		}
		// A press means the release was lost (pointer left the window).
		a.resizing = core.ResizeNone
	}
	if msg.Action == core.MousePress && msg.Button == core.MouseLeft {
		if r := a.resizeHit(l, x, y); r != core.ResizeNone {
			a.resizing = r
			if r == core.ResizeExplorer {
				return a.SetFocus(core.FocusChanges)
			}
			return nil
		}
	}

	if cmd, ok := a.termMouse(l, msg); ok {
		return cmd
	}

	left := msg.Button == core.MouseLeft
	switch msg.Action {
	case core.MouseRelease:
		a.SetDragging(false)
		return nil
	case core.MouseMotion:
		if a.IsDragging() {
			a.mouseToCursor(l, msg.Row, msg.Col, true)
		}
		return nil
	}
	if !left {
		return nil
	}
	double := msg.Clicks >= 2

	switch {
	case msg.Region == core.RegionNone:
		return nil // the diff mode header and help lines
	case msg.Region == core.RegionExplorer:
		return a.ClickSide(msg.Row, double)
	case msg.Region == core.RegionTerminal:
		return a.SetFocus(core.FocusTerminal)
	case msg.Region == core.RegionEditor && msg.Row >= l.editY-l.top && msg.Row < l.editY-l.top+l.editH:
		t := a.Cur()
		if t == nil {
			return nil
		}
		if t.Kind == core.TabText {
			if a.DiffFile(t) != nil {
				if msg.Row == l.editY-l.top { // the title row
					if !a.DiffSingle(a.DiffFile(t)) {
						a.ToggleSplit()
					}
					return nil
				}
				g := a.dvGeom(l)
				if msg.Col >= g.BarX-l.rightX {
					if dv := a.DiffFor(t); dv != nil {
						a.DiffBarClick(t, dv, g, y)
					}
					return nil
				}
				if msg.Col < g.RightX-l.rightX {
					return nil // the base side is read-only
				}
				if th := a.markerAt(l, x, y); th != nil {
					a.ShowCard(th, t.Buf.Path, x, y, false)
					return nil
				}
			} else if t.Preview != core.PreviewOff && msg.Col >= a.editW(t, l) {
				return nil // the preview pane is read-only
			}
			a.CloseCompletion()
			a.mouseToCursor(l, msg.Row, msg.Col, false)
			a.SetDragging(true)
			return a.SetFocus(core.FocusEditor)
		}
	}
	return nil
}

func (a *App) mouseToCursor(l layout, row, col int, extend bool) {
	t := a.CurText()
	if t == nil {
		return
	}
	if dv := a.DiffFor(t); dv != nil {
		a.diffMouseToCursor(t, dv, l, row, col, extend)
		return
	}
	// row and col count from the pane's corner; the text starts below the title
	// (and the find bar).
	body := row - (l.editY - l.top)
	col = min(col, a.editW(t, l)-1)
	b := t.Buf
	if body < 0 {
		b.MoveLines(-1, extend)
	} else if body >= l.editH {
		b.MoveLines(1, extend)
	}
	r := b.ScrollRow + core.Clamp(body, 0, max(0, l.editH-1))
	vis := max(0, col-core.GutterWidth(b)) + b.ScrollCol
	b.SetCursor(r, vis, extend)
	a.FollowCursor()
}

// passiveMotion handles pointer movement with no button held. It never moves
// the cursor: it only starts the dwell timer, or closes a popup the pointer
// left.
func (a *App) passiveMotion(l layout, msg core.Mouse) []core.Effect {
	x, y := a.screenCell(l, msg)
	if cmd, ok := a.termMouse(l, msg); ok {
		return cmd
	}
	if th := a.markerAt(l, x, y); th != nil && a.Settings().Editor.MouseHover && a.FocusIs(core.FocusEditor) && !a.HoverBlocked() && a.resizing == core.ResizeNone {
		// resting on a comment marker shows its thread
		if !a.CardOnThread(th) {
			a.ShowCard(th, a.CurText().Buf.Path, x, y, true)
			a.CloseHover()
		}
		return nil
	}
	if a.PointerCard() {
		// like a hover, it stays while the pointer reads it, and closes when it leaves
		if box, bx, by := a.cardBox(l); box == nil || x < bx || y < by || y >= by+len(box) || x >= bx+ansi.StringWidth(box[0]) {
			a.CloseCard()
		}
	}
	if !a.Settings().Editor.MouseHover || a.HoverBlocked() || a.resizing != core.ResizeNone {
		return nil
	}
	if !a.FocusIs(core.FocusEditor) {
		// The keys belong to another pane; a popup must not open over them.
		if a.HoverByPointer() {
			a.CloseHover()
		}
		return nil
	}
	pt := core.ScreenPt{x, y}
	if a.RestedOn(pt) {
		return nil
	}
	_, word, ok := a.mouseWord(l, pt)
	return a.PointerRests(word, ok, a.Hovering() && a.inHoverBox(l, pt))
}

// mouseBufPos maps a screen cell to the buffer position of the character under
// it. ok is false outside the text of a plain editor, including past the end
// of a line.
func (a *App) mouseBufPos(l layout, x, y int) (buffer.Pos, bool) {
	t := a.CurText()
	if t == nil || a.FullPreview(t) || a.DiffFor(t) != nil {
		return buffer.Pos{}, false
	}
	b := t.Buf
	gw := core.GutterWidth(b)
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
func (a *App) mouseWord(l layout, pt core.ScreenPt) (buffer.Pos, core.WordSpan, bool) {
	pos, ok := a.mouseBufPos(l, pt.X, pt.Y)
	if !ok {
		return pos, core.WordSpan{}, false
	}
	s, e := buffer.WordRange(a.CurText().Buf.Lines[pos.Row], pos.Col)
	return pos, core.NewWordSpan(pos.Row, s, e), true
}

// inHoverBox reports whether pt is on the open hover popup.
func (a *App) inHoverBox(l layout, pt core.ScreenPt) bool {
	x, y, ok := a.hoverPos(l)
	if !ok {
		return false
	}
	box := a.hoverBox(l)
	bw := 0
	for _, s := range box {
		bw = max(bw, ansi.StringWidth(s))
	}
	return pt.X >= x && pt.X < x+bw && pt.Y >= y && pt.Y < y+len(box)
}

func (a *App) searchMouse(msg core.Mouse) []core.Effect {
	if msg.Action != core.MousePress {
		return nil
	}
	switch msg.Button {
	case core.MouseWheelUp:
		a.SearchScroll(-3)
	case core.MouseWheelDown:
		a.SearchScroll(3)
	case core.MouseLeft:
		x, y := a.screenCell(a.layout(), msg)
		a.SearchClick(x, y)
	}
	return nil
}

// HoverCellWord is mouseWord for the cell the pointer last rested on.
func (a *App) HoverCellWord() (buffer.Pos, core.WordSpan, bool) {
	return a.mouseWord(a.layout(), a.HoverCellPos())
}
