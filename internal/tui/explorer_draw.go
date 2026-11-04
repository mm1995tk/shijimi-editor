package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/core"
)

// explorerRow is a row of the tree as drawn.
type explorerRow struct {
	depth     int
	name      string
	dir       bool
	expanded  bool
	icon      string
	iconColor string
	open      bool // the file open in the editor
	selected  bool
	git       string // the letter of its git status
	ignored   bool
}

// explorerRows are the rows of the tree in view, h rows high with the title,
// as fit left the scroll. activePath is the file open in the editor. It
// changes nothing.
func explorerRows(e *core.Explorer, h int, activePath string) []explorerRow {
	var rows []explorerRow
	for i := range max(0, h-1) {
		idx := e.Scroll + i
		if idx >= len(e.Flat) {
			break
		}
		n := e.Flat[idx]
		r := explorerRow{
			depth: n.Depth(), name: n.Name, dir: n.IsDir(), expanded: n.Expanded(),
			open: n.Path == activePath, selected: idx == e.Sel,
			git: e.Git[n.Path], ignored: e.IsIgnored(n),
		}
		if e.Icons {
			ic := core.IconFor(n)
			r.icon, r.iconColor = ic.Glyph+" ", ic.Color
		}
		rows = append(rows, r)
	}
	return rows
}

// drawExplorer draws the tree as h lines of w cells.
func drawExplorer(e *core.Explorer, activePath string, w, h int, focused bool) []string {
	rows := explorerRows(e, h, activePath)
	lines := make([]string, 0, h)
	lines = append(lines, stSideTitle.Render(core.Fit(" EXPLORER: "+strings.ToUpper(e.RootName()), w)))
	for i := range max(0, h-1) {
		if i >= len(rows) {
			lines = append(lines, stSide.Render(core.Fit("", w)))
			continue
		}
		n := rows[i]
		chevron := "  "
		if n.dir {
			chevron = "▸ "
			if n.expanded {
				chevron = "▾ "
			}
			if e.Icons {
				chevron = "\ueab6 " // codicon chevron-right
				if n.expanded {
					chevron = "\ueab4 " // codicon chevron-down
				}
			}
		}
		// Each nesting level draws a faint guide under its parent's chevron.
		indent := " " + strings.Repeat("│ ", n.depth)
		st := stSide
		if n.dir {
			st = stSideDir
		}
		if n.open {
			st = stSideActive
		}
		if n.selected {
			if focused {
				st = stSideSelFocus
			} else {
				st = stSideSel
			}
		}
		letter := n.git
		textSt, tail, avail := st, "", w
		if n.ignored {
			textSt = st.Foreground(lipgloss.Color(core.CurTheme.Dim))
		} else if letter != "" {
			col := gitColor(letter)
			if letter == "•" {
				col = cDirty
			}
			textSt = st.Foreground(lipgloss.Color(col))
			tail = textSt.Render(" " + letter)
			avail = w - 2
		}
		iconSt := textSt
		if n.iconColor != "" && !n.ignored {
			iconSt = st.Foreground(lipgloss.Color(n.iconColor))
		}
		// Segments are styled separately so each keeps the row background.
		guideSt := st.UnsetBold().Foreground(lipgloss.Color(core.CurTheme.Dim))
		head := guideSt.Render(indent) + st.Render(chevron) + iconSt.Render(n.icon)
		lead := indent + chevron + n.icon
		name := ansi.Truncate(n.name, max(0, avail-ansi.StringWidth(lead)), "…")
		used := ansi.StringWidth(lead) + ansi.StringWidth(name)
		line := head + textSt.Render(name) + textSt.Render(strings.Repeat(" ", max(0, avail-used)))
		lines = append(lines, ansi.Truncate(line, avail, "")+tail)
	}
	return lines
}

func gitColor(letter string) string {
	switch letter {
	case "U", "A":
		return core.CurTheme.GitAdded
	case "D":
		return core.CurTheme.GitDeleted
	case "R":
		return core.CurTheme.GitAdded
	}
	return cDirty
}
