package ui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"

	"github.com/mm1995tk/gh-editor/internal/buffer"
	"github.com/mm1995tk/gh-editor/internal/config"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

const mdSample = "# 見出し\n\n段落に **太字** と `code` と [リンク](http://x)。\n\n- 項目1\n- 項目2\n  - 入れ子\n\n1. 一\n2. 二\n\n```go\nfunc main() {}\n```\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n```mermaid\ngraph TD; A-->B\n```\n"

func TestRenderMarkdownWidth(t *testing.T) {
	for _, w := range []int{20, 40, 80} {
		called := false
		lines := renderMarkdown(mdSample, w, mdOpts{mermaid: func(code string, w int) []string {
			called = strings.Contains(code, "A-->B")
			return []string{strings.Repeat("x", w)}
		}})
		if !called {
			t.Fatal("mermaid callback not called")
		}
		for i, l := range lines {
			if got := ansi.StringWidth(l); got != w {
				t.Fatalf("w=%d line %d width=%d: %q", w, i, got, ansi.Strip(l))
			}
		}
		plain := ansi.Strip(strings.Join(lines, "\n"))
		for _, want := range []string{"見出し", "太字", "• 項目1", "1.", "func main", "┌", "xxxx"} {
			if !strings.Contains(plain, want) {
				t.Errorf("w=%d: missing %q in\n%s", w, want, plain)
			}
		}
		if strings.Contains(plain, "**") || strings.Contains(plain, "```") {
			t.Errorf("w=%d: markup left over\n%s", w, plain)
		}
	}
}

func TestMermaidBlocks(t *testing.T) {
	got := mermaidBlocks(mdSample)
	if len(got) != 1 || got[0] != "graph TD; A-->B" {
		t.Fatalf("got %q", got)
	}
}

func TestMermaidRenderWithExternalCommand(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	src := filepath.Join(t.TempDir(), "fixture.png")
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	f, _ := os.Create(src)
	png.Encode(f, img)
	f.Close()

	b := buffer.New()
	b.Insert("# t\n\n```mermaid\ngraph TD; A-->B\n```\n")
	b.Path = "x.md"
	cfg := &config.Config{Editor: config.Editor{MermaidCommand: "cp " + shellQuote(src) + " {out}"}}
	a := &App{cfg: cfg, tabs: []*tab{newTab(b)}}
	a.tabs[0].preview = true

	if cmd := a.syncPreview(); cmd == nil {
		t.Fatal("expected a scheduled render")
	}
	code := mermaidBlocks(b.Text())[0]
	key := a.mermaidKey(code)
	if got := a.mermaidLines(code, 30); !strings.Contains(ansi.Strip(strings.Join(got, "\n")), "描画中") {
		t.Fatalf("pending placeholder expected: %q", got)
	}
	msg := a.handleMermaidStart(mermaidStartMsg{key, code})()
	done, ok := msg.(mermaidDoneMsg)
	if !ok || done.err != nil {
		t.Fatalf("msg = %#v", msg)
	}
	a.handleMermaidDone(done)
	lines := a.mermaidLines(code, 30)
	if len(lines) < 3 || strings.Contains(ansi.Strip(strings.Join(lines, "")), "mermaid") {
		t.Fatalf("diagram expected, got %d lines", len(lines))
	}
	for _, l := range lines {
		if ansi.StringWidth(l) != 30 {
			t.Fatalf("width %d", ansi.StringWidth(l))
		}
	}
}
