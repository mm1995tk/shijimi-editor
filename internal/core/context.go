package core

import (
	"fmt"
	"strings"

	"github.com/mm1995tk/shijimi-editor/internal/lsp"
	"github.com/mm1995tk/shijimi-editor/internal/msg"
)

// CtxPreview is the confirmation box shown before the context is pasted.
type CtxPreview struct {
	Text   string
	Scroll int
}

// maxContextLines caps how much of a selection is sent; the rest is elided.
const maxContextLines = 200

func diagSeverity() map[int]msg.Msg {
	return map[int]msg.Msg{1: msg.T("diag.error"), 2: msg.T("diag.warning"), 3: msg.T("diag.info"), 4: msg.T("diag.hint")}
}

// formatContext renders the current selection (or cursor line) as
// "path:行-行", the selected text in a fence, and the diagnostics that touch
// those lines. rows are 0-based and inclusive.
func (a *Core) formatContext(path string, startRow, endRow int, selected string, diags []lsp.Diagnostic) string {
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
			lines = append(lines[:maxContextLines], a.text(msg.T("ctx.elided", rest)))
		}
		sb.WriteString("```\n" + strings.Join(lines, "\n") + "\n```\n")
	}
	for _, d := range diags {
		if d.Range.End.Line < startRow || d.Range.Start.Line > endRow {
			continue
		}
		name := a.text(diagSeverity()[sevRank(d.Severity)])
		if name == "" {
			name = a.text(msg.T("ctx.diagnostics"))
		}
		src := ""
		if d.Source != "" {
			src = " (" + d.Source + ")"
		}
		fmt.Fprintf(&sb, "%s:%d: [%s] %s%s\n", path, d.Range.Start.Line+1, name, FirstLine(d.Message), src)
	}
	return sb.String()
}

// currentContext builds the context text for the active text tab.
func (a *Core) currentContext() (string, bool) {
	t := a.CurText()
	if t == nil || t.Buf.Path == "" {
		return "", false
	}
	b := t.Buf
	path := a.Rel(b.Path)
	if s, e, ok := b.Selection(); ok {
		selected := b.SelectedText()
		endRow := e.Row
		if e.Col == 0 && endRow > s.Row {
			endRow-- // a selection ending at a line start does not include that line
			selected = strings.TrimSuffix(selected, "\n")
		}
		return a.formatContext(path, s.Row, endRow, selected, a.Lsp.diagnostics(b.Path)), true
	}
	return a.formatContext(path, b.Cur.Row, b.Cur.Row, "", a.Lsp.diagnostics(b.Path)), true
}

// SendContext tells a connected IDE-integrated agent about the current
// selection; otherwise it opens the preview of the context to paste.
func (a *Core) SendContext() []Effect {
	if a.single {
		return nil
	}
	if a.mentionContext() {
		return nil
	}
	text, ok := a.currentContext()
	if !ok {
		a.Fail(msg.Err("ctx.savefirst"))
		return nil
	}
	a.Ctx = &CtxPreview{Text: text}
	return nil
}

// HandleContextKey drives the preview: Enter pastes, Esc cancels.
func (a *Core) HandleContextKey(msg Key) []Effect {
	switch msg.String() {
	case "enter":
		text := a.Ctx.Text
		a.Ctx = nil
		if !a.TermInPane() {
			a.SetTerm(true)
			a.SetFocus(FocusTerminal)
		}
		a.port.TermPaste(text)
		return nil
	case "esc", "ctrl+c", "q":
		a.Ctx = nil
	case "ctrl+d", "pgdown", "down", "j":
		a.Ctx.Scroll++
	case "ctrl+u", "pgup", "up", "k":
		a.Ctx.Scroll = max(0, a.Ctx.Scroll-1)
	}
	return nil
}
