package core

import (
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/lsp"
)

// Token positions are UTF-16; the roles are per rune.
func TestSemanticRolesSurrogatePair(t *testing.T) {
	// UTF-16: a=0, 😀=1-2, b=3, c=4
	got := semanticRoles([]string{"a😀bc"}, []lsp.SemanticToken{
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
	got := semanticRoles([]string{"あいう := 1"}, []lsp.SemanticToken{
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
