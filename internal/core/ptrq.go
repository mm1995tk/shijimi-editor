package core

// ptrq.go is what the pointer does to the state, once mouse.go has found out
// where it is: the wheel and the clicks in the side bar and in the editor, the
// hover popup and the review card it opens. mouse.go keeps the cells (the
// layout, the regions); these take rows and columns of a region.

// wheelSide scrolls the side bar by wheel rows: the file tree, or the changed
// files below it in the diff mode, which row (of the screen's body) is over.
// bodyH is the height of the body.
func (a *Core) WheelSide(row, bodyH, wheel int) {
	e := a.explorer
	if a.gd.on && row >= a.ExplorerH() {
		g := &a.gd
		body := bodyH - a.ExplorerH() - 1
		g.scroll = Clamp(g.scroll+wheel, 0, max(0, len(g.vis())-body))
		g.keepSelVisible(body)
	} else {
		eh := a.ExplorerH()
		e.scroll = Clamp(e.scroll+wheel, 0, max(0, len(e.flat)-(eh-1)))
		e.sel = Clamp(e.sel, e.scroll, e.scroll+eh-2)
	}
}

// wheelEditor scrolls the current text tab by wheel rows.
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

// clickSide is a left press on row sy of the side bar (double: the second
// press of a double click).
func (a *Core) ClickSide(sy int, double bool) []Effect {
	if a.gd.on && sy >= a.ExplorerH() {
		g := &a.gd
		if vis, i := g.vis(), sy-1-a.ExplorerH()+g.scroll; sy > a.ExplorerH() && i < len(vis) {
			r := vis[i]
			g.selectRow(r)
			if g.rows[r].file < 0 {
				g.setFolder(r, g.closed[g.rows[r].path])
				return a.SetFocus(FocusChanges)
			}
			return a.openChange(g.files[g.sel])
		}
		return a.SetFocus(FocusChanges)
	}
	row := sy - 1 + a.explorer.scroll
	if sy >= 1 && row < len(a.explorer.flat) {
		a.explorer.sel = row
		n := a.explorer.flat[row]
		switch {
		case n.isDir:
			a.explorer.toggle(n)
		case !double:
			a.openTransient(n.path)
		case a.openFile(n.path):
			return a.SetFocus(FocusEditor)
		}
	}
	return a.SetFocus(FocusExplorer)
}

// hoverByPointer reports whether the open hover popup was opened by the
// pointer (it hangs from the character under it, not from the cursor).
func (a *Core) HoverByPointer() bool { return a.hoverAt != nil }

// closeHover closes the hover popup.
func (a *Core) CloseHover() { a.hoverText, a.hoverAt = "", nil }

// pointerMoved ends the pointer's hover: a pending one is stale, the word it
// belonged to and the cell it rested on are forgotten, and an open popup closes.
func (a *Core) PointerMoved() {
	a.hoverSeq++
	a.hoverWord = WordSpan{}
	a.hoverCell = ScreenPt{-1, -1}
	a.CloseHover()
}

// scrollCard scrolls the open review card by d rows.
func (a *Core) ScrollCard(d int) { a.card.scroll = max(0, a.card.scroll+d) }

// closeCard closes the review card.
func (a *Core) CloseCard() { a.card = nil }

// showCard opens the review thread th of the file path as a card anchored at
// the screen cell (x, y); byPointer says the pointer's hover opened it.
func (a *Core) ShowCard(th *PrThread, path string, x, y int, byPointer bool) {
	a.card = &prCard{Thread: th, Path: a.RepoPath(path), byPointer: byPointer, At: &ScreenPt{x, y}}
}

// cardOnThread reports whether the open card is on the thread th.
func (a *Core) CardOnThread(th *PrThread) bool { return a.card != nil && a.card.Thread == th }

// pointerCard reports whether the open card was opened by the pointer's hover.
func (a *Core) PointerCard() bool { return a.card != nil && a.card.byPointer }

// restedOn reports whether the pointer rested on the cell pt last time, and
// remembers pt as the cell it rests on.
func (a *Core) RestedOn(pt ScreenPt) bool {
	if pt == a.hoverCell {
		return true
	}
	a.hoverCell = pt
	return false
}

// hoverCellPos is the cell the pointer last rested on.
func (a *Core) HoverCellPos() ScreenPt { return a.hoverCell }

// pointerRests is the pointer resting on a cell of the editor whose word is
// word (ok: there is one). inBox says it is on the open hover popup. It
// starts the dwell timer for the word, or leaves a popup that the pointer is
// reading or whose word it is still on.
func (a *Core) PointerRests(word WordSpan, ok, inBox bool) []Effect {
	if a.hoverText != "" && inBox {
		return nil // reading it (the wheel scrolls it); whatever hides below is not asked about
	}
	if a.hoverText != "" && a.hoverAt != nil {
		// A pointer popup also stays while the pointer is on the word it describes.
		if ok && word == a.hoverWord {
			return nil
		}
		a.CloseHover()
	} else if ok && word == a.hoverWord {
		return nil // the timer for this word is already running
	}
	a.hoverSeq++
	a.hoverWord = WordSpan{}
	if !ok {
		return nil
	}
	a.hoverWord = word
	seq := a.hoverSeq
	return []Effect{StartTimer{After: hoverDelay, Fire: HoverDwell{seq}}}
}

// closePalette closes the palette.
func (a *Core) ClosePalette() { a.pal.close() }

// searchScroll moves the project search's selection by d rows.
func (a *Core) SearchScroll(d int) { a.search.move(d) }

// searchClick is a left press on the screen cell (x, y) with the project
// search open: outside its box it closes.
func (a *Core) SearchClick(x, y int) {
	s := a.search
	r := s.rect
	if x < r[0] || x >= r[0]+r[2] || y < r[1] || y >= r[1]+r[3] {
		s.close()
	}
}
