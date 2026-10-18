package ui

import (
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mm1995tk/gh-editor/internal/buffer"
)

func TestParseSnippet(t *testing.T) {
	cases := []struct {
		in    string
		text  string
		stops []snipStop
	}{
		{"Printf(${1:format}, ${2:a...})", "Printf(format, a...)", []snipStop{{1, 7, 13}, {2, 15, 19}}},
		{"f($1)$0", "f()", []snipStop{{1, 2, 2}, {0, 3, 3}}},
		{"${1|a,b|} x", "a x", []snipStop{{1, 0, 1}}},
		{`\$1 \} ${1:x}`, "$1 } x", []snipStop{{1, 5, 6}}},
		{"${1:a ${2:b}}", "a b", []snipStop{{2, 2, 3}, {1, 0, 3}}},
		{"${TM_FILENAME:x}$HOME", "x", nil},
	}
	for _, c := range cases {
		text, stops := parseSnippet(c.in)
		if text != c.text || !reflect.DeepEqual(stops, c.stops) {
			t.Errorf("%q: got %q %v, want %q %v", c.in, text, stops, c.text, c.stops)
		}
	}
}

func snipSetup(t *testing.T, content string) (*App, *buffer.Buffer) {
	a := newTestApp(t, content)
	a.focus = focusEditor
	a.mode = modeInsert
	b := a.curText().buf
	b.SetCursor(0, 1000, false)
	return a, b
}

func pressKey(a *App, k tea.KeyType) { a.Update(tea.KeyMsg{Type: k}) }

func TestSnippetTabMovesBetweenStops(t *testing.T) {
	a, b := snipSetup(t, "fmt.")
	a.startSnippet(a.curText(), buffer.Pos{Col: 4}, "Printf(${1:format}, ${2:a...})$0")
	if got := b.Text(); got != "fmt.Printf(format, a...)" {
		t.Fatalf("text = %q", got)
	}
	keys(a, "x") // replaces "format"
	pressKey(a, tea.KeyTab)
	keys(a, "y", "z") // replaces "a..."
	if got := b.Text(); got != "fmt.Printf(x, yz)" {
		t.Fatalf("text = %q", got)
	}
	pressKey(a, tea.KeyShiftTab)
	keys(a, "w")
	if got := b.Text(); got != "fmt.Printf(w, yz)" {
		t.Fatalf("after shift+tab: %q", got)
	}
	pressKey(a, tea.KeyTab)
	pressKey(a, tea.KeyTab) // $0
	if a.snip != nil || b.Cur != (buffer.Pos{Col: 17}) {
		t.Fatalf("session = %v, cursor = %v", a.snip, b.Cur)
	}
}

func TestSnippetMirrorsEditTogether(t *testing.T) {
	a, b := snipSetup(t, "")
	a.startSnippet(a.curText(), buffer.Pos{}, "${1:n} := $1")
	keys(a, "a", "b")
	if got := b.Text(); got != "ab := ab" {
		t.Fatalf("text = %q", got)
	}
}

func TestSnippetEndsWhenCursorLeavesStop(t *testing.T) {
	a, b := snipSetup(t, "")
	a.startSnippet(a.curText(), buffer.Pos{}, "f(${1:a}, ${2:b})")
	b.SetCursor(0, 0, false)
	pressKey(a, tea.KeyTab)
	if a.snip != nil {
		t.Fatal("session should have ended")
	}
}

func TestSnippetEscEndsSession(t *testing.T) {
	a, _ := snipSetup(t, "")
	a.startSnippet(a.curText(), buffer.Pos{}, "f(${1:a}, ${2:b})")
	pressKey(a, tea.KeyEsc)
	if a.snip != nil {
		t.Fatal("session should have ended")
	}
}

func TestSnippetAdjacentStopsDoNotSwallowTyping(t *testing.T) {
	a, b := snipSetup(t, "")
	a.startSnippet(a.curText(), buffer.Pos{}, "${1:x}$0")
	keys(a, "a", "b")
	pressKey(a, tea.KeyTab)
	if a.snip != nil || b.Cur != (buffer.Pos{Col: 2}) || b.Anchor != nil {
		t.Fatalf("cursor = %v anchor = %v session = %v", b.Cur, b.Anchor, a.snip)
	}

	a, b = snipSetup(t, "")
	a.startSnippet(a.curText(), buffer.Pos{}, "${1:x}${2:y}")
	keys(a, "a", "b")
	pressKey(a, tea.KeyTab)
	keys(a, "Z")
	if got := b.Text(); got != "abZ" {
		t.Fatalf("text = %q", got)
	}
}

func TestSnippetUndoEndsSession(t *testing.T) {
	a, b := snipSetup(t, "")
	a.startSnippet(a.curText(), buffer.Pos{}, "f(${1:a}, ${2:b})")
	keys(a, "x")
	pressKey(a, tea.KeyCtrlZ)
	if got := b.Text(); got != "f(a, b)" {
		t.Fatalf("text = %q", got)
	}
	pressKey(a, tea.KeyTab) // plain indentation now, not a jump to a stale stop
	if a.snip != nil {
		t.Fatal("session should have ended")
	}
}

func TestSnippetReverseOrderAdjacentStops(t *testing.T) {
	a, b := snipSetup(t, "")
	a.startSnippet(a.curText(), buffer.Pos{}, "${2:b}${1:a}")
	keys(a, "x")
	pressKey(a, tea.KeyTab)
	keys(a, "Z")
	if got := b.Text(); got != "Zx" {
		t.Fatalf("text = %q", got)
	}

	a, b = snipSetup(t, "")
	a.startSnippet(a.curText(), buffer.Pos{}, "$1$2")
	keys(a, "p")
	pressKey(a, tea.KeyTab)
	keys(a, "q")
	if got := b.Text(); got != "pq" {
		t.Fatalf("empty stops: text = %q", got)
	}
}

func TestSnippetStopBeforeMirror(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"${1:a} ${2:b}${1:c}", "x Zx"},
		{"${1:a}${2:b}${1:c}", "xZx"},
	} {
		a, b := snipSetup(t, "")
		a.startSnippet(a.curText(), buffer.Pos{}, c.in)
		keys(a, "x")
		pressKey(a, tea.KeyTab)
		keys(a, "Z")
		if got := b.Text(); got != c.want {
			t.Errorf("%q: text = %q, want %q", c.in, got, c.want)
		}
	}
}
