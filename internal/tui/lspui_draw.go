package tui

import (
	"github.com/charmbracelet/x/ansi"
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// drawCompletion draws a completion Overlay as a box of cells.
func drawCompletion(ov screen.Overlay) []string {
	w := 20
	for _, it := range ov.Items {
		w = max(w, ansi.StringWidth(it.Label)+10)
	}
	w = min(w, 60)
	out := make([]string, 0, len(ov.Items))
	for i, it := range ov.Items {
		st, dim := stPalette, stPaletteDim
		if i == ov.Active {
			st, dim = stPaletteSel, bgfg(cListSel, core.CurTheme.SubSel)
		}
		label := core.Fit(" "+it.Label, w-9)
		out = append(out, st.Render(label)+dim.Render(core.Fit(" "+it.Kind, 9)))
	}
	return out
}
