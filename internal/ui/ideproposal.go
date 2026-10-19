package ui

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/dageki/internal/diff"
	"github.com/mm1995tk/dageki/internal/ide"
)

// diffContext is how many unchanged lines surround each change.
const diffContext = 3

// proposal is a change the agent asked to apply with openDiff. Accepting
// answers FILE_SAVED; Claude Code then writes the file itself, as it does for
// the VS Code extension, and the file watcher picks the change up.
type proposal struct {
	name             string
	oldPath, newPath string
	newText          string
	lines            []propLine
	scroll           int
	reply            chan ideReply
}

type propLine struct {
	kind byte // ' ', '+', '-' or '@'
	text string
}

func newProposal(name, oldPath, newPath, oldText, newText string, reply chan ideReply) *proposal {
	return &proposal{
		name: name, oldPath: oldPath, newPath: newPath, newText: newText,
		lines: unifiedDiff(oldText, newText), reply: reply,
	}
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

// unifiedDiff renders the change from oldText to newText as hunks.
func unifiedDiff(oldText, newText string) []propLine {
	old, cur := splitLines(oldText), splitLines(newText)
	blocks := diff.Lines(old, cur)
	var out []propLine
	for i := 0; i < len(blocks); {
		// group blocks whose context windows touch
		j := i
		for j+1 < len(blocks) && blocks[j+1].OldStart-blocks[j].OldEnd <= 2*diffContext {
			j++
		}
		first, last := blocks[i], blocks[j]
		oStart := max(0, first.OldStart-diffContext)
		oEnd := min(len(old), last.OldEnd+diffContext)
		nStart := first.NewStart - (first.OldStart - oStart)
		nEnd := last.NewEnd + (oEnd - last.OldEnd)
		out = append(out, propLine{'@', fmt.Sprintf("@@ -%d,%d +%d,%d @@", oStart+1, oEnd-oStart, nStart+1, nEnd-nStart)})
		o := oStart
		for _, b := range blocks[i : j+1] {
			for ; o < b.OldStart; o++ {
				out = append(out, propLine{' ', old[o]})
			}
			for ; o < b.OldEnd; o++ {
				out = append(out, propLine{'-', old[o]})
			}
			for n := b.NewStart; n < b.NewEnd; n++ {
				out = append(out, propLine{'+', cur[n]})
			}
		}
		for ; o < oEnd; o++ {
			out = append(out, propLine{' ', old[o]})
		}
		i = j + 1
	}
	if len(out) == 0 {
		out = append(out, propLine{' ', "(変更はありません)"})
	}
	return out
}

// resolveProposal answers the agent and drops the proposal. The editor never
// writes the file: that is left to Claude Code after FILE_SAVED.
func (a *App) resolveProposal(p *proposal, accept bool) {
	if accept {
		a.info("承認しました: %s", a.rel(p.newPath))
		p.reply <- ideReply{content: []ide.Content{ide.Text("FILE_SAVED"), ide.Text(p.newText)}}
		a.treeTicks = treeRefreshTicks - 2 // the client writes the file after this reply
	} else {
		a.info("却下しました: %s", a.rel(p.newPath))
		p.reply <- ideReply{content: []ide.Content{ide.Text("DIFF_REJECTED"), ide.Text(p.name)}}
	}
	a.props = slices.DeleteFunc(a.props, func(q *proposal) bool { return q == p })
}

// closeProposal drops a proposal the agent closed itself (close_tab,
// closeAllDiffTabs), typically after the decision was made on the Claude Code
// side. The pending request is still answered, but the editor rejected
// nothing, so the status bar must not say so.
func (a *App) closeProposal(p *proposal) {
	a.info("Claude Code 側で決まったので差分を閉じました: %s", a.rel(p.newPath))
	p.reply <- ideReply{content: []ide.Content{ide.Text("DIFF_REJECTED"), ide.Text(p.name)}}
	a.props = slices.DeleteFunc(a.props, func(q *proposal) bool { return q == p })
}

// cancelProposal drops the proposal whose client disconnected.
func (a *App) cancelProposal(reply chan ideReply) {
	a.props = slices.DeleteFunc(a.props, func(q *proposal) bool { return q.reply == reply })
}

// handleProposalKey drives the first pending proposal: Enter accepts, Esc
// rejects.
func (a *App) handleProposalKey(msg tea.KeyPressMsg) tea.Cmd {
	p := a.props[0]
	switch msg.String() {
	case "enter", "y":
		a.resolveProposal(p, true)
	case "esc", "ctrl+c", "n", "q":
		a.resolveProposal(p, false)
	case "ctrl+d", "pgdown", "down", "j":
		p.scroll++
	case "ctrl+u", "pgup", "up", "k":
		p.scroll = max(0, p.scroll-1)
	}
	return nil
}

func (p *proposal) render(a *App, w, maxH int) []string {
	body := max(1, maxH-3)
	p.scroll = clamp(p.scroll, 0, max(0, len(p.lines)-body))
	title := fmt.Sprintf(" エージェントの変更: %s（Enter: 適用 / Esc: 却下）", a.rel(p.newPath))
	if len(a.props) > 1 {
		title += fmt.Sprintf(" 他 %d 件待ち", len(a.props)-1)
	}
	out := []string{stPaletteBox.Render(fit(title, w))}
	for _, l := range p.lines[p.scroll:min(len(p.lines), p.scroll+body)] {
		text := fit(" "+string(l.kind)+" "+strings.ReplaceAll(l.text, "\t", "    "), w)
		if l.kind == '@' {
			text = fit(" "+l.text, w)
		}
		switch l.kind {
		case '+':
			out = append(out, lipgloss.NewStyle().Background(lipgloss.Color(cAddBg)).Render(text))
		case '-':
			out = append(out, lipgloss.NewStyle().Background(lipgloss.Color(cDelBg)).Render(text))
		case '@':
			out = append(out, stPaletteDim.Render(text))
		default:
			out = append(out, stPalette.Render(text))
		}
	}
	if len(p.lines) > body {
		out = append(out, stPaletteDim.Render(fit(fmt.Sprintf(" %d-%d / %d 行", p.scroll+1, min(len(p.lines), p.scroll+body), len(p.lines)), w)))
	}
	return out
}

func (a *App) renderProposal(rows []string) []string {
	w := clamp(a.w*4/5, 30, 140)
	box := a.props[0].render(a, w, max(4, a.h-4))
	if ansi.StringWidth(box[0]) > w {
		box[0] = ansi.Truncate(box[0], w, "")
	}
	return overlay(rows, box, (a.w-w)/2, 1)
}
