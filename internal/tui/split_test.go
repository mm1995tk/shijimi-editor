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

// This package is split into files that draw and files that decide what is
// shown (#383); the two lists below say which file may know what, ahead
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
	"layout.go", "mouse.go", "resize.go", "caret.go", "inputtea.go", "app_draw.go", "gitdiff_draw.go", "prreview_draw.go",
	"signature_draw.go", "spacemenu_draw.go",
	"window.go",
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
			t.Errorf("%s uses layout; ask editRows / textCols (layout.go) instead", name)
		}
	}
}

// tuiFields are the fields of App that belong to the TUI (the table in
// docs/design/gui-protocol.md): the size of the screen, which regions are
// shown and the terminal under the editor.
var tuiFields = []string{"w", "h", "showSide", "showTerm", "showAgent", "term"}

// TestRenderFrameReadsQueries keeps renderFrame from reading the state of App
// directly. What is shown comes from the screens (Screen, Pane, Overlay) and
// from questions App answers (a.cur(), a.diffMode(), a.focusIn(...)), which
// become core.Screen() and core's query methods when App is split (#384).
// Only the TUI's own fields may be read by name.
func TestRenderFrameReadsQueries(t *testing.T) {
	fset := token.NewFileSet()
	appFile, err := parser.ParseFile(fset, "app.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]bool{}
	ast.Inspect(appFile, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name.Name != "App" {
			return true
		}
		for _, f := range ts.Type.(*ast.StructType).Fields.List {
			for _, id := range f.Names {
				fields[id.Name] = true
			}
		}
		return false
	})
	drawFile, err := parser.ParseFile(fset, "app_draw.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	seen := false
	for _, d := range drawFile.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Name.Name != "renderFrame" {
			continue
		}
		seen = true
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "a" && fields[sel.Sel.Name] && !slices.Contains(tuiFields, sel.Sel.Name) {
				t.Errorf("renderFrame reads a.%s; ask a method of App instead (%s)", sel.Sel.Name, fset.Position(sel.Pos()))
			}
			return true
		})
	}
	if !seen {
		t.Fatal("renderFrame not found in app_draw.go")
	}
}
