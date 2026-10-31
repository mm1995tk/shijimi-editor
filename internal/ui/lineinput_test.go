package ui

import "testing"

func TestLineInputEditing(t *testing.T) {
	type step = Input
	for _, c := range []struct {
		name  string
		start string
		keys  []step
		want  string
		pos   int
	}{
		{"type", "", []step{textKey("a"), textKey("日"), textKey("b")}, "a日b", 3},
		{"backspace", "abc", []step{Key{Code: KeyBackspace}}, "ab", 2},
		{"left delete", "abc", []step{Key{Code: KeyLeft}, Key{Code: KeyDelete}}, "ab", 2},
		{"home type", "abc", []step{Key{Code: KeyHome}, textKey("x")}, "xabc", 1},
		{"ctrl+w", "foo bar", []step{Key{Code: 'w', Mod: ModCtrl}}, "foo ", 4},
		{"ctrl+u", "foo bar", []step{Key{Code: KeyLeft, Mod: ModCtrl}, Key{Code: 'u', Mod: ModCtrl}}, "bar", 0},
		{"ctrl+k", "foo bar", []step{Key{Code: KeyHome}, Key{Code: KeyRight, Mod: ModCtrl}, Key{Code: 'k', Mod: ModCtrl}}, "foo", 3},
		{"alt+d", "foo bar", []step{Key{Code: KeyHome}, Key{Code: 'd', Mod: ModAlt}}, " bar", 0},
		{"paste", "ab", []step{Key{Code: KeyLeft}, Paste{Content: "x\ny\tz"}}, "ax y zb", 6},
	} {
		l := newLineInput(0)
		l.Focus()
		l.SetValue(c.start)
		l.CursorEnd()
		for _, k := range c.keys {
			l.Update(k)
		}
		if l.Value() != c.want || l.Position() != c.pos {
			t.Errorf("%s: got %q at %d, want %q at %d", c.name, l.Value(), l.Position(), c.want, c.pos)
		}
	}
}

func TestLineInputLimitAndBlur(t *testing.T) {
	l := newLineInput(3)
	l.Focus()
	l.Insert("abcdef")
	if l.Value() != "abc" {
		t.Fatalf("limit: %q", l.Value())
	}
	l.Blur()
	l.Update(textKey("x"))
	if l.Value() != "abc" {
		t.Fatalf("blurred input changed the text: %q", l.Value())
	}
}
