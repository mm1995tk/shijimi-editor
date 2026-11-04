package core

// headless is the front end of a core nothing draws yet: a workspace of `shj
// serve` before a GUI looks at it. It has only the editor on its screen, no
// terminal panel and no multiplexer; until the GUI sends a size the screen is
// 80 x 24 cells (newHeadlessCore). The GUI will tell the
// sizes of its regions (view.resize) and pick the theme (theme.get); until
// then the core keeps its state as it would behind any screen.
type headless struct{}

// newHeadless is a headless front end.
func newHeadless() *headless { return &headless{} }

// newHeadlessCore is a core rooted at dir on a headless front end.
func newHeadlessCore(sh *Shared, dir string) (*Core, error) {
	c, err := NewIn(sh, newHeadless(), dir, "")
	if err != nil {
		return nil, err
	}
	c.SetSizes(Sizes{ScreenW: 80, ScreenH: 24, EditRows: 23, EditorCols: 80, TextCols: 80})
	return c, nil
}

func (h *headless) ReleasesReported() bool   { return false }
func (h *headless) SetReleasesReported(bool) {}
func (h *headless) SetPalette(*Theme)        {} // the theme is the process's (ActivateTheme); the GUI asks for it with theme.get
