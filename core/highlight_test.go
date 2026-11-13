package core

import (
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"os"
	"path/filepath"
	"testing"
)

func newGoApp(t *testing.T, src string) (*App, *Tab) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "a.go")
	os.WriteFile(p, []byte(src), 0o644)
	a := newApp(dir, p)
	a.Helix = true
	resized(a, 100, 30)
	return a, a.CurText()
}

// Until an edit is lexed, unchanged lines keep their colors and edited ones
// have none, even when lines were inserted above.
func TestAlignColors(t *testing.T) {
	lexed := []string{"a", "b", "c"}
	colors := [][]string{{"1"}, {"2"}, {"3"}}
	got := alignColors(lexed, colors, [][]rune{[]rune("a"), []rune("x"), []rune("y"), []rune("c")})
	want := [][]string{{"1"}, nil, nil, {"3"}}
	for i := range want {
		if len(got[i]) != len(want[i]) || len(got[i]) > 0 && got[i][0] != want[i][0] {
			t.Fatalf("line %d = %v, want %v", i, got[i], want[i])
		}
	}
}

// chroma's Go lexer gives every "//x" comment the preprocessor color; only
// directives keep it (#634).
func TestLexColorsGoComment(t *testing.T) {
	style := styles.Get("github-dark")
	pre := style.Get(chroma.CommentPreproc).Colour.String()
	com := style.Get(chroma.CommentSingle).Colour.String()
	if pre == com {
		t.Skip("style does not distinguish the two")
	}
	lines := []string{
		"//RecordMain は x", "// RecordMain は x", "//go:build linux",
		"//go:generate stringer", "//line a.go:1", "//export Foo", "//http://x",
	}
	want := []string{com, com, pre, pre, pre, pre, com}
	got := LexColors(lexers.Get("go"), style, lines)
	for i := range lines {
		if got[i][0] != want[i] {
			t.Errorf("%q: color %q, want %q", lines[i], got[i][0], want[i])
		}
	}
}
