package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// drawExplorer draws the tree as h lines of w cells.
func drawExplorer(ex screen.Explorer, w, h int, focused bool) []string {
	lines := make([]string, 0, h)
	lines = append(lines, stSideTitle.Render(core.Fit(ex.Title, w)))
	for i := range max(0, h-1) {
		if i >= len(ex.Rows) {
			lines = append(lines, stSide.Render(core.Fit("", w)))
			continue
		}
		n := ex.Rows[i]
		chevron := "  "
		if n.Dir {
			chevron = "▸ "
			if n.Expanded {
				chevron = "▾ "
			}
			if ex.Icons {
				chevron = "\ueab6 " // codicon chevron-right
				if n.Expanded {
					chevron = "\ueab4 " // codicon chevron-down
				}
			}
		}
		// Each nesting level draws a faint guide under its parent's chevron.
		indent := " " + strings.Repeat("│ ", n.Depth)
		st := stSide
		if n.Dir {
			st = stSideDir
		}
		if n.Open {
			st = stSideActive
		}
		if n.Selected {
			if focused {
				st = stSideSelFocus
			} else {
				st = stSideSel
			}
		}
		letter := n.Git
		textSt, tail, avail := st, "", w
		if n.Ignored {
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
		if n.IconColor != "" && !n.Ignored {
			iconSt = st.Foreground(lipgloss.Color(n.IconColor))
		}
		// Segments are styled separately so each keeps the row background.
		guideSt := st.UnsetBold().Foreground(lipgloss.Color(core.CurTheme.Dim))
		head := guideSt.Render(indent) + st.Render(chevron) + iconSt.Render(n.Icon)
		lead := indent + chevron + n.Icon
		name := ansi.Truncate(n.Name, max(0, avail-ansi.StringWidth(lead)), "…")
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
