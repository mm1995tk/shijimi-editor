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

// TestRenderFrameReadsQueries keeps renderFrame from reading the state of App
// directly. What is shown comes from the screens (Screen, Pane, Overlay) and
// from questions App answers (a.cur(), a.diffMode(), a.focusIn(...)), which
// become core.Screen() and core's query methods when App is split (#384).
// Only the fields that stay on App (appOnlyFields) may be read by name.
func TestRenderFrameReadsQueries(t *testing.T) {
	fset := token.NewFileSet()
	appFile, err := parser.ParseFile(fset, "app.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]bool{}
	ast.Inspect(appFile, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name.Name != "App" && ts.Name.Name != "Core" {
			return true
		}
		for _, f := range ts.Type.(*ast.StructType).Fields.List {
			for _, id := range f.Names {
				fields[id.Name] = true
			}
		}
		return true
	})
	if len(fields) == 0 || !fields["gd"] {
		t.Fatal("the fields of App and Core were not found in app.go")
	}
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
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "a" && fields[sel.Sel.Name] && !slices.Contains(appOnlyFields, sel.Sel.Name) {
				t.Errorf("renderFrame reads a.%s; ask a method of App instead (%s)", sel.Sel.Name, fset.Position(sel.Pos()))
			}
			return true
		})
	}
	if !seen {
		t.Fatal("renderFrame not found in app_draw.go")
	}
}

// appOnlyFields are the fields that stay on App when it is split: they are
// not in Core (the struct, in app.go).
var appOnlyFields = []string{
	"caret", "cellSize", "w", "h", "term", "mux", "showSide", "showTerm", "showAgent",
	"sideWPref", "panelHPref", "previewPct", "diffLeftPct", "explorerPct", "agentWPref",
	"resizeMode", "resizing", "lastClick", "keyReleases", "askReleases", "quitting", "capture",
}

// appFieldFiles are the files that stay in tui and so may read appOnlyFields.
var appFieldFiles = []string{
	"app_draw.go", "caret.go", "frontend.go", "model.go", "layout.go", "mouse.go", "resize.go", "mux.go",
	"eventtui.go", "inputtea.go", "ideproposal_draw.go", "prreview_draw.go",
}

// TestCoreFields keeps Core free of the fields that stay on App, and files
// that go to core from reading them (they ask the front end: frontend.go).
func TestCoreFields(t *testing.T) {
	fset := token.NewFileSet()
	appFile, err := parser.ParseFile(fset, "app.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	ast.Inspect(appFile, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name.Name != "Core" {
			return true
		}
		for _, f := range ts.Type.(*ast.StructType).Fields.List {
			for _, id := range f.Names {
				if slices.Contains(appOnlyFields, id.Name) {
					t.Errorf("Core has the field %s, which stays on App", id.Name)
				}
			}
		}
		return false
	})
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		var reads []string
		ast.Inspect(f, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == "a" && slices.Contains(appOnlyFields, sel.Sel.Name) {
					if !slices.Contains(reads, sel.Sel.Name) {
						reads = append(reads, sel.Sel.Name)
					}
				}
			}
			return true
		})
		if len(reads) > 0 && !slices.Contains(appFieldFiles, name) {
			t.Errorf("%s reads %v of App, which stay on App; only %v may", name, reads, appFieldFiles)
		}
	}
}

// tuiMethodFiles are the files whose methods stay on App: they draw, locate a
// screen cell, or talk to bubbletea or the multiplexer. The methods of every
// other file are Core's.
var tuiMethodFiles = []string{
	"app_draw.go", "caret.go", "frontend.go", "model.go", "layout.go", "mouse.go", "resize.go",
	"mux.go", "eventtui.go", "inputtea.go", "ideproposal_draw.go", "prreview_draw.go",
	"editor_draw.go", "explorer_draw.go", "gitdiff_draw.go", "inputview.go", "lines_draw.go",
	"preview_draw.go", "signature_draw.go", "spacemenu_draw.go", "styles.go", "theme_apply.go",
}

// TestCoreMethods keeps the methods of the files that go to core on Core.
func TestCoreMethods(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || slices.Contains(tuiMethodFiles, name) {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || len(fd.Recv.List) != 1 {
				continue
			}
			star, ok := fd.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			if id, ok := star.X.(*ast.Ident); ok && id.Name == "App" {
				t.Errorf("%s: %s is a method of App; the methods of this file are Core's", name, fd.Name.Name)
			}
		}
	}
}

// TestThemeApplyIsFrontEnd keeps the files that go to core from applying a
// theme to the styles themselves: they ask the front end (setPalette).
func TestThemeApplyIsFrontEnd(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || slices.Contains(tuiMethodFiles, name) {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && (id.Name == "applyTheme" || id.Name == "rebuildStyles") {
				t.Errorf("%s uses %s; ask the front end (setPalette, frontend.go)", name, id.Name)
			}
			return true
		})
	}
}

// TestFrontEndAsksCore keeps the files that stay in tui from reaching into
// Core's fields: they ask its methods (frameq.go, ptrq.go, ...), which are
// the questions core answers once it is its own package (#384). A field
// that stays on App (appOnlyFields) is theirs to read.
func TestFrontEndAsksCore(t *testing.T) {
	fset := token.NewFileSet()
	appFile, err := parser.ParseFile(fset, "app.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]bool{}
	ast.Inspect(appFile, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name.Name != "Core" {
			return true
		}
		for _, f := range ts.Type.(*ast.StructType).Fields.List {
			for _, id := range f.Names {
				fields[id.Name] = true
			}
		}
		return false
	})
	if !fields["gd"] {
		t.Fatal("the fields of Core were not found in app.go")
	}
	for _, name := range tuiMethodFiles {
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "a" && fields[sel.Sel.Name] && !slices.Contains(appOnlyFields, sel.Sel.Name) {
				t.Errorf("%s reads a.%s of Core; ask a method of Core instead (%s)", name, sel.Sel.Name, fset.Position(sel.Pos()))
			}
			return true
		})
	}
}
