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

// This package is the terminal front end of internal/core: it draws a
// core.Core to a terminal and turns what bubbletea delivers into the Core's
// Inputs and Events. What may reach into core, and what core may know of a
// terminal, is kept by the package boundary (core's fields and methods are
// unexported, core's TestNoTerminalDeps). Two rules remain within this package:
//
//   - lipgloss is imported only by the files that draw. The files that decide
//     what to draw (screen.Overlay, screen.Pane) never build a style.
//   - the layout (cells of each pane) is used only by the files that draw or
//     locate a screen cell.

// lipglossFiles are the files that may import lipgloss.
var lipglossFiles = []string{
	"app_draw.go", "editor_draw.go", "explorer_draw.go", "ideproposal_draw.go",
	"inputview.go", "lines_draw.go", "preview_draw.go", "styles.go", "theme_apply.go",
}

// layoutFiles are the files that may use layout: they draw, or turn a screen
// cell into a region and back.
var layoutFiles = []string{
	"layout.go", "mouse.go", "resize.go", "caret.go", "inputtea.go", "app_draw.go", "gitdiff_draw.go", "prreview_draw.go",
	"signature_draw.go", "spacemenu_draw.go",
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
		if usesLayout && !slices.Contains(layoutFiles, name) {
			t.Errorf("%s uses layout; only %v may", name, layoutFiles)
		}
	}
}
