package ui

import tea "charm.land/bubbletea/v2"

// \ shows the line numbers the other way (absolute or relative, the opposite
// of editor.line-number), so both are at hand without two columns:
//   - When the terminal reports key releases (kitty keyboard protocol), only
//     while the key is held. Another key also ends it, as a release may not
//     come once that key leaves the mode where releases are asked for.
//   - Otherwise each press switches between the two.

const lineFlipKey = '\\'

func (a *App) flipLineNumbers() {
	if !a.keyReleases {
		a.lineFlip = !a.lineFlip
		return
	}
	a.lineFlip = true // a repeat while held changes nothing
}

// lineFlipOtherKey ends the flip when a key other than \ is pressed while
// the terminal reports key releases.
func (a *App) lineFlipOtherKey(msg tea.KeyPressMsg) {
	if a.keyReleases && msg.Code != lineFlipKey {
		a.lineFlip = false
	}
}

// lineFlipRelease ends the flip when \ is let go.
func (a *App) lineFlipRelease(msg tea.KeyReleaseMsg) {
	if msg.Code == lineFlipKey {
		a.lineFlip = false
	}
}

// relativeLineNumbers reports whether line numbers count from the cursor line.
func (a *App) relativeLineNumbers() bool {
	return (a.cfg.Editor.LineNumber == "relative") != a.lineFlip
}
