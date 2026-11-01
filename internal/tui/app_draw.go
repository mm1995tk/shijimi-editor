package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// renderTitle is the line above the editor: the path of the file being
// edited (● when it is unsaved). Open files are listed with "Space b".
func (a *App) renderTitle(t *tab, w int, focused bool) string {
	return drawTitle(a.titleOf(t), w, focused)
}

// findInputCells is the width of the find bar between its label and hint.
func findInputCells(w int) int {
	return w - lipgloss.Width(stFindLabel.Render(findLabel())) - lipgloss.Width(stFind.Render(i18n.T("app.findhint")))
}

func (a *App) renderFind(w int) string {
	return drawFind(*a.findState(), a.findField().view(inputStyleOn(cPaletteBg)), w)
}

var modeStyles map[editMode]lipgloss.Style

func (a *App) renderStatus() string { return drawStatus(a.statusOf(), a.confirmOverlay(), a.w) }

// editW is the width of the text editor, narrower while the Markdown preview
// is open and zero while the preview fills the area.
func (a *App) editW(t *tab, l layout) int {
	if a.diffFile(t) != nil {
		return a.dvGeom(l).rightW
	}
	switch t.preview {
	case previewSplit:
		return a.editorWidth(l.rightW)
	case previewFull:
		return 0
	}
	return l.rightW
}

func (a *App) renderText(t *tab, l layout, focused bool) []string {
	if dv := a.diffFor(t); dv != nil {
		return a.renderDiffText(t, dv, l, focused)
	}
	if t.preview == previewFull {
		return a.previewLines(t, l.rightW, l.editH)
	}
	ew := a.editW(t, l)
	edit := renderEditor(t, ew, l.editH, a.editorOpts(t, focused))
	if t.preview == previewOff {
		return edit
	}
	pw := l.rightW - ew - 1
	prev := a.previewLines(t, pw, l.editH)
	sep := stSeparator.Render("│")
	for i := range edit {
		edit[i] += sep + prev[i]
	}
	return edit
}

// renderFrame draws the frame and returns, with it, where the terminal cursor
// goes (nil when it is hidden). The cursor is a result of drawing, not state
// kept on App.
func (a *App) renderFrame() (string, *tea.Cursor) {
	if a.w <= 0 || a.h <= 0 {
		return "", nil
	}
	if a.w < 30 || a.h < 8 {
		return i18n.T("app.toosmall"), nil
	}
	l := a.layout()

	right := make([]string, 0, l.bodyH)
	editorFocused := a.focusIn(focusEditor)
	mine := a.renderPane(a.cur(), l, true, editorFocused)
	if l.split {
		theirs := a.renderPane(a.otherTab(), l.otherLayout(), false, false)
		if a.focusedFirst() {
			right = append(right, joinPanes(mine, theirs, l.rightW, l.otherW)...)
		} else {
			right = append(right, joinPanes(theirs, mine, l.otherW, l.rightW)...)
		}
	} else {
		right = append(right, mine...)
	}
	t := a.cur()
	if a.showTerm {
		right = append(right, a.term.render(l.fullW, l.panelH, a.focusIn(focusTerminal))...)
	}

	rows := make([]string, l.bodyH)
	var side []string
	activePath := ""
	if t != nil {
		activePath = t.path()
	}
	if a.showSide {
		side = drawExplorer(a.explorerScreen(a.explorerH(), activePath), l.sideW, a.explorerH(), a.focusIn(focusExplorer))
		if a.diffMode() {
			side = append(side, drawChanges(a.changesScreen(l.sideW, l.bodyH-a.explorerH()), l.sideW, l.bodyH-a.explorerH(), a.focusIn(focusChanges))...)
		}
	}
	var agent []string
	if l.agentW > 0 {
		focused := a.focusIn(focusAgent)
		agent = drawAgent(a.agentScreen(l.agentW, l.bodyH, focused), l.agentW, l.bodyH, focused)
	}
	for i := range rows {
		var sb strings.Builder
		if a.showSide {
			sb.WriteString(side[i])
			sb.WriteString(stSeparator.Render("│"))
		}
		if i < len(right) {
			sb.WriteString(right[i])
		}
		if agent != nil {
			if i < len(right) {
				if rw := ansi.StringWidth(right[i]); rw < l.fullW {
					sb.WriteString(stEmpty.Render(strings.Repeat(" ", l.fullW-rw)))
				}
			} else {
				sb.WriteString(stEmpty.Render(strings.Repeat(" ", l.fullW)))
			}
			sb.WriteString(stSeparator.Render("│"))
			sb.WriteString(agent[i])
		}
		rows[i] = sb.String()
	}
	if a.diffMode() {
		rows = append([]string{a.renderDiffHeader(a.w)}, rows...)
		rows = append(rows, a.renderDiffHelp(a.w))
	}

	// popups anchored at the cursor
	if t := a.curText(); t != nil && !a.fullPreview(t) && (a.completing() || a.hovering()) {
		cx, cy := a.popupAnchor(t, l)
		var box []string
		if a.completing() {
			box = drawCompletion(a.completionOverlay(max(3, l.editH/2)))
		} else {
			box = a.hoverBox(l)
		}
		x, y := popupPos(l, a.w, cx, cy, box)
		rows = overlay(rows, box, x, y)
	}

	if st := a.signatureTab(); st != nil {
		if cx, cy := a.cursorScreen(st, l); a.cursorShown(st, l, cx, cy) {
			box := drawSignature(a.signatureOverlay(), max(10, a.w-l.rightX))
			x, y := signaturePos(l, a.w, cx, cy, box)
			rows = overlay(rows, box, x, y)
		}
	}

	if a.cardOpen() {
		if box, x, y := a.cardBox(l); box != nil {
			rows = overlay(rows, box, x, y)
		}
	}
	if a.composing() {
		box, x, y := a.composerBox(l)
		rows = overlay(rows, box, x, y)
	}

	if a.menuOpen(" ") {
		rows = a.renderSpaceMenu(rows, l)
	}
	if a.menuOpen("space v") {
		rows = a.renderMenu(rows, l, i18n.T("app.reviewhint"), reviewItems)
	}
	if a.menuOpen("space w") {
		rows = a.renderMenu(rows, l, i18n.T("win.hint"), windowItems)
	}

	if a.paletteOpen() {
		pw := a.paletteWidth()
		box := drawPalette(a.paletteOverlay(), a.paletteField(), pw)
		rows = overlay(rows, box, (a.w-pw)/2, 1)
	}

	if a.searchOpen() {
		x, y, box := a.renderSearch()
		rows = overlay(rows, box, x, y)
	}

	if a.contextOpen() {
		rows = a.renderContext(rows)
	}
	if a.proposalsOpen() {
		rows = a.renderProposal(rows)
	}

	if a.commandLineOpen() {
		rows = append(rows, a.renderCmdline())
	} else {
		rows = append(rows, a.renderStatus())
	}
	return strings.Join(rows, "\n"), a.caretCursor(l)
}

// popupPos places box below the cursor cell (cx, cy), or above it when there
// is no room below, clamped to the editor area of a screen w cells wide.
func popupPos(l layout, w, cx, cy int, box []string) (x, y int) {
	bw := 0
	for _, s := range box {
		bw = max(bw, ansi.StringWidth(s))
	}
	x = clamp(cx, l.rightX, max(l.rightX, w-bw))
	y = cy + 1
	if y+len(box) > l.editY+l.editH && cy-len(box) >= l.editY {
		y = cy - len(box)
	}
	y = max(l.editY, min(y, l.editY+l.editH-len(box)))
	return x, y
}

// hoverPos is the screen position of the hover popup's top-left corner.
func (a *App) hoverPos(l layout) (x, y int, ok bool) {
	t := a.curText()
	if t == nil || a.fullPreview(t) {
		return 0, 0, false
	}
	cx, cy := a.popupAnchor(t, l)
	x, y = popupPos(l, a.w, cx, cy, a.hoverBox(l))
	return x, y, true
}

// hoverView lays out the current hover text for layout l. The box height is
// capped to the larger free side of the cursor so it never overflows.
func (a *App) hoverView(l layout) hoverView {
	h := l.editH * 2 / 3
	if t := a.curText(); t != nil {
		_, cy := a.popupAnchor(t, l)
		below := l.editY + l.editH - (cy + 1)
		above := cy - l.editY
		h = min(h, max(below, above))
	}
	return newHoverView(a.hoverContent(), min(80, l.rightW-2), max(h, 3))
}

// hoverOverlay is the open hover popup as a screen.Overlay, hanging from the
// character it describes: the pointer's for a hover it triggered, the cursor's
// otherwise. Drawing it as cells is drawHover's work.
func (a *App) hoverOverlay(l layout) screen.Overlay {
	ov := a.hoverView(l).overlay(a.hoverTop())
	if t := a.curText(); t != nil {
		ov.Pane = t.path()
		at := toScreenPos(a.popupAnchorPos(t))
		ov.At = &at
	}
	return ov
}

// hoverBox is the hover popup as a box of cells.
func (a *App) hoverBox(l layout) []string { return drawHover(a.hoverOverlay(l)) }

func (a *App) scrollHover(d int) {
	v := a.hoverView(a.layout())
	a.setHoverTop(clamp(a.hoverTop()+d, 0, v.maxScroll()))
}

// fit brings the state that depends on the window size and on what is open
// up to date for the frame about to be drawn: the scroll that follows the
// cursor, the palette's filtered list and the widths of the inputs. It runs
// before renderFrame, which only reads the state.
func (a *App) fit() {
	// Key releases let Ctrl+D / Ctrl+U scroll only while held. Some terminals
	// (herdr 0.9.5) then drop text committed by an IME, so they are asked
	// for only where those keys scroll. The request is frozen once quitting
	// (see exit).
	if !a.quitting {
		a.askReleases = a.wantKeyReleases()
	}
	if a.w < 30 || a.h < 8 {
		return
	}
	l := a.layout()
	// The panes: a tab's diff view is made here, and the scroll of a pane
	// that is not being edited follows its cursor in the width it has now.
	type pane struct {
		t       *tab
		focused bool
	}
	panes := []pane{{a.cur(), a.focusIn(focusEditor)}}
	if l.split {
		panes = append(panes, pane{a.otherTab(), false})
	}
	for _, p := range panes {
		pl := l
		if p.t == a.otherTab() && l.split {
			pl = l.otherLayout()
		}
		if p.t == nil || p.t.kind == tabImage {
			continue
		}
		if p.t.kind == tabText {
			a.diffFor(p.t)
		}
		if !p.focused && !a.fullPreview(p.t) {
			ensureCursorVisible(p.t.buf, a.editW(p.t, pl), pl.editH) // the width changed under it
		}
	}
	if t := a.curText(); t != nil {
		a.fitCursor(t, a.editW(t, l), l.editH, a.dvGeom(l).h)
	}
	a.fitPalette()
	if a.showTerm {
		a.term.fit(l.fullW, l.panelH)
	}
	a.fitFields(findInputCells(l.rightW), a.w)
	if a.showSide {
		a.fitSide(l.bodyH)
	}
	a.fitAgent(l)
	a.fitProposals()
}

// fitAgent fits the suggestion panel's state to the area it is drawn in.
func (a *App) fitAgent(l layout) {
	if l.agentW <= 0 {
		return
	}
	a.fitAgentPanel(l.agentW, l.bodyH)
}

// popupAnchor is the cell a popup hangs from: the pointer for a hover it
// triggered, the cursor otherwise.
func (a *App) popupAnchor(t *tab, l layout) (x, y int) {
	return a.posScreen(t, l, a.popupAnchorPos(t))
}

// renderPane draws one pane: title row, the find bar row when open (only the
// focused pane has the bar), and the body.
func (a *App) renderPane(t *tab, l layout, current, focused bool) []string {
	rows := make([]string, 0, l.editH+2)
	rows = append(rows, a.renderTitle(t, l.rightW, current))
	if a.findShown() {
		if current {
			rows = append(rows, a.renderFind(l.rightW))
		} else {
			rows = append(rows, stTabBar.Render(strings.Repeat(" ", l.rightW)))
		}
	}
	switch {
	case t == nil:
		rows = append(rows, renderWelcome(l.rightW, l.editH, a.helixOn())...)
	case t.kind == tabImage:
		rows = append(rows, a.imageLines(t, l.rightW, l.editH)...)
	default:
		rows = append(rows, a.renderText(t, l, focused)...)
	}
	return rows
}
