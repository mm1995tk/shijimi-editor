package ui

import (
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
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

func TestCodeActionRange(t *testing.T) {
	b := buffer.New()
	b.Insert("for x {\n\treturn\n}\nnext")
	diag := lsp.Diagnostic{Message: "loop", Range: lsp.Range{
		Start: lsp.Position{Line: 0, Character: 4},
		End:   lsp.Position{Line: 2, Character: 1},
	}}
	other := lsp.Diagnostic{Message: "other", Range: lsp.Range{
		Start: lsp.Position{Line: 3, Character: 0},
		End:   lsp.Position{Line: 3, Character: 4},
	}}
	all := []lsp.Diagnostic{diag, other}

	// Inside the diagnostic: ask at the cursor.
	b.Anchor = nil
	b.Cur = buffer.Pos{Row: 1, Col: 1}
	rng, ds := codeActionRange(b, all)
	if rng.Start != (lsp.Position{Line: 1, Character: 1}) || rng.End != rng.Start || len(ds) != 1 || ds[0].Message != "loop" {
		t.Errorf("inside: %+v %+v", rng, ds)
	}

	// Before the diagnostic on the same line: use the diagnostic's range.
	b.Cur = buffer.Pos{Row: 0, Col: 0}
	rng, ds = codeActionRange(b, all)
	if rng != diag.Range || len(ds) != 1 || ds[0].Message != "loop" {
		t.Errorf("same line: %+v %+v", rng, ds)
	}

	// A selection is used as is.
	b.Anchor = &buffer.Pos{Row: 3, Col: 0}
	b.Cur = buffer.Pos{Row: 3, Col: 2}
	rng, ds = codeActionRange(b, all)
	if rng.End != (lsp.Position{Line: 3, Character: 2}) || len(ds) != 1 || ds[0].Message != "other" {
		t.Errorf("selection: %+v %+v", rng, ds)
	}
}

func TestSymbolDetail(t *testing.T) {
	s := lsp.Symbol{Name: "Start", Kind: 6, Container: "Server", Range: lsp.Range{Start: lsp.Position{Line: 11}}}
	if got := symbolDetail(s, ""); got != "メソッド  Server  12行目" {
		t.Errorf("file: %q", got)
	}
	if got := symbolDetail(s, "a/b.go"); got != "メソッド  Server  a/b.go:12" {
		t.Errorf("project: %q", got)
	}
}
