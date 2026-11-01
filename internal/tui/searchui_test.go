package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

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

	drive(a, cmdOf(a, a.openSearchArgs("foo => X", true)))
	if !a.search.open {
		t.Fatal("panel must open")
	}
	if got := len(a.search.res.Hits); got != 3 {
		t.Fatalf("hits = %d", got)
	}
	if got := ansi.Strip(strings.Join(a.renderPanelForTest(), "\n")); !strings.Contains(got, "3 件") || !strings.Contains(got, "置換対象 3 件") {
		t.Fatalf("panel = \n%s", got)
	}

	// leave the first line out, then replace the rest
	a.handleSearchKey(Key{Code: 'x', Mod: ModCtrl})
	if !a.search.res.Hits[0].Skip {
		t.Fatal("ctrl+x must exclude the line")
	}
	drive(a, cmdOf(a, a.handleSearchKey(Key{Code: KeyEnter, Mod: ModAlt})))
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
	drive(a, cmdOf(a, a.openSearchArgs("needle", false)))
	if len(a.search.res.Hits) != 1 {
		t.Fatalf("hits = %+v", a.search.res.Hits)
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
	if !ok || len(done.res.Hits) != 1 {
		t.Fatalf("got %#v", done)
	}
	if a.handleEvent(done) != nil || a.search.busy || len(a.search.res.Hits) != 1 {
		t.Fatal("the result was not taken")
	}
}

func (a *App) renderPanelForTest() []string {
	a.fit()
	_, _, box := a.renderSearch()
	return box
}

func TestSearchReplaceOne(t *testing.T) {
	a := newTestApp(t, "foo one\nbar\nfoo two\nfoo three\n")
	drive(a, cmdOf(a, a.openSearchArgs("foo => X", true)))
	if len(a.search.res.Hits) != 3 {
		t.Fatalf("hits = %d", len(a.search.res.Hits))
	}
	a.handleSearchKey(Key{Code: KeyDown})
	drive(a, cmdOf(a, a.handleSearchKey(altKey(string('1')))))
	if !a.search.open {
		t.Fatal("panel must stay open")
	}
	if got := a.curText().buf.Text(); !strings.HasPrefix(got, "foo one\nbar\nX two\nfoo three") {
		t.Fatalf("text = %q", got)
	}
	if len(a.search.res.Hits) != 2 || a.search.cursor != 1 {
		t.Fatalf("hits=%d cursor=%d", len(a.search.res.Hits), a.search.cursor)
	}
	if h := a.search.cur(); h == nil || h.Row != 3 {
		t.Fatalf("cursor must be on the next hit: %+v", h)
	}
}

func TestSearchReplaceOneExcludedLine(t *testing.T) {
	a := newTestApp(t, "foo one\nfoo two\n")
	drive(a, cmdOf(a, a.openSearchArgs("foo => X", true)))
	a.handleSearchKey(Key{Code: 'x', Mod: ModCtrl})
	if !a.search.res.Hits[0].Skip {
		t.Fatal("ctrl+x must exclude the line")
	}
	drive(a, cmdOf(a, a.handleSearchKey(altKey(string('1')))))
	if got := a.curText().buf.Text(); !strings.HasPrefix(got, "X one\nfoo two") {
		t.Fatalf("text = %q", got)
	}
	if len(a.search.skipped) != 0 {
		t.Fatalf("skipped = %v", a.search.skipped)
	}
}

func TestSearchReplaceOneStillMatching(t *testing.T) {
	a := newTestApp(t, "foo\nfoo\n")
	drive(a, cmdOf(a, a.openSearchArgs("foo => foobar", true)))
	drive(a, cmdOf(a, a.handleSearchKey(altKey(string('1')))))
	if h := a.search.cur(); h == nil || h.Row != 1 {
		t.Fatalf("cursor must move past the replaced line: %+v", h)
	}
	if got := a.curText().buf.Text(); !strings.HasPrefix(got, "foobar\nfoo") {
		t.Fatalf("text = %q", got)
	}
}

func TestSearchPanelLayout(t *testing.T) {
	a := newTestApp(t, "foo\n")
	a.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	drive(a, cmdOf(a, a.openSearch(fQuery)))
	a.fit()
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
	drive(a, cmdOf(a, a.openSearch(fQuery)))
	// field box width is 26, so the text area is 22 cells wide.
	for _, n := range []int{20, 21, 22, 23, 30} {
		a.search.in[fQuery].SetValue(strings.Repeat("a", n))
		a.search.in[fQuery].CursorEnd()
		a.fit()
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
	a.fit()
	_, _, box := a.renderSearch()
	if strings.Contains(strings.Join(box, ""), "\x00") {
		t.Fatal("NUL in output")
	}
}
