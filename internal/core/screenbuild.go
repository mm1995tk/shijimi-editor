package core

import (
	"fmt"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// screenbuild.go makes the whole screen.Screen of a core for the GUI of `shj
// serve` (docs/design/gui-protocol.md, 3 章「画面（Screen）の送り方」): the
// files region, the status line, the command line and the overlays. The TUI
// draws the same parts (internal/tui's renderFrame) with its own layout; here
// the GUI's regions have the sizes it sent with view.resize. The regions only
// the TUI shows (the tree, the changes list, the agent panel) are left out.

// BuildScreen fits the core to the regions' sizes v, as the TUI fits a frame
// before it draws it (the scroll that follows the cursor, the widths of the
// inputs), and makes what is on screen. Rev and AckSeq are left for the caller.
func (a *Core) BuildScreen(v screen.View) screen.Screen {
	cols, rows := max(1, v.Files.Cols), max(1, v.Files.Rows)
	a.fitScreenSize(cols, rows)
	slots := a.screenSlots(cols)
	a.fitSlots(slots, rows)
	a.FitPalette()
	a.FitFields(max(1, v.Cmdline.Cols-2), max(1, v.Cmdline.Cols))

	sc := screen.Screen{
		Focus:   a.screenFocus(),
		Files:   a.screenFiles(slots, rows),
		Status:  a.StatusOf(),
		Cmdline: a.CmdlineState(),
		Overlay: a.screenOverlays(v),
	}
	if sc.Cmdline == nil {
		sc.Cmdline = a.FindState()
	}
	return sc
}

// fitScreenSize makes the front end's screen the files region (cols x rows)
// and the status line: the core's paging and scrolling read it (EditRows,
// TextCols). A new size makes the scroll follow the cursor again.
func (a *Core) fitScreenSize(cols, rows int) {
	if a.ScreenW() != cols || a.ScreenH() != rows+1 {
		a.SetScreen(cols, rows+1)
		a.Follow = true
	}
}

// screenSlot is one half of the files region (or all of it): the tab it
// shows and its width.
type screenSlot struct {
	t       *Tab
	w       int
	focused bool // the editor has the keys and this is the focused pane
	mine    bool // this is the focused pane (Cur), whether or not the editor has the keys
}

// screenSlots splits cols columns between the panes of the editor: all of them
// to the current file, or two halves with a column between them when split.
func (a *Core) screenSlots(cols int) []screenSlot {
	mine := screenSlot{t: a.Cur(), w: cols, focused: a.FocusIn(FocusEditor), mine: true}
	if !a.Split {
		return []screenSlot{mine}
	}
	first := (cols - 1) / 2
	theirs := screenSlot{t: a.Other}
	if a.FocusedFirst() {
		mine.w, theirs.w = first, cols-1-first
		return []screenSlot{mine, theirs}
	}
	theirs.w, mine.w = first, cols-1-first
	return []screenSlot{theirs, mine}
}

// fitSlots scrolls the texts of the slots to their cursors in rows rows, as
// the TUI's App.fit does.
func (a *Core) fitSlots(slots []screenSlot, rows int) {
	for _, s := range slots {
		if s.t == nil || s.t.Kind != TabText {
			continue
		}
		a.DiffFor(s.t)
		if a.DiffFile(s.t) == nil {
			switch s.t.Preview { // the widths screenPanes gives the preview
			case PreviewFull:
				a.FitPreview(s.t, s.w)
			case PreviewSplit:
				a.FitPreview(s.t, s.w-previewSplitW(s.w)-1)
			}
		}
		if s.t != a.Cur() && !a.FullPreview(s.t) {
			EnsureCursorVisible(s.t.Buf, a.screenEditW(s.t, s.w), rows)
		}
	}
	if t := a.CurText(); t != nil {
		for _, s := range slots {
			if s.t == t {
				a.FitCursor(t, a.screenEditW(t, s.w), rows, rows)
			}
		}
	}
}

// screenFiles is the files region: the Panes of the slots, rows high. Active
// is the pane of the current file that takes the keys. The two halves of a
// split may show the same file; the second gets "#2" on its ID.
func (a *Core) screenFiles(slots []screenSlot, rows int) *screen.Files {
	files := &screen.Files{Panes: []screen.Pane{}}
	for i, p := range a.placePanes(slots, rows) {
		if p.slot.mine && p.editable {
			files.Active = i
		}
		files.Panes = append(files.Panes, p.Pane)
	}
	return files
}

// placedPane is a Pane of the files region with the slot it is in.
type placedPane struct {
	screen.Pane
	slot     screenSlot
	editable bool // the pane of the slot that has the cursor (see screenPanes)
}

// placePanes is the Panes of the slots, rows high, in the order the GUI gets
// them. The text an input method is composing shows in the pane that takes
// the keys.
func (a *Core) placePanes(slots []screenSlot, rows int) []placedPane {
	var out []placedPane
	seen := map[string]int{}
	for _, s := range slots {
		panes, editable := a.screenPanes(s.t, s.w, rows, s.focused)
		for i, p := range panes {
			if n := seen[p.ID]; n > 0 {
				seen[p.ID]++
				p.ID = fmt.Sprintf("%s#%d", p.ID, n+1)
			} else {
				seen[p.ID] = 1
			}
			if s.focused && i == editable && p.Kind == screen.KindText && a.preedit != nil && !a.CmdOpen && !a.FindOpen {
				splicePreedit(&p, ToScreenPos(s.t.Buf.Cur), a.preedit.Text)
			}
			out = append(out, placedPane{Pane: p, slot: s, editable: i == editable})
		}
	}
	return out
}

// screenFocus is the region that has the keys.
func (a *Core) screenFocus() screen.Region {
	switch {
	case a.CmdOpen || a.FindOpen:
		return screen.RegionCmdline
	case a.overlayOpen():
		return screen.RegionOverlay
	}
	switch a.Focus {
	case FocusExplorer:
		return screen.RegionExplorer
	case FocusChanges:
		return screen.RegionChanges
	case FocusAgent:
		return screen.RegionAgent
	case FocusTerminal:
		return screen.RegionTerminal
	}
	return screen.RegionFiles
}

// screenEditW is the width of the text editor of t in a pane w columns wide:
// half of it beside the Markdown preview, none while the preview fills it.
func (a *Core) screenEditW(t *Tab, w int) int {
	if a.DiffFile(t) != nil {
		return a.screenDiffGeom(w, 0).RightW
	}
	switch t.Preview {
	case PreviewSplit:
		return previewSplitW(w)
	case PreviewFull:
		return 0
	}
	return w
}

// screenDiffGeom lays out a diff view w columns wide and h rows high: the base
// and the buffer side by side with a column between them when the diff is
// split. The GUI draws the change bar of its own, so it takes no column.
func (a *Core) screenDiffGeom(w, h int) DiffGeom {
	g := DiffGeom{H: h, RightW: w}
	if a.SplitShown() {
		g.LeftW = (w - 1) / 2
		g.RightX = g.LeftW + 1
		g.RightW = w - g.LeftW - 1
	}
	return g
}

// previewSplitW is the width of the text beside its Markdown preview in a pane
// w columns wide; the preview has the rest but the column between them.
func previewSplitW(w int) int { return (w - 1) / 2 }

// screenPanes is the Panes that show t in a pane w columns wide and h rows
// high: one, or two for a split diff or a text beside its preview. editable
// is the index of the one that has the cursor and takes the keys.
func (a *Core) screenPanes(t *Tab, w, h int, focused bool) (panes []screen.Pane, editable int) {
	switch {
	case t == nil:
		return []screen.Pane{{ID: "welcome", Kind: screen.KindWelcome, Viewport: screen.Viewport{Rows: h, Cols: w}}}, 0
	case t.Kind == TabImage:
		return []screen.Pane{a.BuildImagePane(t, w, h)}, 0
	}
	o := a.EditorOpts(t, focused)
	if dv := a.DiffOf(t); dv != nil {
		// the buffer's side is the last: [base, buffer] or [unified]
		panes = a.BuildDiffPanes(t, dv, a.screenDiffGeom(w, h), o.PaneOpts)
		return panes, len(panes) - 1
	}
	switch t.Preview {
	case PreviewFull:
		return []screen.Pane{a.BuildPreviewPane(t, w, h)}, 0
	case PreviewSplit:
		ew := previewSplitW(w)
		return []screen.Pane{BuildTextPane(t, ew, h, o.PaneOpts), a.BuildPreviewPane(t, w-ew-1, h)}, 0
	}
	return []screen.Pane{BuildTextPane(t, w, h, o.PaneOpts)}, 0
}

// screenOverlays is what is over the panes, in the order the TUI draws it
// (the later on top).
func (a *Core) screenOverlays(v screen.View) []screen.Overlay {
	ow, oh := max(1, v.Overlay.Cols), max(1, v.Overlay.Rows)
	out := []screen.Overlay{}
	if t := a.CurText(); t != nil && !a.FullPreview(t) && (a.Completing() || a.Hovering()) {
		var ov screen.Overlay
		if a.Completing() {
			ov = a.CompletionOverlay(max(3, oh/2))
		} else {
			ov = NewHoverView(a.HoverContent(), min(80, ow-2), max(3, oh*2/3)).Overlay(a.HoverTop())
		}
		ov.Pane = t.Path()
		at := ToScreenPos(a.PopupAnchorPos(t))
		ov.At = &at
		out = append(out, ov)
	}
	if st := a.SignatureTab(); st != nil {
		ov := a.SignatureOverlay()
		ov.Pane = st.Path()
		at := ToScreenPos(st.Buf.Cur)
		ov.At = &at
		out = append(out, ov)
	}
	if a.CardOpen() {
		if w := min(84, ow-2); w >= 20 {
			out = append(out, a.CardOverlay(a.OpenCard(), w-4, max(3, oh*2/3-2)))
		}
	}
	if a.Composing() {
		w := min(96, ow-2)
		if w < 24 {
			w = max(10, ow)
		}
		out = append(out, a.ComposerOverlay(w-4, min(max(0, oh/2), 12)))
	}
	switch {
	case a.MenuOpen(" "):
		out = append(out, MenuOverlay(" Space", a.SpaceMenuItems()))
	case a.MenuOpen("space v"):
		out = append(out, MenuOverlay(i18n.T("app.reviewhint"), ReviewItems))
	case a.MenuOpen("space w"):
		out = append(out, MenuOverlay(i18n.T("win.hint"), WindowItems))
	}
	if a.PaletteOpen() {
		out = append(out, a.PaletteOverlay())
	}
	if c := a.ConfirmOverlay(); c != nil {
		out = append(out, *c)
	}
	return out
}
