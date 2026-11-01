package core

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// widthFree are the files in internal/core that already work without the width
// of terminal cells (docs/design/gui-protocol.md). Add a file here once its
// widths have moved to the TUI.
var widthFree = []string{"hoverparse.go", "lineinput.go"}

func TestWidthFreeFiles(t *testing.T) {
	for _, name := range widthFree {
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if strings.Contains(p, "x/ansi") || strings.Contains(p, "runewidth") || strings.Contains(p, "uniseg") || strings.Contains(p, "lipgloss") {
				t.Errorf("%s imports %s", name, p)
			}
		}
	}
}
