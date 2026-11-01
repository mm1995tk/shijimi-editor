package core

// \ shows the line numbers the other way (absolute or relative, the opposite
// of editor.line-number), so both are at hand without two columns:
//   - When the terminal reports key releases (kitty keyboard protocol), only
//     while the key is held. Another key also ends it, as a release may not
//     come once that key leaves the mode where releases are asked for.
//   - Otherwise each press switches between the two.

const lineFlipKey = '\\'

func (a *Core) flipLineNumbers() {
	if !a.ReleasesReported() {
		a.LineFlip = !a.LineFlip
		return
	}
	a.LineFlip = true // a repeat while held changes nothing
}

// lineFlipOtherKey ends the flip when a key other than \ is pressed while
// the terminal reports key releases.
func (a *Core) lineFlipOtherKey(msg Key) {
	if a.ReleasesReported() && msg.Code != lineFlipKey {
		a.LineFlip = false
	}
}

// lineFlipRelease ends the flip when \ is let go.
func (a *Core) lineFlipRelease(msg Key) {
	if msg.Code == lineFlipKey {
		a.LineFlip = false
	}
}

// relativeLineNumbers reports whether line numbers count from the cursor line.
func (a *Core) relativeLineNumbers() bool {
	return (a.Cfg.Editor.LineNumber == "relative") != a.LineFlip
}
