package core

import (
	"reflect"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
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
	a.Focus = FocusEditor
	a.Mode = ModeInsert
	b := a.CurText().Buf
	b.SetCursor(0, 1000, false)
	return a, b
}

func pressKey(a *App, k rune, mod ...Mod) {
	m := Key{Code: k}
	for _, x := range mod {
		m.Mod |= x
	}
	a.Update(m)
}

func TestSnippetTabMovesBetweenStops(t *testing.T) {
	a, b := snipSetup(t, "fmt.")
	a.startSnippet(a.CurText(), buffer.Pos{Col: 4}, "Printf(${1:format}, ${2:a...})$0")
	if got := b.Text(); got != "fmt.Printf(format, a...)" {
		t.Fatalf("text = %q", got)
	}
	keys(a, "x") // replaces "format"
	pressKey(a, KeyTab)
	keys(a, "y", "z") // replaces "a..."
	if got := b.Text(); got != "fmt.Printf(x, yz)" {
		t.Fatalf("text = %q", got)
	}
	pressKey(a, KeyTab, ModShift)
	keys(a, "w")
	if got := b.Text(); got != "fmt.Printf(w, yz)" {
		t.Fatalf("after shift+tab: %q", got)
	}
	pressKey(a, KeyTab)
	pressKey(a, KeyTab) // $0
	if a.snip != nil || b.Cur != (buffer.Pos{Col: 17}) {
		t.Fatalf("session = %v, cursor = %v", a.snip, b.Cur)
	}
}

func TestSnippetMirrorsEditTogether(t *testing.T) {
	a, b := snipSetup(t, "")
	a.startSnippet(a.CurText(), buffer.Pos{}, "${1:n} := $1")
	keys(a, "a", "b")
	if got := b.Text(); got != "ab := ab" {
		t.Fatalf("text = %q", got)
	}
}

func TestSnippetEndsWhenCursorLeavesStop(t *testing.T) {
	a, b := snipSetup(t, "")
	a.startSnippet(a.CurText(), buffer.Pos{}, "f(${1:a}, ${2:b})")
	b.SetCursor(0, 0, false)
	pressKey(a, KeyTab)
	if a.snip != nil {
		t.Fatal("session should have ended")
	}
}

func TestSnippetEscEndsSession(t *testing.T) {
	a, _ := snipSetup(t, "")
	a.startSnippet(a.CurText(), buffer.Pos{}, "f(${1:a}, ${2:b})")
	pressKey(a, KeyEsc)
	if a.snip != nil {
		t.Fatal("session should have ended")
	}
}

func TestSnippetAdjacentStopsDoNotSwallowTyping(t *testing.T) {
	a, b := snipSetup(t, "")
	a.startSnippet(a.CurText(), buffer.Pos{}, "${1:x}$0")
	keys(a, "a", "b")
	pressKey(a, KeyTab)
	if a.snip != nil || b.Cur != (buffer.Pos{Col: 2}) || b.Anchor != nil {
		t.Fatalf("cursor = %v anchor = %v session = %v", b.Cur, b.Anchor, a.snip)
	}

	a, b = snipSetup(t, "")
	a.startSnippet(a.CurText(), buffer.Pos{}, "${1:x}${2:y}")
	keys(a, "a", "b")
	pressKey(a, KeyTab)
	keys(a, "Z")
	if got := b.Text(); got != "abZ" {
		t.Fatalf("text = %q", got)
	}
}

func TestSnippetUndoEndsSession(t *testing.T) {
	a, b := snipSetup(t, "")
	a.startSnippet(a.CurText(), buffer.Pos{}, "f(${1:a}, ${2:b})")
	keys(a, "x")
	pressKey(a, 'z', ModCtrl)
	if got := b.Text(); got != "f(a, b)" {
		t.Fatalf("text = %q", got)
	}
	pressKey(a, KeyTab) // plain indentation now, not a jump to a stale stop
	if a.snip != nil {
		t.Fatal("session should have ended")
	}
}

func TestSnippetReverseOrderAdjacentStops(t *testing.T) {
	a, b := snipSetup(t, "")
	a.startSnippet(a.CurText(), buffer.Pos{}, "${2:b}${1:a}")
	keys(a, "x")
	pressKey(a, KeyTab)
	keys(a, "Z")
	if got := b.Text(); got != "Zx" {
		t.Fatalf("text = %q", got)
	}

	a, b = snipSetup(t, "")
	a.startSnippet(a.CurText(), buffer.Pos{}, "$1$2")
	keys(a, "p")
	pressKey(a, KeyTab)
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
		a.startSnippet(a.CurText(), buffer.Pos{}, c.in)
		keys(a, "x")
		pressKey(a, KeyTab)
		keys(a, "Z")
		if got := b.Text(); got != c.want {
			t.Errorf("%q: text = %q, want %q", c.in, got, c.want)
		}
	}
}
