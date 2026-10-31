package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

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
		q, r, ok := splitReplaceArgs(c.in)
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

func mustPlan(t *testing.T, o searchOpts, line string) *searchHit {
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

func noMatch(t *testing.T, o searchOpts, line string) {
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
	h := mustPlan(t, searchOpts{query: "foo", repl: "X"}, "あ Foo foo.bar")
	if h.new != "あ X X.bar" || len(h.spans) != 2 {
		t.Fatalf("plan = %+v", h)
	}
	noMatch(t, searchOpts{query: "Foo", caseSens: true}, "foo")
	noMatch(t, searchOpts{query: "a.c"}, "abc")

	b := buffer.New()
	b.Insert("あ Foo foo.bar")
	b.Dirty = false
	edits, n, stale := hitEdits([]*searchHit{h}, b.Lines)
	if n != 1 || stale != 0 {
		t.Fatalf("n=%d stale=%d", n, stale)
	}
	b.ApplyEdits(edits)
	if got := b.Text(); got != h.new || !b.Dirty {
		t.Fatalf("text = %q dirty=%v", got, b.Dirty)
	}
	b.Undo()
	if got := b.Text(); got != "あ Foo foo.bar" {
		t.Fatalf("after undo = %q", got)
	}
}

func TestSearchWordRegexKeepCase(t *testing.T) {
	noMatch(t, searchOpts{query: "foo", word: true}, "foobar")
	mustPlan(t, searchOpts{query: "検索", word: true}, "全文 検索 窓")
	noMatch(t, searchOpts{query: "検索", word: true}, "全文検索")
	noMatch(t, searchOpts{query: "cafe", word: true}, "cafeé")
	mustPlan(t, searchOpts{query: "foo", word: true}, "a foo b")

	h := mustPlan(t, searchOpts{query: `(\w+)@(\w+)`, repl: "$2:$1", regex: true}, "x a@b y")
	if h.new != "x b:a y" {
		t.Fatalf("capture = %q", h.new)
	}
	// `$1` is literal when the regex toggle is off
	h = mustPlan(t, searchOpts{query: "a", repl: "$1"}, "a")
	if h.new != "$1" {
		t.Fatalf("literal = %q", h.new)
	}

	h = mustPlan(t, searchOpts{query: "hello", repl: "world", keepCase: true}, "hello Hello HELLO hElLo")
	if h.new != "world World WORLD world" {
		t.Fatalf("keep case = %q", h.new)
	}

	if _, err := (searchOpts{query: "(", regex: true}).compile(); err == nil {
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

	res := runSearch(dir, searchOpts{query: "test", exclude: []string{"gen/**", "*.lock"}}, nil, cancel)
	if res.err != nil || res.files != 2 || len(res.hits) != 2 {
		t.Fatalf("res = %+v", res)
	}
	res = runSearch(dir, searchOpts{query: "test", include: []string{"*.go"}}, nil, cancel)
	if res.files != 3 {
		t.Fatalf("include files = %d", res.files)
	}
	// unsaved buffer content wins over the disk
	open := map[string][]string{filepath.Join(dir, "b.go"): {"nothing here"}}
	res = runSearch(dir, searchOpts{query: "TestB"}, open, cancel)
	if len(res.hits) != 0 {
		t.Fatalf("hits = %+v", res.hits)
	}
	res = runSearch(dir, searchOpts{query: "("}, nil, cancel)
	if len(res.hits) == 0 {
		t.Fatal("literal parenthesis must match")
	}
	if res := runSearch(dir, searchOpts{query: "("}, nil, func() bool { return true }); len(res.hits) != 0 {
		t.Fatal("cancelled search must stop")
	}
}

// drive feeds the messages produced by cmd back into the app until it settles.
func drive(a *App, cmd tea.Cmd) {
	for cmd != nil {
		msg := cmd()
		if b, ok := msg.(tea.BatchMsg); ok {
			for _, c := range b {
				drive(a, c)
			}
			return
		}
		if msg == nil {
			return
		}
		_, cmd = a.Update(msg)
	}
}

func TestSearchPanelFlow(t *testing.T) {
	a := newTestApp(t, "foo one\nbar\nfoo two\n")
	other := filepath.Join(a.root, "b.txt")
	os.WriteFile(other, []byte("foo three\n"), 0o644)

	drive(a, a.openSearchArgs("foo => X", true))
	if !a.search.open {
		t.Fatal("panel must open")
	}
	if got := len(a.search.res.hits); got != 3 {
		t.Fatalf("hits = %d", got)
	}
	if got := ansi.Strip(strings.Join(a.renderPanelForTest(), "\n")); !strings.Contains(got, "3 件") || !strings.Contains(got, "置換対象 3 件") {
		t.Fatalf("panel = \n%s", got)
	}

	// leave the first line out, then replace the rest
	a.handleSearchKey(Key{Code: 'x', Mod: ModCtrl})
	if !a.search.res.hits[0].skip {
		t.Fatal("ctrl+x must exclude the line")
	}
	drive(a, a.handleSearchKey(Key{Code: KeyEnter, Mod: ModAlt}))
	if a.search.open {
		t.Fatal("panel must close after replacing")
	}
	var text string
	for _, tb := range a.tabs {
		if tb.kind == tabText && tb.buf.Path == filepath.Join(a.root, "a.txt") {
			text = tb.buf.Text()
			tb.buf.Undo()
			if tb.buf.Text() != "foo one\nbar\nfoo two\n" && tb.buf.Text() != "foo one\nbar\nfoo two" {
				t.Fatalf("undo = %q", tb.buf.Text())
			}
		}
	}
	if !strings.HasPrefix(text, "foo one\nbar\nX two") {
		t.Fatalf("text = %q", text)
	}
	if b, _ := os.ReadFile(other); string(b) != "foo three\n" {
		t.Fatal("disk must not change before saving")
	}
}

func TestSearchPanelOpenHit(t *testing.T) {
	a := newTestApp(t, "one\ntwo\nthree needle\n")
	drive(a, a.openSearchArgs("needle", false))
	if len(a.search.res.hits) != 1 {
		t.Fatalf("hits = %+v", a.search.res.hits)
	}
	a.handleSearchKey(Key{Code: KeyEnter})
	b := a.curText().buf
	if a.search.open || b.Cur.Row != 2 || b.Cur.Col != 6 {
		t.Fatalf("open=%v cur=%+v", a.search.open, b.Cur)
	}
}

func TestSearchPanelDebounce(t *testing.T) {
	a := newTestApp(t, "needle\n")
	a.openSearch(fQuery)
	keys := func(s string) {
		for _, r := range s {
			a.handleSearchKey(textKey(string(r)))
		}
	}
	keys("nee")
	stale := SearchTick{seq: a.search.seq.Load()}
	keys("dle")
	if a.handleEvent(stale) != nil {
		t.Fatal("a superseded tick must be dropped")
	}
	effs := a.handleEvent(SearchTick{seq: a.search.seq.Load()})
	if len(effs) != 1 {
		t.Fatalf("the latest tick must start a search, got %#v", effs)
	}
	run, ok := effs[0].(RunSearch)
	if !ok {
		t.Fatalf("got %#v", effs[0])
	}
	done, ok := run.Run().(SearchDone)
	if !ok || len(done.res.hits) != 1 {
		t.Fatalf("got %#v", done)
	}
	if a.handleEvent(done) != nil || a.search.busy || len(a.search.res.hits) != 1 {
		t.Fatal("the result was not taken")
	}
}

func (a *App) renderPanelForTest() []string {
	_, _, box := a.renderSearch()
	return box
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

func TestSearchReplaceOne(t *testing.T) {
	a := newTestApp(t, "foo one\nbar\nfoo two\nfoo three\n")
	drive(a, a.openSearchArgs("foo => X", true))
	if len(a.search.res.hits) != 3 {
		t.Fatalf("hits = %d", len(a.search.res.hits))
	}
	a.handleSearchKey(Key{Code: KeyDown})
	drive(a, a.handleSearchKey(altKey(string('1'))))
	if !a.search.open {
		t.Fatal("panel must stay open")
	}
	if got := a.curText().buf.Text(); !strings.HasPrefix(got, "foo one\nbar\nX two\nfoo three") {
		t.Fatalf("text = %q", got)
	}
	if len(a.search.res.hits) != 2 || a.search.cursor != 1 {
		t.Fatalf("hits=%d cursor=%d", len(a.search.res.hits), a.search.cursor)
	}
	if h := a.search.cur(); h == nil || h.row != 3 {
		t.Fatalf("cursor must be on the next hit: %+v", h)
	}
}

func TestSearchReplaceOneExcludedLine(t *testing.T) {
	a := newTestApp(t, "foo one\nfoo two\n")
	drive(a, a.openSearchArgs("foo => X", true))
	a.handleSearchKey(Key{Code: 'x', Mod: ModCtrl})
	if !a.search.res.hits[0].skip {
		t.Fatal("ctrl+x must exclude the line")
	}
	drive(a, a.handleSearchKey(altKey(string('1'))))
	if got := a.curText().buf.Text(); !strings.HasPrefix(got, "X one\nfoo two") {
		t.Fatalf("text = %q", got)
	}
	if len(a.search.skipped) != 0 {
		t.Fatalf("skipped = %v", a.search.skipped)
	}
}

func TestSearchReplaceOneStillMatching(t *testing.T) {
	a := newTestApp(t, "foo\nfoo\n")
	drive(a, a.openSearchArgs("foo => foobar", true))
	drive(a, a.handleSearchKey(altKey(string('1'))))
	if h := a.search.cur(); h == nil || h.row != 1 {
		t.Fatalf("cursor must move past the replaced line: %+v", h)
	}
	if got := a.curText().buf.Text(); !strings.HasPrefix(got, "foobar\nfoo") {
		t.Fatalf("text = %q", got)
	}
}

func TestSearchPanelLayout(t *testing.T) {
	a := newTestApp(t, "foo\n")
	a.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	drive(a, a.openSearch(fQuery))
	x, y, box := a.renderSearch()
	if x != 1 || y != 1 || len(box) != a.h-2 {
		t.Fatalf("window must leave a 1-cell margin: x=%d y=%d rows=%d (h=%d)", x, y, len(box), a.h)
	}
	for _, r := range box {
		if w := ansi.StringWidth(r); w != a.w-2 {
			t.Fatalf("row width = %d, want %d: %q", w, a.w-2, ansi.Strip(r))
		}
	}
	got := ansi.Strip(strings.Join(box, "\n"))
	for _, want := range []string{"┌─ 検索 ", "┌─ 置換 ", "┌─ 含める ", "┌─ 除外 ", "置換", "含めるファイル（例: *.go）"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestSearchFieldKeepsCursorCell(t *testing.T) {
	a := newTestApp(t, "foo\n")
	a.Update(tea.WindowSizeMsg{Width: 60, Height: 40})
	drive(a, a.openSearch(fQuery))
	// field box width is 26, so the text area is 22 cells wide.
	for _, n := range []int{20, 21, 22, 23, 30} {
		a.search.in[fQuery].SetValue(strings.Repeat("a", n))
		a.search.in[fQuery].CursorEnd()
		_, _, box := a.renderSearch()
		got := ansi.Strip(box[2])
		// text, then the cursor cell, then the padding column before the border
		i := strings.Index(got, "│ │ ") + len("│ │ ") // outer border, field border, padding
		field := []rune(got[i:])
		inner := string(field[:22])
		if !strings.HasSuffix(inner, " ") {
			t.Fatalf("n=%d: no cursor cell at the end: %q", n, got)
		}
	}
	a.search.in[fQuery].SetValue("")
	_, _, box := a.renderSearch()
	if strings.Contains(strings.Join(box, ""), "\x00") {
		t.Fatal("NUL in output")
	}
}
