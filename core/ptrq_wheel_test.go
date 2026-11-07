package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The wheel stops scrolling the editor when the last line of the file is on
// the bottom row: it does not scroll on into the empty space below it.
func TestWheelEditorStopsAtEnd(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file, []byte(strings.Repeat("x\n", 99)+"x"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, f := newTestCore(t, dir, file, 80, 24)
	f.push() // the rows to edit come from the body, which needs the screen first
	h := a.EditRows()
	if h <= 0 {
		t.Fatalf("no rows to edit: %d", h)
	}
	a.WheelEditor(1000)
	if got, want := a.Cur().Buf.ScrollRow, 100-h; got != want {
		t.Errorf("scroll row %d, want %d (100 lines, %d rows)", got, want, h)
	}
}

// A file shorter than the view does not scroll at all.
func TestMaxScrollRow(t *testing.T) {
	for _, c := range []struct{ n, h, want int }{
		{100, 20, 80}, {10, 20, 0}, {20, 20, 0}, {5, 0, 4}, {0, 10, 0},
	} {
		if got := MaxScrollRow(c.n, c.h); got != c.want {
			t.Errorf("MaxScrollRow(%d, %d) = %d, want %d", c.n, c.h, got, c.want)
		}
	}
}
