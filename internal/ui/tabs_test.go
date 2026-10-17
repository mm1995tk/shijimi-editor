package ui

import (
	"os"
	"path/filepath"
	"testing"
)

// Closing the last tab leaves active at -1, which used to drag tabOffset to -1
// and index pieces[-1] once a new tab was opened.
func TestRenderTabsAfterClosingLastTab(t *testing.T) {
	a := newTestApp(t, "one\n")
	a.closeTab(true)
	if len(a.tabs) != 0 {
		t.Fatalf("tabs = %d", len(a.tabs))
	}
	a.View()
	p := filepath.Join(a.root, "b.txt")
	os.WriteFile(p, []byte("two\n"), 0o644)
	if !a.openFile(p) {
		t.Fatal("openFile failed")
	}
	a.View()
	if a.tabOffset < 0 {
		t.Fatalf("tabOffset = %d", a.tabOffset)
	}
}
