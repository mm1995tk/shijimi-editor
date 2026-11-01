package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// render fits the tree to h rows and draws it, the way View does.
func (e *explorer) render(w, h int, focused bool, activePath string) []string {
	e.ensureVisible(h - 1)
	return drawExplorer(e.screen(h, activePath), w, h, focused)
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

func TestExplorerDimsIgnoredFiles(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	dir := t.TempDir()
	if out, err := run(dir, nil, "git", "init", "-q"); err != nil {
		t.Skipf("git unavailable: %v %s", err, out)
	}
	if err := os.MkdirAll(filepath.Join(dir, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		".gitignore": "build/\n*.log\n", "a.log": "", "main.go": "", "build/out.txt": "",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	e := newExplorer(dir)
	e.reveal(filepath.Join(dir, "build", "out.txt"))
	byName := map[string]*node{}
	for _, n := range e.flat {
		byName[n.name] = n
	}
	// An ignored directory stands for everything under it.
	for _, name := range []string{"build", "out.txt", "a.log"} {
		if !e.isIgnored(byName[name]) {
			t.Errorf("%s: not ignored", name)
		}
		if l := e.git[byName[name].path]; l != "" {
			t.Errorf("%s: git letter %q", name, l)
		}
	}
	for _, name := range []string{".gitignore", "main.go"} {
		if e.isIgnored(byName[name]) {
			t.Errorf("%s: ignored", name)
		}
		if l := e.git[byName[name].path]; l != "U" {
			t.Errorf("%s: git letter %q want U", name, l)
		}
	}
	// Ignored rows are dimmed and carry no status letter.
	var r, g, b int
	if _, err := fmt.Sscanf(CurTheme.Dim, "#%02x%02x%02x", &r, &g, &b); err != nil {
		t.Fatalf("theme Dim %q: %v", CurTheme.Dim, err)
	}
	seq := fmt.Sprintf("38;2;%d;%d;%d", r, g, b)
	for i, n := range e.flat {
		line := e.render(40, len(e.flat)+1, false, "")[i+1]
		plain := strings.TrimRight(ansi.Strip(line), " ")
		if e.isIgnored(n) {
			if !strings.Contains(line, seq) {
				t.Errorf("%s: not dimmed: %q", n.name, line)
			}
			if strings.HasSuffix(plain, " U") {
				t.Errorf("%s: has status letter: %q", n.name, plain)
			}
		} else if !strings.HasSuffix(plain, " U") {
			t.Errorf("%s: missing status letter: %q", n.name, plain)
		}
	}
}
