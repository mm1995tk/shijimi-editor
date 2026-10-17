package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestNodeLessDirsFirstDotFirst(t *testing.T) {
	ns := []*node{
		{name: ".gitignore"}, {name: ".github", isDir: true}, {name: "README.md"},
		{name: "src", isDir: true}, {name: "go.mod"}, {name: ".env"},
	}
	sort.SliceStable(ns, func(i, j int) bool { return nodeLess(ns[i], ns[j]) })
	want := []string{".github", "src", ".env", ".gitignore", "go.mod", "README.md"}
	for i, n := range ns {
		if n.name != want[i] {
			t.Fatalf("pos %d: got %s want %s", i, n.name, want[i])
		}
	}
}

func TestExplorerRenderIndentGuides(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a", "b", "f.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	e := newExplorer(dir)
	e.reveal(filepath.Join(dir, "a", "b", "f.txt"))
	lines := e.render(40, 4, false, "")
	want := []string{" ▾ a", " │ ▾ b", " │ │   f.txt"}
	for i, w := range want {
		got := strings.TrimRight(ansi.Strip(lines[i+1]), " ")
		if got != w {
			t.Fatalf("row %d: got %q want %q", i, got, w)
		}
	}
}

func TestExplorerRenderChevronWithIcons(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	e := newExplorer(dir)
	e.icons = true
	e.toggle(e.flat[0])
	lines := e.render(40, 3, false, "")
	if got := ansi.Strip(lines[1]); !strings.HasPrefix(got, "  ") {
		t.Fatalf("expanded dir: got %q", got)
	}
	if got := ansi.Strip(lines[2]); !strings.HasPrefix(got, "  ") {
		t.Fatalf("collapsed dir: got %q", got)
	}
}
