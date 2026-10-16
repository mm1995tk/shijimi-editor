package ui

import (
	"testing"

	"github.com/mm1995tk/gh-editor/internal/lsp"
)

func TestOnLocation(t *testing.T) {
	loc := lsp.Location{URI: lsp.URI("/a.go"), Range: lsp.Range{
		Start: lsp.Position{Line: 3, Character: 5},
		End:   lsp.Position{Line: 3, Character: 10},
	}}
	cases := []struct {
		path string
		pos  lsp.Position
		want bool
	}{
		{"/a.go", lsp.Position{Line: 3, Character: 5}, true},
		{"/a.go", lsp.Position{Line: 3, Character: 7}, true},
		{"/a.go", lsp.Position{Line: 3, Character: 10}, true},
		{"/a.go", lsp.Position{Line: 3, Character: 4}, false},
		{"/a.go", lsp.Position{Line: 3, Character: 11}, false},
		{"/a.go", lsp.Position{Line: 2, Character: 7}, false},
		{"/b.go", lsp.Position{Line: 3, Character: 7}, false},
	}
	for _, c := range cases {
		if got := onLocation([]lsp.Location{loc}, c.path, c.pos); got != c.want {
			t.Errorf("%s %+v: got %v, want %v", c.path, c.pos, got, c.want)
		}
	}
}
