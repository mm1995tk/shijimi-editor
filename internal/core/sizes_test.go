package core

import "testing"

// The core reads the sizes the UI gave it as an Input, and a new screen size
// makes the scroll follow the cursor again.
func TestSizesInput(t *testing.T) {
	a := newApp(t.TempDir(), "")
	a.Update(Sizes{ScreenW: 80, ScreenH: 24, EditRows: 20, EditorCols: 60, TextCols: 58, DiffBaseCols: 29, SuggestionsShown: true})
	if a.ScreenW() != 80 || a.ScreenH() != 24 || a.EditRows() != 20 || a.EditorCols() != 60 ||
		a.TextCols() != 58 || a.DiffBaseCols() != 29 || !a.SuggestionsShown() {
		t.Fatalf("sizes not taken: %+v", a.sizes)
	}
	a.Follow = false
	a.Update(Sizes{ScreenW: 80, ScreenH: 24, EditRows: 19})
	if a.Follow {
		t.Error("the scroll followed the cursor although the screen kept its size")
	}
	a.Update(Sizes{ScreenW: 100, ScreenH: 24})
	if !a.Follow {
		t.Error("the scroll did not follow the cursor after the screen changed size")
	}
}
