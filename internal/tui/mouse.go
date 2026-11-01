package tui

import (
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
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
func (a *App) termMouse(l layout, msg Mouse) (cmd []Effect, ok bool) {
	t := a.term
	if !a.showTerm {
		return nil, false
	}
	x, y := msg.Col, msg.Row
	if t.sel.dragging {
		switch msg.Action {
		case MouseMotion:
			t.dragTo(x, y)
		case MouseRelease:
			t.dragTo(x, y)
			if s := t.endSelect(); s != "" {
				a.copyText(s)
				a.info(i18n.T("app.termcopied"), utf8.RuneCountInString(s))
			}
		default:
			t.sel.dragging = false // the release was lost
			return nil, false
		}
		return nil, true
	}
	inBody := msg.Region == RegionTerminal && x >= 0 && x < l.fullW && y >= 0
	grabbed := t.grab
	if !grabbed && !inBody {
		return nil, false
	}
	sent := false
	if grabbed || !msg.Shift {
		sent = t.mouse(msg, x, y)
	}
	switch msg.Action {
	case MousePress:
		if sent {
			t.sel = termSel{}
			t.grab = true
			return a.setFocus(focusTerminal), true
		}
		if msg.Button == MouseLeft {
			t.selectAt(x, y)
			return a.setFocus(focusTerminal), true
		}
	case MouseRelease:
		t.grab = false
	}
	return nil, sent || grabbed
}

func (a *App) HandleMouse(msg Mouse) []Effect {
	if a.searchOpen() {
		return a.searchMouse(msg)
	}
	if a.paletteOpen() {
		if msg.Action == MousePress && msg.Button == MouseLeft {
			a.closePalette()
		}
		return nil
	}
	l := a.layout()
	// A press or the wheel over the other pane first moves the focus there.
	if msg.Region == RegionOther && msg.Action == MousePress && (msg.Button == MouseLeft || msg.Button == MouseWheelUp || msg.Button == MouseWheelDown) {
		a.focusPane(a.otherPane())
		l = a.layout()
		// The pane under the pointer is the focused one now. Its corner has not
		// moved, so Row and Col stay; only the name of the region changes.
		msg.Region = RegionEditor
		if a.capture == RegionOther {
			a.capture = RegionEditor
		}
	}
	if a.agentDragging() {
		if cmd, ok := a.agentDrag(msg); ok {
			return cmd
		}
	}
	if a.resizing == ResizeNone && !a.isDragging() && !a.term.sel.dragging {
		if cmd, ok := a.agentMouse(msg); ok {
			return cmd
		}
	}
	// Some terminals report a drag as motion with no button; while a border
	// or a selection is being dragged, motion is the drag whatever it says.
	if msg.Action == MouseMotion && msg.Button == MouseNone && a.resizing == ResizeNone && !a.isDragging() && !a.term.sel.dragging {
		return a.passiveMotion(l, msg)
	}
	x, y := a.screenCell(l, msg)
	wheel := 0
	switch msg.Button {
	case MouseWheelUp:
		wheel = -3
	case MouseWheelDown:
		wheel = 3
	}
	// Sideways: the horizontal wheel, or the vertical one with Shift held.
	hwheel := 0
	switch {
	case msg.Button == MouseWheelLeft:
		hwheel = -hscrollWheel
	case msg.Button == MouseWheelRight:
		hwheel = hscrollWheel
	}
	// A popup the pointer opened only owns the wheel while the pointer is on it.
	if wheel != 0 && a.hovering() && (!a.hoverByPointer() || a.inHoverBox(l, screenPt{x, y})) {
		if msg.Action == MousePress {
			a.ScrollHover(wheel)
		}
		return nil
	}
	if a.hovering() && msg.Button == MouseLeft {
		// The button copies on press; the matching release must not close the hover.
		if bx, by, ok := a.hoverPos(l); ok && a.hoverView(l).copyHit(x-bx, y-by) {
			if msg.Action == MousePress {
				a.copyHover()
			}
			return nil
		}
	}
	if a.openCard() != nil && msg.Action == MousePress {
		if box, bx, by := a.cardBox(l); box != nil && x >= bx && y >= by && y < by+len(box) && x < bx+ansi.StringWidth(box[0]) {
			if wheel != 0 {
				a.scrollCard(wheel)
			}
			return nil // reading the card
		}
		if wheel == 0 {
			a.closeCard()
		}
	}
	// Past the branches that keep the popup open: the pointer's hover is over.
	a.pointerMoved()
	if msg.Action == MousePress && hwheel == 0 && wheel != 0 && msg.Shift && a.inEditorText(l, msg) {
		hwheel = wheel / 3 * hscrollWheel // Shift turns the vertical wheel sideways, over the text only
	}
	if hwheel != 0 && msg.Action == MousePress {
		if a.inEditorText(l, msg) {
			a.scrollCols(a.cur(), hwheel)
			return nil
		}
	}
	if wheel != 0 && msg.Action == MousePress {
		switch {
		case msg.Region == RegionNone:
			// the diff mode header and help lines
		case msg.Region == RegionExplorer:
			a.wheelSide(msg.Row, l.bodyH, wheel)
		case msg.Region == RegionTerminal:
			a.term.wheel(msg, msg.Col, msg.Row, wheel)
		default:
			a.wheelEditor(wheel)
		}
		return nil
	}

	if a.resizing != ResizeNone {
		switch msg.Action {
		case MouseMotion:
			a.dragResize(l, x, y)
			return nil
		case MouseRelease:
			a.resizing = ResizeNone
			return nil
		}
		// A press means the release was lost (pointer left the window).
		a.resizing = ResizeNone
	}
	if msg.Action == MousePress && msg.Button == MouseLeft {
		if r := a.resizeHit(l, x, y); r != ResizeNone {
			a.resizing = r
			if r == ResizeExplorer {
				return a.setFocus(focusChanges)
			}
			return nil
		}
	}

	if cmd, ok := a.termMouse(l, msg); ok {
		return cmd
	}

	left := msg.Button == MouseLeft
	switch msg.Action {
	case MouseRelease:
		a.setDragging(false)
		return nil
	case MouseMotion:
		if a.isDragging() {
			a.mouseToCursor(l, msg.Row, msg.Col, true)
		}
		return nil
	}
	if !left {
		return nil
	}
	double := msg.Clicks >= 2

	switch {
	case msg.Region == RegionNone:
		return nil // the diff mode header and help lines
	case msg.Region == RegionExplorer:
		return a.clickSide(msg.Row, double)
	case msg.Region == RegionTerminal:
		return a.setFocus(focusTerminal)
	case msg.Region == RegionEditor && msg.Row >= l.editY-l.top && msg.Row < l.editY-l.top+l.editH:
		t := a.cur()
		if t == nil {
			return nil
		}
		if t.kind == tabText {
			if a.diffFile(t) != nil {
				if msg.Row == l.editY-l.top { // the title row
					if !a.diffSingle(a.diffFile(t)) {
						a.toggleSplit()
					}
					return nil
				}
				g := a.dvGeom(l)
				if msg.Col >= g.BarX-l.rightX {
					if dv := a.diffFor(t); dv != nil {
						a.diffBarClick(t, dv, g, y)
					}
					return nil
				}
				if msg.Col < g.RightX-l.rightX {
					return nil // the base side is read-only
				}
				if th := a.markerAt(l, x, y); th != nil {
					a.showCard(th, t.buf.Path, x, y, false)
					return nil
				}
			} else if t.preview != previewOff && msg.Col >= a.editW(t, l) {
				return nil // the preview pane is read-only
			}
			a.closeCompletion()
			a.mouseToCursor(l, msg.Row, msg.Col, false)
			a.setDragging(true)
			return a.setFocus(focusEditor)
		}
	}
	return nil
}

func (a *App) mouseToCursor(l layout, row, col int, extend bool) {
	t := a.curText()
	if t == nil {
		return
	}
	if dv := a.diffFor(t); dv != nil {
		a.diffMouseToCursor(t, dv, l, row, col, extend)
		return
	}
	// row and col count from the pane's corner; the text starts below the title
	// (and the find bar).
	body := row - (l.editY - l.top)
	col = min(col, a.editW(t, l)-1)
	b := t.buf
	if body < 0 {
		b.MoveLines(-1, extend)
	} else if body >= l.editH {
		b.MoveLines(1, extend)
	}
	r := b.ScrollRow + clamp(body, 0, max(0, l.editH-1))
	vis := max(0, col-gutterWidth(b)) + b.ScrollCol
	b.SetCursor(r, vis, extend)
	a.followCursor()
}

// passiveMotion handles pointer movement with no button held. It never moves
// the cursor: it only starts the dwell timer, or closes a popup the pointer
// left.
func (a *App) passiveMotion(l layout, msg Mouse) []Effect {
	x, y := a.screenCell(l, msg)
	if cmd, ok := a.termMouse(l, msg); ok {
		return cmd
	}
	if th := a.markerAt(l, x, y); th != nil && a.settings().Editor.MouseHover && a.focusIs(focusEditor) && !a.hoverBlocked() && a.resizing == ResizeNone {
		// resting on a comment marker shows its thread
		if !a.cardOnThread(th) {
			a.showCard(th, a.curText().buf.Path, x, y, true)
			a.closeHover()
		}
		return nil
	}
	if a.pointerCard() {
		// like a hover, it stays while the pointer reads it, and closes when it leaves
		if box, bx, by := a.cardBox(l); box == nil || x < bx || y < by || y >= by+len(box) || x >= bx+ansi.StringWidth(box[0]) {
			a.closeCard()
		}
	}
	if !a.settings().Editor.MouseHover || a.hoverBlocked() || a.resizing != ResizeNone {
		return nil
	}
	if !a.focusIs(focusEditor) {
		// The keys belong to another pane; a popup must not open over them.
		if a.hoverByPointer() {
			a.closeHover()
		}
		return nil
	}
	pt := screenPt{x, y}
	if a.restedOn(pt) {
		return nil
	}
	_, word, ok := a.mouseWord(l, pt)
	return a.pointerRests(word, ok, a.hovering() && a.inHoverBox(l, pt))
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

func (a *App) searchMouse(msg Mouse) []Effect {
	if msg.Action != MousePress {
		return nil
	}
	switch msg.Button {
	case MouseWheelUp:
		a.searchScroll(-3)
	case MouseWheelDown:
		a.searchScroll(3)
	case MouseLeft:
		x, y := a.screenCell(a.layout(), msg)
		a.searchClick(x, y)
	}
	return nil
}

// hoverCellWord is mouseWord for the cell the pointer last rested on.
func (a *App) HoverCellWord() (buffer.Pos, wordSpan, bool) {
	return a.mouseWord(a.layout(), a.hoverCellPos())
}
