package diff

import (
	"reflect"
	"strings"
	"testing"
)

func TestInline(t *testing.T) {
	tests := []struct {
		name     string
		old, cur string
		wantOld  []Span
		wantCur  []Span
		wantOK   bool
	}{
		{"inserted", "func Load(root string) *Config {", "func Load(root string, poipoi int) *Config {",
			nil, []Span{{21, 33}}, true},
		{"replaced", `ListenHost: "127.0.0.1"`, `ListenHost: "0.0.0.0"`,
			[]Span{{13, 16}, {21, 22}}, []Span{{13, 14}, {19, 20}}, true},
		{"deleted", "a, b, c", "a, c",
			[]Span{{3, 6}}, nil, true},
		{"same", "abc", "abc", nil, nil, true},
		{"unrelated", "return nil", "x := 42 // ok", nil, nil, false},
		{"one side empty", "", "abc", nil, nil, false},
		{"multibyte", "名前を変える", "名前を直す",
			[]Span{{3, 6}}, []Span{{3, 5}}, true},
		// two changes 1 rune apart are drawn as one
		{"joined", "abcdef", "aXcYef", []Span{{1, 4}}, []Span{{1, 4}}, true},
		// changes far apart stay separate
		{"apart", "a long line here", "A long line herE", []Span{{0, 1}, {15, 16}}, []Span{{0, 1}, {15, 16}}, true},
	}
	for _, tt := range tests {
		o, c, ok := Inline(tt.old, tt.cur)
		if ok != tt.wantOK || !reflect.DeepEqual(o, tt.wantOld) || !reflect.DeepEqual(c, tt.wantCur) {
			t.Errorf("%s: Inline(%q, %q) = %v, %v, %v; want %v, %v, %v", tt.name, tt.old, tt.cur, o, c, ok, tt.wantOld, tt.wantCur, tt.wantOK)
		}
	}
}

func TestInlineLongLines(t *testing.T) {
	long := strings.Repeat("x", MaxInlineRunes+1)
	if _, _, ok := Inline(long, long+"y"); ok {
		t.Errorf("lines over MaxInlineRunes were compared")
	}
	ok := strings.Repeat("x", MaxInlineRunes)
	if _, c, found := Inline(ok[1:], ok); !found || len(c) != 1 {
		t.Errorf("a line of MaxInlineRunes runes was not compared: %v %v", c, found)
	}
}

func TestInlineHalfCommon(t *testing.T) {
	// 4 of 8 runes common: related
	if _, _, ok := Inline("abcd", "abcdWXYZ"); !ok {
		t.Errorf("half common was treated as unrelated")
	}
	// 3 of 8 runes common: unrelated
	if _, _, ok := Inline("abc", "abcVWXYZ"); ok {
		t.Errorf("less than half common was treated as related")
	}
}

func TestSharedRunes(t *testing.T) {
	if got := sharedRunes([]rune("aabc"), []rune("abca")); got != 4 {
		t.Errorf("same runes: %d", got)
	}
	if got := sharedRunes([]rune("aaa"), []rune("ab")); got != 1 {
		t.Errorf("repeats: %d", got)
	}
}
