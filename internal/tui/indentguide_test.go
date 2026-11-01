package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/mm1995tk/shijimi-editor/internal/buffer"
)

func guideRows(src string, on bool) []string {
	b := buffer.New()
	b.Lines = nil
	for l := range strings.SplitSeq(src, "\n") {
		b.Lines = append(b.Lines, []rune(l))
	}
	rows := renderEditor(newTab(b), 40, len(b.Lines), editorOpts{indentGuides: on})
	gw := gutterWidth(b)
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = strings.TrimRight(string([]rune(ansi.Strip(r))[gw:]), " ")
	}
	return out
}

func TestIndentGuides(t *testing.T) {
	src := "func x() {\n    if ok {\n\n        f()\n    }\n}"
	got := guideRows(src, true)
	want := []string{
		"func x() {",
		"│   if ok {",
		"│   │",
		"│   │   f()",
		"│   }",
		"}",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestIndentGuidesTabsAndOff(t *testing.T) {
	src := "a {\n\tb {\n\t\tc\n\t}\n}"
	got := guideRows(src, true)
	if got[2] != "│   │   c" {
		t.Errorf("tab row = %q", got[2])
	}
	if off := guideRows(src, false); strings.Contains(strings.Join(off, ""), "│") {
		t.Errorf("guides drawn while disabled: %q", off)
	}
}
