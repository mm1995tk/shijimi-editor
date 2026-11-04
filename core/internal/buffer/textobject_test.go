package buffer

import "testing"

func TestSelectTextObject(t *testing.T) {
	cases := []struct {
		name   string
		text   string
		at     Pos
		kind   rune
		around bool
		want   string // "" with ok=false means no object
		ok     bool
	}{
		{"iw", "foo bar baz", Pos{0, 5}, 'w', false, "bar", true},
		{"aw trailing space", "foo bar baz", Pos{0, 5}, 'w', true, "bar ", true},
		{"aw leading space at end", "foo bar", Pos{0, 5}, 'w', true, " bar", true},
		{"aw no space either side", "(foo)", Pos{0, 2}, 'w', true, "foo", true},
		{"aw after punctuation", "foo.bar", Pos{0, 5}, 'w', true, "bar", true},
		{"iw punctuation", "foo.bar", Pos{0, 3}, 'w', false, ".", true},
		{"iW", "foo.bar baz", Pos{0, 1}, 'W', false, "foo.bar", true},
		{"aW", "foo.bar baz", Pos{0, 1}, 'W', true, "foo.bar ", true},
		{"iw whitespace", "a   b", Pos{0, 2}, 'w', false, "   ", true},
		{"iw empty line", "", Pos{0, 0}, 'w', false, "", false},
		{"ip", "a\nb\n\nc\n", Pos{0, 0}, 'p', false, "a\nb\n", true},
		{"ap", "a\nb\n\n\nc\n", Pos{1, 0}, 'p', true, "a\nb\n\n\n", true},
		{"ap last paragraph", "a\n\nc\nd", Pos{2, 0}, 'p', true, "\nc\nd", true},
		{"i(", "f(a, (b), c)", Pos{0, 3}, '(', false, "a, (b), c", true},
		{"a(", "f(a, (b), c)", Pos{0, 3}, '(', true, "(a, (b), c)", true},
		{"i) nested", "f(a, (b), c)", Pos{0, 7}, ')', false, "b", true},
		{"i( on open", "f(a)", Pos{0, 1}, '(', false, "a", true},
		{"i( on close", "f(a)", Pos{0, 3}, '(', false, "a", true},
		{"i[", "x[1, 2]", Pos{0, 4}, '[', false, "1, 2", true},
		{"i{ multiline", "{\n  a\n}", Pos{1, 2}, '}', false, "\n  a\n", true},
		{"a<", "a<b>c", Pos{0, 2}, '>', true, "<b>", true},
		{"i( none", "abc", Pos{0, 1}, '(', false, "", false},
		{"i( outside", "(a) b", Pos{0, 4}, '(', false, "", false},
		{"i( empty", "()", Pos{0, 0}, '(', false, "", false},
		{"i\"", `x "hi there" y`, Pos{0, 5}, '"', false, "hi there", true},
		{"a\"", `x "hi there" y`, Pos{0, 5}, '"', true, `"hi there"`, true},
		{"i' second pair", "'a' 'b'", Pos{0, 5}, '\'', false, "b", true},
		{"i` escaped", "`a\\`b`", Pos{0, 1}, '`', false, "a\\`b", true},
		{"i\" outside", `"a" b`, Pos{0, 4}, '"', false, "", false},
	}
	for _, c := range cases {
		b := New()
		b.Insert(c.text)
		b.Cur = c.at
		b.Anchor = nil
		ok := b.SelectTextObject(c.kind, c.around)
		if ok != c.ok {
			t.Errorf("%s: ok = %v", c.name, ok)
			continue
		}
		if ok && b.SelectedText() != c.want {
			t.Errorf("%s: selected %q, want %q", c.name, b.SelectedText(), c.want)
		}
	}
}
