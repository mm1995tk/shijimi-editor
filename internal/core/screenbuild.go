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
	if a.ScreenW() != cols || a.ScreenH() != rows+1 {
		// The front end's screen is the files region and the status line: the
		// core's paging and scrolling read it (EditRows, TextCols).
		a.SetScreen(cols, rows+1)
		a.Follow = true
	}

	type slot struct {
		t       *Tab
		w       int
		focused bool
	}
	slots := []slot{{t: a.Cur(), w: cols, focused: a.FocusIn(FocusEditor)}}
	if a.Split {
		first := (cols - 1) / 2
		mine, theirs := slot{t: a.Cur(), focused: slots[0].focused}, slot{t: a.Other}
		if a.FocusedFirst() {
			mine.w, theirs.w = first, cols-1-first
			slots = []slot{mine, theirs}
		} else {
			theirs.w, mine.w = first, cols-1-first
			slots = []slot{theirs, mine}
		}
	}

	// fit, as the TUI's App.fit does
	for _, s := range slots {
		if s.t == nil || s.t.Kind != TabText {
			continue
		}
		a.DiffFor(s.t)
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
	a.FitPalette()
	a.FitFields(max(1, v.Cmdline.Cols-2), max(1, v.Cmdline.Cols))

	sc := screen.Screen{Focus: a.screenFocus()}
	files := &screen.Files{Panes: []screen.Pane{}}
	seen := map[string]int{}
	for _, s := range slots {
		panes := a.screenPanes(s.t, s.w, rows, s.focused)
		if s.t == a.Cur() {
			// the focused pane is the editable one: the last of a diff
			files.Active = len(files.Panes) + len(panes) - 1
		}
		for _, p := range panes {
			// the two halves of a split may show the same file
			if n := seen[p.ID]; n > 0 {
				seen[p.ID]++
				p.ID = fmt.Sprintf("%s#%d", p.ID, n+1)
			} else {
				seen[p.ID] = 1
			}
			files.Panes = append(files.Panes, p)
		}
	}
	sc.Files = files
	sc.Status = a.StatusOf()
	if c := a.CmdlineState(); c != nil {
		sc.Cmdline = c
	} else if f := a.FindState(); f != nil {
		sc.Cmdline = f
	}
	sc.Overlay = a.screenOverlays(v)
	return sc
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
		return (w - 1) / 2
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

// screenPanes is the Panes that show t in a pane w columns wide and h rows
// high: one, or two for a split diff or a text beside its preview.
func (a *Core) screenPanes(t *Tab, w, h int, focused bool) []screen.Pane {
	switch {
	case t == nil:
		return []screen.Pane{{ID: "welcome", Kind: screen.KindWelcome, Viewport: screen.Viewport{Rows: h, Cols: w}}}
	case t.Kind == TabImage:
		return []screen.Pane{a.BuildImagePane(t, w, h)}
	}
	o := a.EditorOpts(t, focused)
	if dv := a.DiffFor(t); dv != nil {
		return a.BuildDiffPanes(t, dv, a.screenDiffGeom(w, h), o.PaneOpts)
	}
	switch t.Preview {
	case PreviewFull:
		return []screen.Pane{a.BuildPreviewPane(t, w, h)}
	case PreviewSplit:
		ew := (w - 1) / 2
		return []screen.Pane{BuildTextPane(t, ew, h, o.PaneOpts), a.BuildPreviewPane(t, w-ew-1, h)}
	}
	return []screen.Pane{BuildTextPane(t, w, h, o.PaneOpts)}
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
