package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/core"
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

func (a *App) renderPanelForTest() []string {
	a.fit()
	_, _, box := a.renderSearch()
	return box
}

func TestSearchPanelLayout(t *testing.T) {
	a := newTestApp(t, "foo\n")
	a.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	drive(a, cmdOf(a, a.OpenSearch(core.FQuery)))
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
	drive(a, cmdOf(a, a.OpenSearch(core.FQuery)))
	// field box width is 26, so the text area is 22 cells wide.
	for _, n := range []int{20, 21, 22, 23, 30} {
		a.Search.In[core.FQuery].SetValue(strings.Repeat("a", n))
		a.Search.In[core.FQuery].CursorEnd()
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
	a.Search.In[core.FQuery].SetValue("")
	a.fit()
	_, _, box := a.renderSearch()
	if strings.Contains(strings.Join(box, ""), "\x00") {
		t.Fatal("NUL in output")
	}
}

// renderSearch draws the window as the frame does and returns where it sits.
func (a *App) renderSearch() (x, y int, box []string) {
	g := a.SearchGeom()
	return g.X, g.Y, drawSearch(a.SearchWindow(), g, a.Rel)
}

func TestSearchPanelFlow(t *testing.T) {
	a := newTestApp(t, "foo one\nbar\nfoo two\n")
	other := filepath.Join(a.Root, "b.txt")
	os.WriteFile(other, []byte("foo three\n"), 0o644)

	drive(a, cmdOf(a, a.OpenSearchArgs("foo => X", true)))
	if !a.Search.Open {
		t.Fatal("panel must open")
	}
	if got := len(a.Search.Res.Hits); got != 3 {
		t.Fatalf("hits = %d", got)
	}
	if got := ansi.Strip(strings.Join(a.renderPanelForTest(), "\n")); !strings.Contains(got, "3 件") || !strings.Contains(got, "置換対象 3 件") {
		t.Fatalf("panel = \n%s", got)
	}

	// leave the first line out, then replace the rest
	a.HandleSearchKey(core.Key{Code: 'x', Mod: core.ModCtrl})
	if !a.Search.Res.Hits[0].Skip {
		t.Fatal("ctrl+x must exclude the line")
	}
	drive(a, cmdOf(a, a.HandleSearchKey(core.Key{Code: KeyEnter, Mod: core.ModAlt})))
	if a.Search.Open {
		t.Fatal("panel must close after replacing")
	}
	var text string
	for _, tb := range a.Tabs {
		if tb.Kind == core.TabText && tb.Buf.Path == filepath.Join(a.Root, "a.txt") {
			text = tb.Buf.Text()
			tb.Buf.Undo()
			if tb.Buf.Text() != "foo one\nbar\nfoo two\n" && tb.Buf.Text() != "foo one\nbar\nfoo two" {
				t.Fatalf("undo = %q", tb.Buf.Text())
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
	drive(a, cmdOf(a, a.OpenSearchArgs("needle", false)))
	if len(a.Search.Res.Hits) != 1 {
		t.Fatalf("hits = %+v", a.Search.Res.Hits)
	}
	a.HandleSearchKey(core.Key{Code: KeyEnter})
	b := a.CurText().Buf
	if a.Search.Open || b.Cur.Row != 2 || b.Cur.Col != 6 {
		t.Fatalf("open=%v cur=%+v", a.Search.Open, b.Cur)
	}
}

func TestSearchReplaceOne(t *testing.T) {
	a := newTestApp(t, "foo one\nbar\nfoo two\nfoo three\n")
	drive(a, cmdOf(a, a.OpenSearchArgs("foo => X", true)))
	if len(a.Search.Res.Hits) != 3 {
		t.Fatalf("hits = %d", len(a.Search.Res.Hits))
	}
	a.HandleSearchKey(core.Key{Code: KeyDown})
	drive(a, cmdOf(a, a.HandleSearchKey(altKey(string('1')))))
	if !a.Search.Open {
		t.Fatal("panel must stay open")
	}
	if got := a.CurText().Buf.Text(); !strings.HasPrefix(got, "foo one\nbar\nX two\nfoo three") {
		t.Fatalf("text = %q", got)
	}
	if len(a.Search.Res.Hits) != 2 || a.Search.Cursor != 1 {
		t.Fatalf("hits=%d cursor=%d", len(a.Search.Res.Hits), a.Search.Cursor)
	}
	if h := a.Search.Cur(); h == nil || h.Row != 3 {
		t.Fatalf("cursor must be on the next hit: %+v", h)
	}
}

func TestSearchReplaceOneExcludedLine(t *testing.T) {
	a := newTestApp(t, "foo one\nfoo two\n")
	drive(a, cmdOf(a, a.OpenSearchArgs("foo => X", true)))
	a.HandleSearchKey(core.Key{Code: 'x', Mod: core.ModCtrl})
	if !a.Search.Res.Hits[0].Skip {
		t.Fatal("ctrl+x must exclude the line")
	}
	drive(a, cmdOf(a, a.HandleSearchKey(altKey(string('1')))))
	if got := a.CurText().Buf.Text(); !strings.HasPrefix(got, "X one\nfoo two") {
		t.Fatalf("text = %q", got)
	}
	if len(a.Search.Skipped) != 0 {
		t.Fatalf("skipped = %v", a.Search.Skipped)
	}
}

func TestSearchReplaceOneStillMatching(t *testing.T) {
	a := newTestApp(t, "foo\nfoo\n")
	drive(a, cmdOf(a, a.OpenSearchArgs("foo => foobar", true)))
	drive(a, cmdOf(a, a.HandleSearchKey(altKey(string('1')))))
	if h := a.Search.Cur(); h == nil || h.Row != 1 {
		t.Fatalf("cursor must move past the replaced line: %+v", h)
	}
	if got := a.CurText().Buf.Text(); !strings.HasPrefix(got, "foobar\nfoo") {
		t.Fatalf("text = %q", got)
	}
}
