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

	tea "github.com/charmbracelet/bubbletea"
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
	a.tabs[0].preview = previewSplit

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

func TestMermaidBlocksMatchRenderer(t *testing.T) {
	cases := map[string][]string{
		"```mermaid\nA-->B\n```\n```mermaid\nC-->D\n```\n": {"A-->B", "C-->D"},
		"```mermaid\n```\n":              {""},
		"```mermaid\nA-->B":              {"A-->B"},
		"> ```mermaid\n> A-->B\n> ```\n": {"A-->B"},
	}
	for src, want := range cases {
		got := mermaidBlocks(src)
		if strings.Join(got, "|") != strings.Join(want, "|") || len(got) != len(want) {
			t.Errorf("%q: got %q want %q", src, got, want)
		}
	}
}

func TestLongFenceNotClosedByShorter(t *testing.T) {
	src := "````md\n```mermaid\ngraph TD\n```\n````\n\nafter text\n"
	if got := mermaidBlocks(src); len(got) != 0 {
		t.Fatalf("example inside a longer fence must not be drawn: %q", got)
	}
	plain := ansi.Strip(strings.Join(renderMarkdown(src, 40, mdOpts{}), "\n"))
	if !strings.Contains(plain, "after text") || strings.Contains(plain, "````") {
		t.Fatalf("unexpected render:\n%s", plain)
	}
}

func TestMermaidJobRestoredAfterTabSwitch(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	a1 := buffer.New()
	a1.Insert("```mermaid\nA-->B\n```\n")
	a1.Path = "a.md"
	b1 := buffer.New()
	b1.Insert("other")
	b1.Path = "b.md"
	a := &App{cfg: &config.Config{Editor: config.Editor{MermaidCommand: "true {in} {out}"}}, tabs: []*tab{newTab(a1), newTab(b1)}}
	a.tabs[0].preview = previewSplit

	if a.syncPreview() == nil {
		t.Fatal("expected a scheduled render")
	}
	code := mermaidBlocks(a1.Text())[0]
	key := a.mermaidKey(code)
	a.active = 1 // switch away before the timer fires
	if cmd := a.handleMermaidStart(mermaidStartMsg{key, code}); cmd != nil {
		t.Fatal("job for a hidden tab must not start")
	}
	if _, ok := a.mermaid[key]; ok {
		t.Fatal("job should have been dropped")
	}
	a.active = 0 // back, without editing
	if a.syncPreview() == nil {
		t.Fatal("job must be re-registered after returning to the tab")
	}
	if _, ok := a.mermaid[key]; !ok {
		t.Fatal("job missing after return")
	}
}

func TestTogglePreviewCyclesToFullWidth(t *testing.T) {
	b := buffer.New()
	b.Insert("# 見出し\n")
	b.Path = "x.md"
	a := &App{cfg: &config.Config{}, tabs: []*tab{newTab(b)}}
	l := layout{rightW: 81, editH: 5}
	tb := a.tabs[0]

	want := []struct {
		mode  previewMode
		editW int
	}{{previewSplit, 40}, {previewFull, 0}, {previewOff, 81}}
	for _, w := range want {
		a.togglePreview()
		if tb.preview != w.mode || a.editW(tb, l) != w.editW {
			t.Fatalf("mode %v editW %d, want %v %d", tb.preview, a.editW(tb, l), w.mode, w.editW)
		}
	}

	tb.preview = previewFull
	lines := a.renderText(tb, l, true)
	if len(lines) != l.editH || !strings.Contains(ansi.Strip(lines[0]), "見出し") {
		t.Fatalf("full preview: %q", lines)
	}
	for _, s := range lines {
		if ansi.StringWidth(s) != l.rightW {
			t.Fatalf("width %d, want %d", ansi.StringWidth(s), l.rightW)
		}
	}
}

func TestFullPreviewBlocksEditing(t *testing.T) {
	b := buffer.New()
	b.Insert("# 見出し\n")
	b.Path = "x.md"
	a := &App{cfg: &config.Config{}, tabs: []*tab{newTab(b)}, helix: true, mode: modeInsert}
	tb := a.tabs[0]
	tb.preview = previewFull
	ver := b.Version

	press := func(s string) {
		a.handleHelixKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	}
	for _, k := range []string{"i", "a", "o", "d", "x", "x", "K"} {
		press(k)
		if a.mode != modeNormal || b.Version != ver {
			t.Fatalf("key %q: mode %v, version %d (want %d)", k, a.mode, b.Version, ver)
		}
	}

	press("j") // scrolling still works
	if a.pending != "" {
		t.Fatalf("pending %q", a.pending)
	}

	a.Update(hoverMsg{text: "doc"})
	if a.hoverText != "" {
		t.Fatalf("hover shown in full preview: %q", a.hoverText)
	}
}

func TestFullPreviewBlocksLeaderPasteAndVscodeKeymap(t *testing.T) {
	b := buffer.New()
	b.Insert("abc\n")
	b.Path = "x.md"
	a := &App{cfg: &config.Config{}, tabs: []*tab{newTab(b)}, helix: true, pal: &palette{}, search: &searchPanel{}}
	a.tabs[0].preview = previewFull
	ver := b.Version

	a.handleHelixKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	a.handleHelixKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	if b.Version != ver || a.pending != "" {
		t.Fatalf("Space p: version %d (want %d), pending %q", b.Version, ver, a.pending)
	}

	a.helix = false
	a.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	a.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("pasted"), Paste: true})
	if b.Version != ver {
		t.Fatalf("vscode keymap edited the hidden buffer: version %d, want %d", b.Version, ver)
	}
}
