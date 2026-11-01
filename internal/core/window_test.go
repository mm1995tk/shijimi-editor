package core

import (
	"testing"
)

// splitApp is an app with a.txt open on the left and b.txt on the right.
func splitApp(t *testing.T) (*App, []string) {
	t.Helper()
	a := newTestApp(t, "one\n")
	ps := writeFiles(t, a.root, "b.txt", "c.txt")
	a.Update(Key{Code: KeyEscape})
	a.openFile(ps[0])
	a.TrackLast()
	a.openFile(a.tabs[0].Path())
	a.TrackLast()
	keys(a, "space", "w", "v")
	return a, ps
}

func TestSplitCloseFileCollapses(t *testing.T) {
	a, ps := splitApp(t)
	a.closeTab(true)
	a.Update(Key{Code: KeyEscape})
	if a.split || a.Cur() == nil || a.Cur().Path() != a.tabs[0].Path() || ps == nil {
		t.Fatalf("split=%v cur=%v", a.split, a.Cur())
	}
}
