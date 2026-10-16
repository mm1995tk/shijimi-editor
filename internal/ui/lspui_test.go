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

func TestWithoutLocation(t *testing.T) {
	at := func(line, c0, c1 int) lsp.Location {
		return lsp.Location{URI: lsp.URI("/a.go"), Range: lsp.Range{
			Start: lsp.Position{Line: line, Character: c0},
			End:   lsp.Position{Line: line, Character: c1},
		}}
	}
	pos := lsp.Position{Line: 3, Character: 7}
	if got := withoutLocation([]lsp.Location{at(3, 5, 10)}, "/a.go", pos); len(got) != 0 {
		t.Errorf("declaration only: got %v, want empty", got)
	}
	got := withoutLocation([]lsp.Location{at(3, 5, 10), at(8, 1, 6)}, "/a.go", pos)
	if len(got) != 1 || got[0].Range.Start.Line != 8 {
		t.Errorf("got %v, want only the usage on line 8", got)
	}
}
