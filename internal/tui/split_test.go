package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// P13-07 (#383) sorts the files of this package by what they may know, ahead
// of the split into core and tui (P13-08, docs/design/gui-protocol.md):
//
//   - lipgloss is imported only by the files that draw. The state that decides
//     what is shown never builds a style.
//   - the layout (cells of each pane) is used only by the files that draw or
//     locate a screen cell. Code that goes to core asks editRows / textCols
//     (layout.go) instead.

// lipglossFiles are the files that may import lipgloss.
var lipglossFiles = []string{
	"app_draw.go", "editor_draw.go", "explorer_draw.go", "ideproposal_draw.go",
	"inputview.go", "lines_draw.go", "preview_draw.go", "styles.go", "theme_apply.go",
}

// layoutFiles are the files that may use layout: they draw, or turn a screen
// cell into a region and back. They stay in tui.
var layoutFiles = []string{
	"layout.go", "caret.go", "inputtea.go", "signature.go", "spacemenu.go",
	"window.go", "prreview.go", "gitdiff.go",
}

// layoutTodo are files that go to core but still take a layout, mostly for
// mouse hit tests (#383 keeps going). Shrink this list; never add to it.
var layoutTodo = []string{
	"agentpanel.go", "agentsel.go", "app.go", "hovermouse.go", "resize.go", "searchui.go",
}

func TestFileKnowledge(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if strings.HasPrefix(p, "charm.land/lipgloss") && !slices.Contains(lipglossFiles, name) {
				t.Errorf("%s imports lipgloss; only %v may", name, lipglossFiles)
			}
		}
		usesLayout := false
		ast.Inspect(f, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == "layout" {
				usesLayout = true
			}
			return !usesLayout
		})
		if usesLayout && !slices.Contains(layoutFiles, name) && !slices.Contains(layoutTodo, name) {
			t.Errorf("%s uses layout; ask editRows / textCols (layout.go) instead", name)
		}
		if !usesLayout && slices.Contains(layoutTodo, name) {
			t.Errorf("%s no longer uses layout; remove it from layoutTodo", name)
		}
	}
}
