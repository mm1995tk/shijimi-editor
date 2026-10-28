package ui

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
)

// semanticState holds the last semantic tokens of a buffer, drawn over the
// chroma colors. They are kept as roles (names of syntaxColors fields), not
// colors, so that a theme switch recolors them without asking the server again.
type semanticState struct {
	running bool
	version int // buffer version requested last, valid if asked
	asked   bool

	lines []string   // the lines the tokens are for
	roles [][]string // per-rune role of lines ("" = keep the chroma color)

	// cache of the merged colors
	stamp                  int // changes when roles or the chroma colors change
	cVersion, cStamp, cGen int
	cValid                 bool
	merged                 [][]string
}

// semanticMsg carries the result of a semantic tokens request.
type semanticMsg struct {
	h       *highlighter
	version int
	lines   []string
	roles   [][]string
	ok      bool // the server answered; false on errors and unsupported servers
}

// semanticCmd returns a command that asks the language server for the
// semantic tokens of t's current text, or nil when they are current, a request
// is under way, or no server provides them. Edits made meanwhile are requested
// once that one is applied.
func (a *App) semanticCmd(t *tab) tea.Cmd {
	h := t.hl
	s := &h.sem
	if t.kind != tabText || t.lang == nil || s.running || len(t.buf.Lines) > maxHighlightLines ||
		s.asked && s.version == t.buf.Version || t.lspVersion != t.buf.Version {
		return nil
	}
	c := a.lsp.client(t, "semanticTokensProvider")
	if c == nil {
		return nil
	}
	s.running, s.asked, s.version = true, true, t.buf.Version
	lines := make([]string, len(t.buf.Lines))
	for i, l := range t.buf.Lines {
		lines[i] = string(l)
	}
	path, version := t.buf.Path, t.buf.Version
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), lspTimeout)
		defer cancel()
		toks, err := c.SemanticTokens(ctx, path)
		m := semanticMsg{h: h, version: version, lines: lines}
		if err == nil && toks != nil {
			m.roles, m.ok = semanticRoles(lines, toks), true
		}
		return m
	}
}

// semanticTabs requests semantic tokens for every text tab that is behind.
func (a *App) semanticTabs() tea.Cmd {
	var cmds []tea.Cmd
	for _, t := range a.tabs {
		cmds = append(cmds, a.semanticCmd(t))
	}
	return tea.Batch(cmds...)
}

// apply stores the result of a semanticCmd. A server that stops answering
// leaves the last tokens in place.
func (s *semanticState) apply(m semanticMsg) {
	s.running = false
	if !m.ok {
		return
	}
	s.lines, s.roles = m.lines, m.roles
	s.stamp++
}

// overlay draws the semantic roles over base (chroma colors, possibly nil).
// Lines edited since the tokens were taken have no roles, like alignColors.
func (s *semanticState) overlay(b *buffer.Buffer, base [][]string) [][]string {
	if s.cValid && s.cVersion == b.Version && s.cStamp == s.stamp && s.cGen == themeGen {
		return s.merged
	}
	roles := s.roles
	if len(s.lines) != len(b.Lines) || !sameLines(s.lines, b.Lines) {
		roles = alignColors(s.lines, s.roles, b.Lines)
	}
	out := make([][]string, len(b.Lines))
	for i, l := range b.Lines {
		var row []string
		if i < len(base) {
			row = base[i]
		}
		if i >= len(roles) || roles[i] == nil || len(roles[i]) != len(l) {
			out[i] = row
			continue
		}
		merged := make([]string, len(l))
		copy(merged, row)
		for j, role := range roles[i] {
			if c := semanticColor(role); c != "" {
				merged[j] = c
			}
		}
		out[i] = merged
	}
	s.cValid, s.cVersion, s.cStamp, s.cGen, s.merged = true, b.Version, s.stamp, themeGen, out
	return out
}

func sameLines(lexed []string, lines [][]rune) bool {
	for i, l := range lexed {
		if l != string(lines[i]) {
			return false
		}
	}
	return true
}

// semanticColor returns the current theme's color for a role.
func semanticColor(role string) string {
	s := curTheme.Syntax
	switch role {
	case "Type":
		return s.Type
	case "Name":
		return s.Name
	case "Function":
		return s.Function
	case "Const":
		return s.Const
	case "Keyword":
		return s.Keyword
	case "Control":
		if s.Control != "" {
			return s.Control
		}
		return s.Keyword
	}
	return ""
}

// controlKeywords are the keywords VS Code colors as control flow (purple in
// Dark+) rather than as declarations (blue).
var controlKeywords = map[string]bool{
	"if": true, "else": true, "elif": true, "for": true, "while": true, "do": true,
	"switch": true, "case": true, "default": true, "match": true, "return": true,
	"break": true, "continue": true, "goto": true, "select": true, "defer": true,
	"go": true, "fallthrough": true, "range": true, "try": true, "catch": true,
	"finally": true, "throw": true, "yield": true, "await": true, "import": true,
	"from": true, "export": true, "except": true, "raise": true, "with": true,
}

// semanticRole maps a token to a role, or "" to keep the chroma color. Only
// kinds whose color chroma cannot tell are mapped; strings, numbers and
// comments stay as chroma colors them (it knows escapes).
func semanticRole(t lsp.SemanticToken, text string) string {
	has := func(m string) bool {
		for _, x := range t.Modifiers {
			if x == m {
				return true
			}
		}
		return false
	}
	switch t.Type {
	case "namespace", "type", "class", "enum", "interface", "struct", "typeParameter":
		return "Type"
	case "parameter", "property":
		return "Name"
	case "variable":
		if has("defaultLibrary") {
			return "" // nil, true, iota: chroma colors them as keywords
		}
		if has("readonly") {
			return "Const"
		}
		return "Name"
	case "enumMember":
		return "Const"
	case "function", "method", "macro":
		return "Function"
	case "keyword":
		if controlKeywords[text] {
			return "Control"
		}
		return "Keyword"
	case "modifier":
		return "Keyword"
	}
	return ""
}

// semanticRoles turns tokens into per-rune roles of lines. The tokens'
// positions are in UTF-16 code units; they are converted to rune columns here.
// Tokens without a type, outside the text, or spanning lines are skipped.
func semanticRoles(lines []string, toks []lsp.SemanticToken) [][]string {
	out := make([][]string, len(lines))
	runes := make([][]rune, len(lines))
	for _, t := range toks {
		if t.Type == "" || t.Length <= 0 || t.Line < 0 || t.Line >= len(lines) {
			continue
		}
		if runes[t.Line] == nil {
			runes[t.Line] = []rune(lines[t.Line])
			out[t.Line] = make([]string, len(runes[t.Line]))
		}
		l := runes[t.Line]
		start := lsp.FromUTF16(l, t.StartChar)
		end := lsp.FromUTF16(l, t.StartChar+t.Length)
		if start >= end || end > len(l) {
			continue
		}
		role := semanticRole(t, strings.TrimSpace(string(l[start:end])))
		if role == "" {
			continue
		}
		for i := start; i < end; i++ {
			out[t.Line][i] = role
		}
	}
	return out
}
