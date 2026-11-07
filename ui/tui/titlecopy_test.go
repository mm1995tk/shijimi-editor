package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/core"
	"github.com/mm1995tk/shijimi-editor/ui/internal/i18n"
)

func pressAt(a *App, x, y int) {
	a.HandleMouse(a.locate(x, y, core.Mouse{Action: core.MousePress, Button: core.MouseLeft}))
}

// The title row has the copy button at its right end; pressing it copies the
// relative path as :copy-rel does.
func TestTitleCopyButton(t *testing.T) {
	label := i18n.T("title.copy")
	a := newTestApp(t, "one\n")
	l := a.layout()
	row := ansi.Strip(a.renderTitle(a.Cur(), l.rightW, true))
	if !strings.HasSuffix(row, label) || ansi.StringWidth(row) != l.rightW {
		t.Fatalf("title %q", row)
	}
	a.Clipboard = ""
	pressAt(a, l.rightX+2, l.top) // on the path: not the button
	if a.Clipboard != "" {
		t.Fatalf("copied %q", a.Clipboard)
	}
	pressAt(a, l.rightX+l.rightW-2, l.top)
	if a.Clipboard != "a.txt" {
		t.Fatalf("clipboard %q", a.Clipboard)
	}
	if got, want := ansi.Strip(a.renderStatus()), fmt.Sprintf(i18n.T("app.pathcopied"), "a.txt"); !strings.Contains(got, want) {
		t.Fatalf("status %q lacks %q", got, want)
	}
}

// A buffer with no path has no button, and the path is cut before the button.
func TestTitleCopyButtonAbsentAndNarrow(t *testing.T) {
	label := i18n.T("title.copy")
	narrow := ansi.StringWidth(label) + titleCopyMin
	a := newTestApp(t, "one\n")
	a.Cur().Buf.Path = ""
	if row := ansi.Strip(a.renderTitle(a.Cur(), 40, true)); strings.Contains(row, label) {
		t.Fatalf("title %q", row)
	}
	l := a.layout()
	pressAt(a, l.rightX+l.rightW-2, l.top)
	if a.Clipboard != "" {
		t.Fatalf("copied %q", a.Clipboard)
	}

	b := newTestApp(t, "one\n")
	row := ansi.Strip(b.renderTitle(b.Cur(), narrow, true))
	if ansi.StringWidth(row) != narrow || !strings.HasSuffix(row, label) {
		t.Fatalf("narrow title %q", row)
	}
	if row := ansi.Strip(b.renderTitle(b.Cur(), narrow-1, true)); strings.Contains(row, label) {
		t.Fatalf("too narrow for the button: %q", row)
	}
}

// In the diff mode the header has the button too, and a press outside it still
// switches the split.
func TestDiffHeaderCopyButton(t *testing.T) {
	label := i18n.T("title.copy")
	a, _ := gitDiffApp(t)
	l := a.layout()
	head := ansi.Strip(a.renderDiffText(a.CurText(), a.DiffFor(a.CurText()), l, false)[0])
	if !strings.Contains(head, label) {
		t.Fatalf("header %q", head)
	}
	split := a.Gd.Split
	pressAt(a, l.rightX+2, l.editY)
	if a.Gd.Split == split {
		t.Fatal("a press outside the button must switch the split")
	}
	split = a.Gd.Split
	a.Clipboard = ""
	hw := diffHintWidth(diffHint(a.DiffFor(a.CurText())))
	pressAt(a, l.rightX+l.rightW-hw-2, l.editY) // the button, left of the hint
	if a.Clipboard == "" || a.Gd.Split != split {
		t.Fatalf("clipboard %q split %v", a.Clipboard, a.Gd.Split)
	}
	a.Clipboard = ""
	pressAt(a, l.rightX+l.rightW-2, l.editY) // the hint
	if a.Gd.Split == split || a.Clipboard != "" {
		t.Fatal("a press on the hint must switch the split only")
	}
	a.Clipboard = ""
	pressAt(a, l.rightX+l.rightW-2, l.top) // the pane's title row
	if a.Clipboard == "" {
		t.Fatal("title row button")
	}
}

// In a narrow pane the hint of the diff header takes its place before the
// button, so the row never grows past the pane.
func TestDiffHeaderNarrowKeepsWidth(t *testing.T) {
	a, _ := gitDiffApp(t)
	dv := a.DiffFor(a.CurText())
	hint := diffHint(dv)
	hw := diffHintWidth(hint)
	title := a.BuildDiffPanes(a.CurText(), dv, a.dvGeom(a.layout()), core.PaneOpts{})[1].Title
	for w := hw; w < hw+titleCopyMin+ansi.StringWidth(i18n.T("title.copy")); w++ {
		if got := ansi.StringWidth(drawDiffHeader(title, hint, w)); got != w {
			t.Fatalf("w=%d: header is %d wide", w, got)
		}
	}
}
