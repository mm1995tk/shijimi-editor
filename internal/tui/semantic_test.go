package tui

import (
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
)

// The semantic colors go over chroma's, follow the theme, and the lines edited
// since keep chroma's colors only.
func TestSemanticOverlay(t *testing.T) {
	applyTheme(themeOrDefault("dark-plus"))
	b := buffer.New()
	b.Lines = [][]rune{[]rune("type S int"), []rune("var x int")}
	lines := []string{"type S int", "var x int"}
	h := newHighlighter("a.go")
	h.apply(highlightMsg{h: h, version: b.Version, gen: ThemeGen, lines: lines,
		colors: lexColors(h.lexer, chromaStyle, lines)})
	h.sem.apply(semanticMsg{ok: true, lines: lines, roles: semanticRoles(lines, []lsp.SemanticToken{
		{Line: 0, StartChar: 5, Length: 1, Type: "struct"},
		{Line: 1, StartChar: 4, Length: 1, Type: "variable"},
	})})
	c := h.Colors(b)
	if c[0][5] != "#4ec9b0" || c[1][4] != "#9cdcfe" {
		t.Fatalf("semantic colors missing: %q %q", c[0], c[1])
	}
	if c[0][0] == "" || c[0][0] != lexColors(h.lexer, chromaStyle, lines)[0][0] {
		t.Fatalf("chroma color lost: %q", c[0])
	}
	applyTheme(themeOrDefault("light-plus"))
	if c := h.Colors(b); c[0][5] != "#267f99" {
		t.Fatalf("theme switch not followed: %q", c[0][5])
	}
	applyTheme(themeOrDefault("dark-plus"))
	b.Version++
	b.Lines[1] = []rune("var y int")
	c = h.Colors(b)
	if c[0][5] != "#4ec9b0" {
		t.Fatalf("unchanged line lost its color: %q", c[0])
	}
	if c[1] != nil && c[1][4] != "" {
		t.Fatalf("edited line kept an old color: %q", c[1])
	}
}
