package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

const goplsHover = "```go\nfunc Println(a ...any) (n int, err error)\n```\n\n---\n\nPrintln formats using the default formats for its operands and writes to standard output. See `fmt` for [details](https://pkg.go.dev/fmt).\n"

func TestHoverSeparatesSignatureAndDoc(t *testing.T) {
	lines := hoverLines(goplsHover, 60)
	var code, text int
	for _, l := range lines {
		if l.code {
			code++
		} else {
			text++
		}
	}
	if code != 1 || text < 3 {
		t.Fatalf("code=%d text=%d", code, text)
	}
	if strings.Contains(plain(lines), "```") || strings.Contains(plain(lines), "](") {
		t.Fatalf("markdown syntax leaked:\n%s", plain(lines))
	}
}

func TestHoverBoxShapeAndScroll(t *testing.T) {
	v := newHoverView(goplsHover, 34, 6)
	if v.maxScroll() == 0 {
		t.Fatal("expected scrollable content")
	}
	for _, s := range []int{0, 2, 99} {
		box := v.render(s)
		if len(box) != v.height+2 {
			t.Fatalf("scroll %d: %d rows", s, len(box))
		}
		w := ansi.StringWidth(box[0])
		for _, row := range box {
			if ansi.StringWidth(row) != w {
				t.Fatalf("scroll %d: ragged box", s)
			}
		}
	}
	if !strings.Contains(ansi.Strip(v.render(0)[v.height+1]), "↓") {
		t.Fatal("missing scroll indicator")
	}
}

func plain(ls []hoverLine) string {
	var sb strings.Builder
	for _, l := range ls {
		for _, c := range l.cells {
			sb.WriteRune(c.r)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func TestHoverWrapNarrowWideRunes(t *testing.T) {
	done := make(chan struct{})
	go func() {
		hoverLines("あい", 1)
		hoverLines("```go\nあ\n```", 1)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("wrapCells did not terminate")
	}
}

func TestHoverRuleDoesNotWidenBox(t *testing.T) {
	v := newHoverView("```go\nvar x int\n```\n\n---\n\nshort", 80, 10)
	if v.width != len("var x int") {
		t.Fatalf("width=%d", v.width)
	}
}

func TestHoverPlaintextFallback(t *testing.T) {
	lines := hoverLines("func Println(a ...any)\n\nPrintln formats using the default formats.", 60)
	for _, l := range lines {
		if l.code || l.rule {
			t.Fatalf("plaintext rendered as code/rule: %+v", l)
		}
	}
	if !strings.Contains(plain(lines), "func Println(a ...any)") {
		t.Fatalf("text lost:\n%s", plain(lines))
	}
}
