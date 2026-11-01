package tui

import (
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
)

// The semantic colors go over chroma's, follow the theme, and the lines edited
// since keep chroma's colors only.
func TestSemanticOverlay(t *testing.T) {
	applyTheme(core.ThemeOrDefault("dark-plus"))
	b := buffer.New()
	b.Lines = [][]rune{[]rune("type S int"), []rune("var x int")}
	lines := []string{"type S int", "var x int"}
	h := core.NewHighlighter("a.go")
	h.Apply(core.HighlightMsg{H: h, Version: b.Version, Gen: core.ThemeGen, Lines: lines,
		Colors: core.LexColors(h.Lexer, core.ChromaStyle, lines)})
	h.Sem.Apply(core.SemanticMsg{Ok: true, Lines: lines, Roles: core.SemanticRoles(lines, []lsp.SemanticToken{
		{Line: 0, StartChar: 5, Length: 1, Type: "struct"},
		{Line: 1, StartChar: 4, Length: 1, Type: "variable"},
	})})
	c := h.Colors(b)
	if c[0][5] != "#4ec9b0" || c[1][4] != "#9cdcfe" {
		t.Fatalf("semantic colors missing: %q %q", c[0], c[1])
	}
	if c[0][0] == "" || c[0][0] != core.LexColors(h.Lexer, core.ChromaStyle, lines)[0][0] {
		t.Fatalf("chroma color lost: %q", c[0])
	}
	applyTheme(core.ThemeOrDefault("light-plus"))
	if c := h.Colors(b); c[0][5] != "#267f99" {
		t.Fatalf("theme switch not followed: %q", c[0][5])
	}
	applyTheme(core.ThemeOrDefault("dark-plus"))
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
