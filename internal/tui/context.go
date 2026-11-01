package tui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
)

// ctxPreview is the confirmation box shown before the context is pasted.
type ctxPreview struct {
	text   string
	scroll int
}

// maxContextLines caps how much of a selection is sent; the rest is elided.
const maxContextLines = 200

func diagSeverity() map[int]string {
	return map[int]string{1: i18n.T("diag.error"), 2: i18n.T("diag.warning"), 3: i18n.T("diag.info"), 4: i18n.T("diag.hint")}
}

// formatContext renders the current selection (or cursor line) as
// "path:行-行", the selected text in a fence, and the diagnostics that touch
// those lines. rows are 0-based and inclusive.
func formatContext(path string, startRow, endRow int, selected string, diags []lsp.Diagnostic) string {
	var sb strings.Builder
	if startRow == endRow {
		fmt.Fprintf(&sb, "%s:%d", path, startRow+1)
	} else {
		fmt.Fprintf(&sb, "%s:%d-%d", path, startRow+1, endRow+1)
	}
	sb.WriteByte('\n')
	if selected != "" {
		lines := strings.Split(selected, "\n")
		if len(lines) > maxContextLines {
			rest := len(lines) - maxContextLines
			lines = append(lines[:maxContextLines], fmt.Sprintf(i18n.T("ctx.elided"), rest))
		}
		sb.WriteString("```\n" + strings.Join(lines, "\n") + "\n```\n")
	}
	for _, d := range diags {
		if d.Range.End.Line < startRow || d.Range.Start.Line > endRow {
			continue
		}
		name := diagSeverity()[sevRank(d.Severity)]
		if name == "" {
			name = i18n.T("ctx.diagnostics")
		}
		src := ""
		if d.Source != "" {
			src = " (" + d.Source + ")"
		}
		fmt.Fprintf(&sb, "%s:%d: [%s] %s%s\n", path, d.Range.Start.Line+1, name, firstLine(d.Message), src)
	}
	return sb.String()
}

// currentContext builds the context text for the active text tab.
func (a *Core) currentContext() (string, bool) {
	t := a.curText()
	if t == nil || t.buf.Path == "" {
		return "", false
	}
	b := t.buf
	path := a.rel(b.Path)
	if s, e, ok := b.Selection(); ok {
		selected := b.SelectedText()
		endRow := e.Row
		if e.Col == 0 && endRow > s.Row {
			endRow-- // a selection ending at a line start does not include that line
			selected = strings.TrimSuffix(selected, "\n")
		}
		return formatContext(path, s.Row, endRow, selected, a.lsp.diagnostics(b.Path)), true
	}
	return formatContext(path, b.Cur.Row, b.Cur.Row, "", a.lsp.diagnostics(b.Path)), true
}

// sendContext tells a connected IDE-integrated agent about the current
// selection; otherwise it opens the preview of the context to paste.
func (a *Core) sendContext() []Effect {
	if a.single {
		return nil
	}
	if a.mentionContext() {
		return nil
	}
	text, ok := a.currentContext()
	if !ok {
		a.fail(errors.New(i18n.T("ctx.savefirst")))
		return nil
	}
	a.ctx = &ctxPreview{text: text}
	return nil
}

// handleContextKey drives the preview: Enter pastes, Esc cancels.
func (a *Core) handleContextKey(msg Key) []Effect {
	switch msg.String() {
	case "enter":
		text := a.ctx.text
		a.ctx = nil
		if a.muxOn() {
			return a.muxSend(text, false)
		}
		a.setTerm(true)
		cmd := a.setFocus(focusTerminal)
		return batch(cmd, a.termPaste(text))
	case "esc", "ctrl+c", "q":
		a.ctx = nil
	case "ctrl+d", "pgdown", "down", "j":
		a.ctx.scroll++
	case "ctrl+u", "pgup", "up", "k":
		a.ctx.scroll = max(0, a.ctx.scroll-1)
	}
	return nil
}

func (c *ctxPreview) render(w, maxH int) []string {
	lines := strings.Split(strings.TrimRight(c.text, "\n"), "\n")
	body := max(1, maxH-3)
	c.scroll = clamp(c.scroll, 0, max(0, len(lines)-body))
	out := []string{stPaletteBox.Render(fit(i18n.T("ctx.title"), w))}
	for _, l := range lines[c.scroll:min(len(lines), c.scroll+body)] {
		out = append(out, stPalette.Render(fit(" "+strings.ReplaceAll(l, "\t", "    "), w)))
	}
	if len(lines) > body {
		out = append(out, stPaletteDim.Render(fit(fmt.Sprintf(i18n.T("ctx.lines"), c.scroll+1, min(len(lines), c.scroll+body), len(lines)), w)))
	}
	return out
}

func (a *Core) renderContext(rows []string) []string {
	w := clamp(a.screenW()*3/5, 30, 100)
	box := a.ctx.render(w, max(4, a.screenH()-4))
	if ansi.StringWidth(box[0]) > w {
		box[0] = ansi.Truncate(box[0], w, "")
	}
	return overlay(rows, box, (a.screenW()-w)/2, 1)
}
