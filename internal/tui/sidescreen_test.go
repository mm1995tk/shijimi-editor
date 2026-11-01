package tui

import (
	"os"
	"path/filepath"
	"testing"
)

// The screen of the tree holds only the rows in view, and building it
// changes nothing: the scroll is fitted before (ensureVisible).
func TestExplorerScreenRowsInView(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a", "b", "c", "d", "e"} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	e := newExplorer(dir)
	e.sel = 4
	e.ensureVisible(2) // a window of 2 rows
	before := e.scroll
	ex := e.screen(3, filepath.Join(dir, "a"))
	if e.scroll != before {
		t.Fatalf("screen moved the scroll: %d -> %d", before, e.scroll)
	}
	if ex.Top != 3 || ex.Total != 5 || len(ex.Rows) != 2 {
		t.Fatalf("Top=%d Total=%d rows=%d", ex.Top, ex.Total, len(ex.Rows))
	}
	if !ex.Rows[1].Selected || ex.Rows[1].Name != "e" || ex.Rows[0].Selected {
		t.Errorf("rows = %+v", ex.Rows)
	}
}

// Drawing the agent panel twice gives the same rows: fit already put the
// scroll and the selection where they go.
func TestAgentScreenDrawsTwiceTheSame(t *testing.T) {
	a, l := agentSelApp(t, logEntry{kind: logInfo, text: "one\ntwo"}, logEntry{kind: logAgent, text: "# h\n\ntext"})
	one := drawAgent(a.agentScreen(l.agentW, l.bodyH, true), l.agentW, l.bodyH, true)
	scroll, sel := a.ag.scroll, a.sg.sel
	two := drawAgent(a.agentScreen(l.agentW, l.bodyH, true), l.agentW, l.bodyH, true)
	if a.ag.scroll != scroll || a.sg.sel != sel {
		t.Fatal("building the screen changed the state")
	}
	if len(one) != l.bodyH || len(one) != len(two) {
		t.Fatalf("rows %d %d, want %d", len(one), len(two), l.bodyH)
	}
	for i := range one {
		if one[i] != two[i] {
			t.Fatalf("row %d differs", i)
		}
	}
}
