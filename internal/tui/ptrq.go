package tui

// ptrq.go is what the pointer does to the state, once mouse.go has found out
// where it is: the wheel and the clicks in the side bar and in the editor, the
// hover popup and the review card it opens. mouse.go keeps the cells (the
// layout, the regions); these take rows and columns of a region.

// wheelSide scrolls the side bar by wheel rows: the file tree, or the changed
// files below it in the diff mode, which row (of the screen's body) is over.
// bodyH is the height of the body.
func (a *Core) wheelSide(row, bodyH, wheel int) {
	e := a.explorer
	if a.gd.on && row >= a.explorerH() {
		g := &a.gd
		body := bodyH - a.explorerH() - 1
		g.scroll = clamp(g.scroll+wheel, 0, max(0, len(g.vis())-body))
		g.keepSelVisible(body)
	} else {
		eh := a.explorerH()
		e.scroll = clamp(e.scroll+wheel, 0, max(0, len(e.flat)-(eh-1)))
		e.sel = clamp(e.sel, e.scroll, e.scroll+eh-2)
	}
}

// wheelEditor scrolls the current text tab by wheel rows.
func (a *Core) wheelEditor(wheel int) {
	if t := a.cur(); t != nil {
		switch t.kind {
		case tabText:
			if a.fullPreview(t) {
				t.buf.MoveLines(wheel, false) // the preview scrolls with the cursor
				break
			}
			t.buf.ScrollRow = clamp(t.buf.ScrollRow+wheel, 0, max(0, len(t.buf.Lines)-1))
		}
	}
}

// clickSide is a left press on row sy of the side bar (double: the second
// press of a double click).
func (a *Core) clickSide(sy int, double bool) []Effect {
	if a.gd.on && sy >= a.explorerH() {
		g := &a.gd
		if vis, i := g.vis(), sy-1-a.explorerH()+g.scroll; sy > a.explorerH() && i < len(vis) {
			r := vis[i]
			g.selectRow(r)
			if g.rows[r].file < 0 {
				g.setFolder(r, g.closed[g.rows[r].path])
				return a.setFocus(focusChanges)
			}
			return a.openChange(g.files[g.sel])
		}
		return a.setFocus(focusChanges)
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
			return a.setFocus(focusEditor)
		}
	}
	return a.setFocus(focusExplorer)
}

// hoverByPointer reports whether the open hover popup was opened by the
// pointer (it hangs from the character under it, not from the cursor).
func (a *Core) hoverByPointer() bool { return a.hoverAt != nil }

// closeHover closes the hover popup.
func (a *Core) closeHover() { a.hoverText, a.hoverAt = "", nil }

// pointerMoved ends the pointer's hover: a pending one is stale, the word it
// belonged to and the cell it rested on are forgotten, and an open popup closes.
func (a *Core) pointerMoved() {
	a.hoverSeq++
	a.hoverWord = wordSpan{}
	a.hoverCell = screenPt{-1, -1}
	a.closeHover()
}

// scrollCard scrolls the open review card by d rows.
func (a *Core) scrollCard(d int) { a.card.scroll = max(0, a.card.scroll+d) }

// closeCard closes the review card.
func (a *Core) closeCard() { a.card = nil }

// showCard opens the review thread th of the file path as a card anchored at
// the screen cell (x, y); byPointer says the pointer's hover opened it.
func (a *Core) showCard(th *prThread, path string, x, y int, byPointer bool) {
	a.card = &prCard{thread: th, path: a.repoPath(path), byPointer: byPointer, at: &screenPt{x, y}}
}

// cardOnThread reports whether the open card is on the thread th.
func (a *Core) cardOnThread(th *prThread) bool { return a.card != nil && a.card.thread == th }

// pointerCard reports whether the open card was opened by the pointer's hover.
func (a *Core) pointerCard() bool { return a.card != nil && a.card.byPointer }

// restedOn reports whether the pointer rested on the cell pt last time, and
// remembers pt as the cell it rests on.
func (a *Core) restedOn(pt screenPt) bool {
	if pt == a.hoverCell {
		return true
	}
	a.hoverCell = pt
	return false
}

// hoverCellPos is the cell the pointer last rested on.
func (a *Core) hoverCellPos() screenPt { return a.hoverCell }

// pointerRests is the pointer resting on a cell of the editor whose word is
// word (ok: there is one). inBox says it is on the open hover popup. It
// starts the dwell timer for the word, or leaves a popup that the pointer is
// reading or whose word it is still on.
func (a *Core) pointerRests(word wordSpan, ok, inBox bool) []Effect {
	if a.hoverText != "" && inBox {
		return nil // reading it (the wheel scrolls it); whatever hides below is not asked about
	}
	if a.hoverText != "" && a.hoverAt != nil {
		// A pointer popup also stays while the pointer is on the word it describes.
		if ok && word == a.hoverWord {
			return nil
		}
		a.closeHover()
	} else if ok && word == a.hoverWord {
		return nil // the timer for this word is already running
	}
	a.hoverSeq++
	a.hoverWord = wordSpan{}
	if !ok {
		return nil
	}
	a.hoverWord = word
	seq := a.hoverSeq
	return []Effect{StartTimer{After: hoverDelay, Fire: HoverDwell{seq}}}
}

// closePalette closes the palette.
func (a *Core) closePalette() { a.pal.close() }

// searchScroll moves the project search's selection by d rows.
func (a *Core) searchScroll(d int) { a.search.move(d) }

// searchClick is a left press on the screen cell (x, y) with the project
// search open: outside its box it closes.
func (a *Core) searchClick(x, y int) {
	s := a.search
	r := s.rect
	if x < r[0] || x >= r[0]+r[2] || y < r[1] || y >= r[1]+r[3] {
		s.close()
	}
}
