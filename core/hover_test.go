package core

import (
	"strings"
	"testing"
	"time"
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

func plain(ls []hoverLine) string {
	var sb strings.Builder
	for _, l := range ls {
		for _, c := range l.cells {
			sb.WriteRune(c.R)
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
	v := NewHoverView("```go\nvar x int\n```\n\n---\n\nshort", 80, 10)
	if v.Width != len("var x int") {
		t.Fatalf("width=%d", v.Width)
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
