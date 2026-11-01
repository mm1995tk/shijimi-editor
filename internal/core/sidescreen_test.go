package core

import (
	"os"
	"path/filepath"
	"testing"
)

// The screen of the tree holds only the rows in view, and building it
// changes nothing: the scroll is fitted before (EnsureVisible).
func TestExplorerScreenRowsInView(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a", "b", "c", "d", "e"} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	e := NewExplorer(dir)
	e.Sel = 4
	e.EnsureVisible(2) // a window of 2 rows
	before := e.Scroll
	ex := e.Screen(3, filepath.Join(dir, "a"))
	if e.Scroll != before {
		t.Fatalf("screen moved the scroll: %d -> %d", before, e.Scroll)
	}
	if ex.Top != 3 || ex.Total != 5 || len(ex.Rows) != 2 {
		t.Fatalf("Top=%d Total=%d rows=%d", ex.Top, ex.Total, len(ex.Rows))
	}
	if !ex.Rows[1].Selected || ex.Rows[1].Name != "e" || ex.Rows[0].Selected {
		t.Errorf("rows = %+v", ex.Rows)
	}
}
