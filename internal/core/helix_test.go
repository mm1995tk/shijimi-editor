package core

import (
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

func keys(a *App, ks ...string) {
	for _, k := range ks {
		var m Key
		switch k {
		case "esc":
			m = Key{Code: KeyEscape}
		case "enter":
			m = Key{Code: KeyEnter}
		case "space":
			m = Key{Code: KeySpace, Text: " "}
		default:
			m = textKey(k)
		}
		a.Update(m)
	}
}

// textKey is a key press that types s (several runes arrive as one message).
func textKey(s string) Key {
	r, _ := utf8.DecodeRuneInString(s)
	if utf8.RuneCountInString(s) > 1 {
		r = KeyExtended
	}
	return Key{Code: r, Text: s}
}

// altKey is Alt held with the key that types s.
func altKey(s string) Key {
	r, _ := utf8.DecodeRuneInString(s)
	return Key{Code: r, Mod: ModAlt}
}

func newTestApp(t *testing.T, content string) *App {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	os.WriteFile(p, []byte(content), 0o644)
	a := newApp(dir, p)
	a.helix = true
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return a
}

func TestHelixLineOps(t *testing.T) {
	a := newTestApp(t, "one\ntwo\nthree\n")
	b := a.CurText().Buf
	keys(a, "2", "g")
	if b.Cur.Row != 1 {
		t.Fatalf("2g -> row %d", b.Cur.Row)
	}
	keys(a, "x", "d")
	if got := b.Text(); got != "one\nthree\n" {
		t.Fatalf("xd -> %q", got)
	}
	keys(a, "g", "g", "P")
	if got := b.Text(); got != "two\none\nthree\n" {
		t.Fatalf("ggP -> %q", got)
	}
	keys(a, "u", "u")
	if got := b.Text(); got != "one\ntwo\nthree\n" {
		t.Fatalf("uu -> %q", got)
	}
}

func TestHelixWordChangeAndInsert(t *testing.T) {
	a := newTestApp(t, "foo bar\n")
	b := a.CurText().Buf
	keys(a, "w")
	if got := b.SelectedText(); got != "foo " {
		t.Fatalf("w selects %q", got)
	}
	keys(a, ";", "g", "h", "e", "c")
	keys(a, "b", "a", "z", "esc")
	if got := b.Text(); got != "baz bar\n" {
		t.Fatalf("ec -> %q", got)
	}
	keys(a, "A", "!", "esc", "o", "x", "esc")
	if got := b.Text(); got != "baz bar!\nx\n" {
		t.Fatalf("A/o -> %q", got)
	}
}

func TestCmdline(t *testing.T) {
	a := newTestApp(t, "a\nb\nc\n")
	b := a.CurText().Buf
	keys(a, ":", "3", "enter")
	if b.Cur.Row != 2 {
		t.Fatalf(":3 -> row %d", b.Cur.Row)
	}
	keys(a, "/", "b", "enter")
	if b.Cur.Row != 1 {
		t.Fatalf("/b -> %+v", b.Cur)
	}
	keys(a, "i", "X", "esc", ":", "w", "enter")
	data, _ := os.ReadFile(b.Path)
	if string(data) != "a\nXb\nc\n" {
		t.Fatalf("saved %q", data)
	}
}

func TestGroupedRunes(t *testing.T) {
	a := newTestApp(t, "a\nb\nc\n")
	b := a.CurText().Buf
	a.Update(textKey("3gx"))
	if b.SelectedText() != "c\n" {
		t.Fatalf("grouped runes -> %+v %q", b.Cur, b.SelectedText())
	}
}

func TestHelixMultiCursor(t *testing.T) {
	a := newTestApp(t, "one\ntwo\nthree\n")
	b := a.CurText().Buf
	keys(a, "C", "C")
	if len(b.Extra()) != 2 {
		t.Fatalf("cursors = %d", len(b.Extra())+1)
	}
	keys(a, "i", "X", "Y", "esc")
	if got := b.Text(); got != "XYone\nXYtwo\nXYthree\n" {
		t.Fatalf("insert -> %q", got)
	}
	keys(a, "h", "h", "d")
	if got := b.Text(); got != "Yone\nYtwo\nYthree\n" {
		t.Fatalf("hhd -> %q", got)
	}
	keys(a, "c", "-", "esc")
	if got := b.Text(); got != "-one\n-two\n-three\n" {
		t.Fatalf("c -> %q", got)
	}
	keys(a, ",")
	if b.HasExtra() {
		t.Fatal(", should collapse to one cursor")
	}
	a.Update(altKey("C"))
	if !b.HasExtra() || b.Cur.Row != 1 {
		t.Fatalf("alt+C -> extra=%v row=%d", b.HasExtra(), b.Cur.Row)
	}
}

func TestHelixSelectRegex(t *testing.T) {
	a := newTestApp(t, "a1 b2\nc3\n")
	b := a.CurText().Buf
	keys(a, "%", "s")
	for _, r := range `\d` {
		keys(a, string(r))
	}
	keys(a, "enter", "c", "#", "esc")
	if got := b.Text(); got != "a# b#\nc#\n" {
		t.Fatalf("got %q", got)
	}
}

func TestAutoCloseBracketHelixInsert(t *testing.T) {
	a := newTestApp(t, "")
	b := a.CurText().Buf
	keys(a, "i", "{")
	if got := b.Text(); got != "{}" || b.Cur.Col != 1 {
		t.Fatalf("text = %q cur = %v", got, b.Cur)
	}
	a.Update(Key{Code: KeyBackspace})
	if got := b.Text(); got != "" {
		t.Fatalf("after backspace = %q", got)
	}
}

func TestAutoCloseBracketVSCode(t *testing.T) {
	a := newTestApp(t, "")
	a.helix = false
	b := a.CurText().Buf
	keys(a, "(", ")")
	if got := b.Text(); got != "()" || b.Cur.Col != 2 {
		t.Fatalf("text = %q cur = %v", got, b.Cur)
	}
}

func TestHelixMatchTextObjects(t *testing.T) {
	a := newTestApp(t, "foo (bar baz) \"q r\"\n")
	b := a.CurText().Buf
	b.SetCursor(0, 6, false)
	keys(a, "m", "i", "(")
	if got := b.SelectedText(); got != "bar baz" {
		t.Fatalf("mi( = %q", got)
	}
	keys(a, "esc")
	b.SetCursor(0, 6, false)
	keys(a, "m", "a", ")")
	if got := b.SelectedText(); got != "(bar baz)" {
		t.Fatalf("ma) = %q", got)
	}
	keys(a, "esc")
	b.SetCursor(0, 16, false)
	keys(a, "m", "i", "\"")
	if got := b.SelectedText(); got != "q r" {
		t.Fatalf("mi\" = %q", got)
	}
	keys(a, "esc")
	b.SetCursor(0, 1, false)
	keys(a, "m", "a", "w")
	if got := b.SelectedText(); got != "foo " {
		t.Fatalf("maw = %q", got)
	}
	// select mode works too
	keys(a, "esc")
	b.SetCursor(0, 6, false)
	keys(a, "v", "m", "i", "w")
	if got := b.SelectedText(); got != "bar" {
		t.Fatalf("v miw = %q", got)
	}
	// not found keeps the selection and reports it
	keys(a, "m", "i", "[")
	if got := b.SelectedText(); got != "bar" || a.msg == "" {
		t.Fatalf("mi[ = %q msg %q", got, a.msg)
	}
}

func TestHelixMatchTextObjectMultiCursor(t *testing.T) {
	a := newTestApp(t, "ab cd\nef gh\n")
	b := a.CurText().Buf
	b.SetCursor(0, 0, false)
	b.CopySelection(1)
	keys(a, "m", "i", "w")
	if !b.HasExtra() {
		t.Fatal("lost the extra cursor")
	}
	var got []string
	b.ForEachCursor(func() { got = append(got, b.SelectedText()) })
	if len(got) != 2 || got[0] != "ab" && got[0] != "ef" || got[1] != "ab" && got[1] != "ef" || got[0] == got[1] {
		t.Fatalf("selections = %q", got)
	}
}
