package core

import (
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
)

// Token positions are UTF-16; the roles are per rune.
func TestSemanticRolesSurrogatePair(t *testing.T) {
	// UTF-16: a=0, 😀=1-2, b=3, c=4
	got := SemanticRoles([]string{"a😀bc"}, []lsp.SemanticToken{
		{Line: 0, StartChar: 3, Length: 2, Type: "function"},
	})
	want := []string{"", "", "Function", "Function"}
	for i, w := range want {
		if got[0][i] != w {
			t.Fatalf("roles %q, want %q", got[0], want)
		}
	}
}

func TestSemanticRolesJapanese(t *testing.T) {
	got := SemanticRoles([]string{"あいう := 1"}, []lsp.SemanticToken{
		{Line: 0, StartChar: 0, Length: 3, Type: "variable"},
	})
	if got[0][0] != "Name" || got[0][2] != "Name" || got[0][3] != "" {
		t.Fatalf("%q", got[0])
	}
}

func TestSemanticRole(t *testing.T) {
	cases := []struct {
		tok  lsp.SemanticToken
		text string
		want string
	}{
		{lsp.SemanticToken{Type: "struct"}, "S", "Type"},
		{lsp.SemanticToken{Type: "keyword"}, "return", "Control"},
		{lsp.SemanticToken{Type: "keyword"}, "func", "Keyword"},
		{lsp.SemanticToken{Type: "variable", Modifiers: []string{"readonly"}}, "x", "Const"},
		{lsp.SemanticToken{Type: "variable", Modifiers: []string{"readonly", "defaultLibrary"}}, "nil", ""},
		{lsp.SemanticToken{Type: "string"}, "x", ""},
		{lsp.SemanticToken{Type: "method"}, "M", "Function"},
	}
	for _, c := range cases {
		if got := semanticRole(c.tok, c.text); got != c.want {
			t.Errorf("%v %q = %q, want %q", c.tok, c.text, got, c.want)
		}
	}
}

// The semantic colors go over chroma's, follow the theme, and the lines edited
// since keep chroma's colors only.
func TestSemanticOverlay(t *testing.T) {
	ActivateTheme(ThemeOrDefault("dark-plus"))
	b := buffer.New()
	b.Lines = [][]rune{[]rune("type S int"), []rune("var x int")}
	lines := []string{"type S int", "var x int"}
	h := NewHighlighter("a.go")
	h.Apply(HighlightMsg{H: h, Version: b.Version, Gen: ThemeGen, Lines: lines,
		Colors: LexColors(h.Lexer, ChromaStyle, lines)})
	h.Sem.Apply(SemanticMsg{Ok: true, Lines: lines, Roles: SemanticRoles(lines, []lsp.SemanticToken{
		{Line: 0, StartChar: 5, Length: 1, Type: "struct"},
		{Line: 1, StartChar: 4, Length: 1, Type: "variable"},
	})})
	c := h.Colors(b)
	if c[0][5] != "#4ec9b0" || c[1][4] != "#9cdcfe" {
		t.Fatalf("semantic colors missing: %q %q", c[0], c[1])
	}
	if c[0][0] == "" || c[0][0] != LexColors(h.Lexer, ChromaStyle, lines)[0][0] {
		t.Fatalf("chroma color lost: %q", c[0])
	}
	ActivateTheme(ThemeOrDefault("light-plus"))
	if c := h.Colors(b); c[0][5] != "#267f99" {
		t.Fatalf("theme switch not followed: %q", c[0][5])
	}
	ActivateTheme(ThemeOrDefault("dark-plus"))
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
