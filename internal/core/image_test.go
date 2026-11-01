package core

import (
	"testing"
)

func TestFitImage(t *testing.T) {
	CellW, CellH = 10, 20
	// a large image shrinks to the box and keeps its aspect ratio
	pw, ph, cols, rows := FitImage(2000, 1000, 50, 40)
	if pw != 500 || ph != 250 || cols != 50 || rows != 13 {
		t.Errorf("large: %d×%d px on %d×%d cells", pw, ph, cols, rows)
	}
	// a small image is never enlarged
	pw, ph, cols, rows = FitImage(32, 16, 50, 40)
	if pw != 32 || ph != 16 || cols != 4 || rows != 1 {
		t.Errorf("small: %d×%d px on %d×%d cells", pw, ph, cols, rows)
	}
}
