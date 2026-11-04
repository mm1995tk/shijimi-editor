package core

// Sizes is how big the screen and the editor's panes are, in cells. The UI
// works it out (a terminal lays its panels out itself; the GUI reports the
// size of each region) and hands it to the core as an Input whenever it
// changes. The core never asks the UI for a size: it reads the last Sizes it
// was given.
type Sizes struct {
	// ScreenW and ScreenH are the whole screen, the status line included.
	ScreenW, ScreenH int
	// EditRows is the number of text rows the editor shows.
	EditRows int
	// EditorCols is the width of the editor area, which a split divides.
	EditorCols int
	// TextCols is the width of the focused pane's text, its line numbers
	// included, from which a sideways scroll subtracts GutterWidth.
	TextCols int
	// DiffBaseCols is the width of the base side of a split diff.
	DiffBaseCols int
	// SuggestionsShown is whether the suggestion panel is open, in which case
	// the status line leaves the suggestion count to it.
	SuggestionsShown bool
}

func (Sizes) input() {}

// SetSizes takes the sizes the UI gave. A new screen size makes the scroll
// follow the cursor again.
func (a *Core) SetSizes(s Sizes) {
	if a.sizes.ScreenW != s.ScreenW || a.sizes.ScreenH != s.ScreenH {
		a.Follow = true
	}
	a.sizes = s
}

// ScreenW is the width of the screen in cells.
func (a *Core) ScreenW() int { return a.sizes.ScreenW }

// ScreenH is the height of the screen in cells.
func (a *Core) ScreenH() int { return a.sizes.ScreenH }

// EditRows is the number of text rows the editor shows.
func (a *Core) EditRows() int { return a.sizes.EditRows }

// TextCols is the width of the focused pane's text, line numbers included.
func (a *Core) TextCols() int { return a.sizes.TextCols }

// DiffBaseCols is the width of the base side of a split diff.
func (a *Core) DiffBaseCols() int { return a.sizes.DiffBaseCols }

// EditorCols is the width of the editor area, which a split divides.
func (a *Core) EditorCols() int { return a.sizes.EditorCols }

// SuggestionsShown reports whether the suggestion panel is open.
func (a *Core) SuggestionsShown() bool { return a.sizes.SuggestionsShown }
