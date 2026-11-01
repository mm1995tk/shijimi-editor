package core

// ptrq.go is what the pointer does to the state, once mouse.go has found out
// where it is: the wheel and the clicks in the side bar and in the editor, the
// hover popup and the review card it opens. mouse.go keeps the cells (the
// layout, the regions); these take rows and columns of a region.

// WheelSide scrolls the side bar by wheel rows: the file tree, or the changed
// files below it in the diff mode, which row (of the screen's body) is over.
// bodyH is the height of the body.
func (a *Core) WheelSide(row, bodyH, wheel int) {
	e := a.Explorer
	if a.Gd.On && row >= a.ExplorerH() {
		g := &a.Gd
		body := bodyH - a.ExplorerH() - 1
		g.scroll = Clamp(g.scroll+wheel, 0, max(0, len(g.vis())-body))
		g.keepSelVisible(body)
	} else {
		eh := a.ExplorerH()
		e.Scroll = Clamp(e.Scroll+wheel, 0, max(0, len(e.Flat)-(eh-1)))
		e.Sel = Clamp(e.Sel, e.Scroll, e.Scroll+eh-2)
	}
}

// WheelEditor scrolls the current text tab by wheel rows.
func (a *Core) WheelEditor(wheel int) {
	if t := a.Cur(); t != nil {
		switch t.Kind {
		case TabText:
			if a.FullPreview(t) {
				t.Buf.MoveLines(wheel, false) // the preview scrolls with the cursor
				break
			}
			t.Buf.ScrollRow = Clamp(t.Buf.ScrollRow+wheel, 0, max(0, len(t.Buf.Lines)-1))
		}
	}
}

// ClickSide is a left press on row sy of the side bar (double: the second
// press of a double click).
func (a *Core) ClickSide(sy int, double bool) []Effect {
	if a.Gd.On && sy >= a.ExplorerH() {
		g := &a.Gd
		if vis, i := g.vis(), sy-1-a.ExplorerH()+g.scroll; sy > a.ExplorerH() && i < len(vis) {
			r := vis[i]
			g.selectRow(r)
			if g.Rows[r].file < 0 {
				g.setFolder(r, g.closed[g.Rows[r].path])
				return a.SetFocus(FocusChanges)
			}
			return a.OpenChange(g.Files[g.Sel])
		}
		return a.SetFocus(FocusChanges)
	}
	row := sy - 1 + a.Explorer.Scroll
	if sy >= 1 && row < len(a.Explorer.Flat) {
		a.Explorer.Sel = row
		n := a.Explorer.Flat[row]
		switch {
		case n.isDir:
			a.Explorer.Toggle(n)
		case !double:
			a.openTransient(n.Path)
		case a.OpenFile(n.Path):
			return a.SetFocus(FocusEditor)
		}
	}
	return a.SetFocus(FocusExplorer)
}

// HoverByPointer reports whether the open hover popup was opened by the
// pointer (it hangs from the character under it, not from the cursor).
func (a *Core) HoverByPointer() bool { return a.HoverAt != nil }

// CloseHover closes the hover popup.
func (a *Core) CloseHover() { a.HoverText, a.HoverAt = "", nil }

// PointerMoved ends the pointer's hover: a pending one is stale, the word it
// belonged to and the cell it rested on are forgotten, and an open popup closes.
func (a *Core) PointerMoved() {
	a.HoverSeq++
	a.HoverWord = WordSpan{}
	a.HoverCell = ScreenPt{-1, -1}
	a.CloseHover()
}

// ScrollCard scrolls the open review card by d rows.
func (a *Core) ScrollCard(d int) { a.Card.scroll = max(0, a.Card.scroll+d) }

// CloseCard closes the review card.
func (a *Core) CloseCard() { a.Card = nil }

// ShowCard opens the review thread th of the file path as a card anchored at
// the screen cell (x, y); byPointer says the pointer's hover opened it.
func (a *Core) ShowCard(th *PrThread, path string, x, y int, byPointer bool) {
	a.Card = &PrCard{Thread: th, Path: a.RepoPath(path), ByPointer: byPointer, At: &ScreenPt{x, y}}
}

// CardOnThread reports whether the open card is on the thread th.
func (a *Core) CardOnThread(th *PrThread) bool { return a.Card != nil && a.Card.Thread == th }

// PointerCard reports whether the open card was opened by the pointer's hover.
func (a *Core) PointerCard() bool { return a.Card != nil && a.Card.ByPointer }

// RestedOn reports whether the pointer rested on the cell pt last time, and
// remembers pt as the cell it rests on.
func (a *Core) RestedOn(pt ScreenPt) bool {
	if pt == a.HoverCell {
		return true
	}
	a.HoverCell = pt
	return false
}

// HoverCellPos is the cell the pointer last rested on.
func (a *Core) HoverCellPos() ScreenPt { return a.HoverCell }

// PointerRests is the pointer resting on a cell of the editor whose word is
// word (ok: there is one). inBox says it is on the open hover popup. It
// starts the dwell timer for the word, or leaves a popup that the pointer is
// reading or whose word it is still on.
func (a *Core) PointerRests(word WordSpan, ok, inBox bool) []Effect {
	if a.HoverText != "" && inBox {
		return nil // reading it (the wheel scrolls it); whatever hides below is not asked about
	}
	if a.HoverText != "" && a.HoverAt != nil {
		// A pointer popup also stays while the pointer is on the word it describes.
		if ok && word == a.HoverWord {
			return nil
		}
		a.CloseHover()
	} else if ok && word == a.HoverWord {
		return nil // the timer for this word is already running
	}
	a.HoverSeq++
	a.HoverWord = WordSpan{}
	if !ok {
		return nil
	}
	a.HoverWord = word
	seq := a.HoverSeq
	return []Effect{StartTimer{After: hoverDelay, Fire: HoverDwell{seq}}}
}

// ClosePalette closes the palette.
func (a *Core) ClosePalette() { a.Pal.Close() }

// SearchScroll moves the project search's selection by d rows.
func (a *Core) SearchScroll(d int) { a.Search.move(d) }

// SearchClick is a left press on the screen cell (x, y) with the project
// search open: outside its box it closes.
func (a *Core) SearchClick(x, y int) {
	s := a.Search
	r := s.rect
	if x < r[0] || x >= r[0]+r[2] || y < r[1] || y >= r[1]+r[3] {
		s.close()
	}
}
