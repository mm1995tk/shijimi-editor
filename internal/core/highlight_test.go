package core

import (
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
	a.Update(Resized{W: 100, H: 30})
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
