package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
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
		q, r, ok := SplitReplaceArgs(c.in)
		if q != c.q || r != c.r || ok != c.ok {
			t.Errorf("splitReplaceArgs(%q) = %q, %q, %v", c.in, q, r, ok)
		}
	}
}

func TestGlobMatcher(t *testing.T) {
	m := globMatcher([]string{"*.go", "dist/**", "vendor"})
	for p, want := range map[string]bool{
		"a.go": true, "x/y/a.go": true, "README.md": false,
		"dist/a/b.txt": true, "vendor/x/y.txt": true, "src/vendor/z": true, "src/a.txt": false,
	} {
		if got := m(p); got != want {
			t.Errorf("match(%q) = %v, want %v", p, got, want)
		}
	}
	if globMatcher(nil)("a.go") {
		t.Error("empty matcher must not match")
	}
}

func mustPlan(t *testing.T, o SearchOpts, line string) *SearchHit {
	t.Helper()
	re, err := o.compile()
	if err != nil {
		t.Fatal(err)
	}
	h, ok := planHit(re, o, line)
	if !ok {
		t.Fatalf("%q must match %+v", line, o)
	}
	return h
}

func noMatch(t *testing.T, o SearchOpts, line string) {
	t.Helper()
	re, err := o.compile()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := planHit(re, o, line); ok {
		t.Fatalf("%q must not match %+v", line, o)
	}
}

func TestPlanHitAndApply(t *testing.T) {
	h := mustPlan(t, SearchOpts{Query: "foo", Repl: "X"}, "あ Foo foo.bar")
	if h.New != "あ X X.bar" || len(h.Spans) != 2 {
		t.Fatalf("plan = %+v", h)
	}
	noMatch(t, SearchOpts{Query: "Foo", CaseSens: true}, "foo")
	noMatch(t, SearchOpts{Query: "a.c"}, "abc")

	b := buffer.New()
	b.Insert("あ Foo foo.bar")
	b.Dirty = false
	edits, n, stale := HitEdits([]*SearchHit{h}, b.Lines)
	if n != 1 || stale != 0 {
		t.Fatalf("n=%d stale=%d", n, stale)
	}
	b.ApplyEdits(edits)
	if got := b.Text(); got != h.New || !b.Dirty {
		t.Fatalf("text = %q dirty=%v", got, b.Dirty)
	}
	b.Undo()
	if got := b.Text(); got != "あ Foo foo.bar" {
		t.Fatalf("after undo = %q", got)
	}
}

func TestSearchWordRegexKeepCase(t *testing.T) {
	noMatch(t, SearchOpts{Query: "foo", Word: true}, "foobar")
	mustPlan(t, SearchOpts{Query: "検索", Word: true}, "全文 検索 窓")
	noMatch(t, SearchOpts{Query: "検索", Word: true}, "全文検索")
	noMatch(t, SearchOpts{Query: "cafe", Word: true}, "cafeé")
	mustPlan(t, SearchOpts{Query: "foo", Word: true}, "a foo b")

	h := mustPlan(t, SearchOpts{Query: `(\w+)@(\w+)`, Repl: "$2:$1", Regex: true}, "x a@b y")
	if h.New != "x b:a y" {
		t.Fatalf("capture = %q", h.New)
	}
	// `$1` is literal when the regex toggle is off
	h = mustPlan(t, SearchOpts{Query: "a", Repl: "$1"}, "a")
	if h.New != "$1" {
		t.Fatalf("literal = %q", h.New)
	}

	h = mustPlan(t, SearchOpts{Query: "hello", Repl: "world", KeepCase: true}, "hello Hello HELLO hElLo")
	if h.New != "world World WORLD world" {
		t.Fatalf("keep case = %q", h.New)
	}

	if _, err := (SearchOpts{Query: "(", Regex: true}).compile(); err == nil {
		t.Fatal("invalid regex must fail")
	}
}

func TestRunSearch(t *testing.T) {
	dir := t.TempDir()
	write := func(name, s string) {
		p := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(s), 0o644)
	}
	write("a.go", "package a\nfunc TestA() {}\n")
	write("b.go", "func TestB() {}\n")
	write("gen/c.go", "func TestC() {}\n")
	write("d.lock", "TestD\n")
	write("bin.dat", "Test\x00\x01")
	cancel := func() bool { return false }

	res := RunSearch(dir, SearchOpts{Query: "test", Exclude: []string{"gen/**", "*.lock"}}, nil, cancel)
	if res.Err != nil || res.Files != 2 || len(res.Hits) != 2 {
		t.Fatalf("res = %+v", res)
	}
	res = RunSearch(dir, SearchOpts{Query: "test", Include: []string{"*.go"}}, nil, cancel)
	if res.Files != 3 {
		t.Fatalf("include files = %d", res.Files)
	}
	// unsaved buffer content wins over the disk
	open := map[string][]string{filepath.Join(dir, "b.go"): {"nothing here"}}
	res = RunSearch(dir, SearchOpts{Query: "TestB"}, open, cancel)
	if len(res.Hits) != 0 {
		t.Fatalf("hits = %+v", res.Hits)
	}
	res = RunSearch(dir, SearchOpts{Query: "("}, nil, cancel)
	if len(res.Hits) == 0 {
		t.Fatal("literal parenthesis must match")
	}
	if res := RunSearch(dir, SearchOpts{Query: "("}, nil, func() bool { return true }); len(res.Hits) != 0 {
		t.Fatal("cancelled search must stop")
	}
}

func TestPreserveCase(t *testing.T) {
	for _, c := range []struct{ matched, repl, want string }{
		{"foo", "bar", "bar"},
		{"FOO", "bar", "BAR"},
		{"Foo", "bar", "Bar"},
		{"fOo", "bar", "bar"},
		{"A", "bar", "Bar"},
		{"123", "bar", "bar"},
	} {
		if got := preserveCase(c.matched, c.repl); got != c.want {
			t.Errorf("preserveCase(%q, %q) = %q, want %q", c.matched, c.repl, got, c.want)
		}
	}
}
