package core

import (
	"image"
	"image/png"
	"os"
	"strings"
	"testing"
)

const mdSample = "# 見出し\n\n段落に **太字** と `code` と [リンク](http://x)。\n\n- 項目1\n- 項目2\n  - 入れ子\n\n1. 一\n2. 二\n\n```go\nfunc main() {}\n```\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n```mermaid\ngraph TD; A-->B\n```\n"

func TestMermaidBlocks(t *testing.T) {
	got := MermaidBlocks(mdSample)
	if len(got) != 1 || got[0] != "graph TD; A-->B" {
		t.Fatalf("got %q", got)
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
		got := MermaidBlocks(src)
		if strings.Join(got, "|") != strings.Join(want, "|") || len(got) != len(want) {
			t.Errorf("%q: got %q want %q", src, got, want)
		}
	}
}

func TestPreviewSourcesFindsSoleImages(t *testing.T) {
	src := "![図](img/a.png)\n\n文の中の ![b](b.png) は文字のまま\n\n> ![c](c.png)\n"
	_, images := previewSources(src)
	if strings.Join(images, "|") != "img/a.png|c.png" {
		t.Fatalf("images = %q", images)
	}
}

func TestMDImagePath(t *testing.T) {
	cases := map[string]string{
		"a.png":                 "/doc/a.png",
		"../img/a%20b.png":      "/img/a b.png",
		"/abs/a.png":            "/abs/a.png",
		"https://example.com/a": "",
		"data:image/png;base64": "",
		"":                      "",
	}
	for src, want := range cases {
		if got := mdImagePath("/doc/x.md", src); got != want {
			t.Errorf("%q: got %q want %q", src, got, want)
		}
	}
}

func writePNG(t *testing.T, path string, w, h int) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	png.Encode(f, image.NewRGBA(image.Rect(0, 0, w, h)))
	f.Close()
}
