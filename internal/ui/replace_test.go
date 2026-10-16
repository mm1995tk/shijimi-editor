package ui

import (
	"testing"

	"github.com/mm1995tk/gh-editor/internal/buffer"
)

func TestSplitReplaceArgs(t *testing.T) {
	for _, c := range []struct {
		in, q, r string
		ok       bool
	}{
		{"foo => bar", "foo", "bar", true},
		{"-x *.lock foo bar => baz qux", "-x *.lock foo bar", "baz qux", true},
		{"foo =>", "foo", "", true},
		{"foo", "foo", "", false},
	} {
		q, r, ok := splitReplaceArgs(c.in)
		if q != c.q || r != c.r || ok != c.ok {
			t.Errorf("splitReplaceArgs(%q) = %q, %q, %v", c.in, q, r, ok)
		}
	}
}

func TestPlanLineAndApply(t *testing.T) {
	it, ok := planLine(literalRegexp("foo"), "あ Foo foo.bar", "X")
	if !ok || it.new != "あ X X.bar" || len(it.spans) != 2 {
		t.Fatalf("plan = %+v ok=%v", it, ok)
	}
	if _, ok := planLine(literalRegexp("Foo"), "foo", "X"); ok {
		t.Fatal("uppercase query must be case-sensitive")
	}
	if _, ok := planLine(literalRegexp("a.c"), "abc", "X"); ok {
		t.Fatal("query must be literal")
	}

	b := buffer.New()
	b.Insert("あ Foo foo.bar")
	b.Dirty = false
	var edits []buffer.Edit
	for _, sp := range it.spans {
		edits = append(edits, buffer.Edit{Start: buffer.Pos{Col: sp[0]}, End: buffer.Pos{Col: sp[1]}, Text: "X"})
	}
	b.ApplyEdits(edits)
	if got := b.Text(); got != it.new || !b.Dirty {
		t.Fatalf("text = %q dirty=%v", got, b.Dirty)
	}
	b.Undo()
	if got := b.Text(); got != "あ Foo foo.bar" {
		t.Fatalf("after undo = %q", got)
	}
}
