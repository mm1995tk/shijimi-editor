package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

var (
	ctrlD = Key{Code: 'd', Mod: ModCtrl}
	ctrlU = Key{Code: 'u', Mod: ModCtrl}
)

func TestHalfPageCentersCursorWithoutKeyReleases(t *testing.T) {
	a := newTestApp(t, strings.Repeat("x\n", 200))
	b := a.curText().buf
	keys(a, "1", "0", "0", "g")
	mid := a.layout().editH / 2
	for _, k := range []Key{ctrlD, ctrlD, ctrlU} {
		row := b.Cur.Row
		a.Update(k)
		a.render()
		if got := abs(b.Cur.Row - row); got != a.pageSize()/2 {
			t.Fatalf("%v moved %d lines, want %d", k, got, a.pageSize()/2)
		}
		if got := b.Cur.Row - b.ScrollRow; got != mid {
			t.Fatalf("%v: cursor at screen row %d, want %d", k, got, mid)
		}
	}
}

func TestHalfPageScrollsWhileHeld(t *testing.T) {
	a := newTestApp(t, strings.Repeat("x\n", 200))
	b := a.curText().buf
	keys(a, "1", "0", "0", "g")
	a.render()
	a.Update(tea.KeyboardEnhancementsMsg{Flags: ansi.KittyReportEventTypes})
	start, screenRow := b.Cur.Row, b.Cur.Row-b.ScrollRow
	tick := func() { a.Update(holdScrollMsg{a.holdSeq}); a.render() }

	a.Update(ctrlD)
	repeat := ctrlD
	repeat.Repeat = true
	for range 5 {
		a.Update(repeat) // key repeat must not speed it up
		tick()
	}
	if got := b.Cur.Row - start; got != 6 {
		t.Fatalf("held Ctrl+D moved %d lines, want 6", got)
	}
	if got := b.Cur.Row - b.ScrollRow; got != screenRow {
		t.Fatalf("cursor moved on screen: row %d, want %d", got, screenRow)
	}

	a.Update(tea.KeyReleaseMsg{Code: 'd'}) // Ctrl may be let go first
	row := b.Cur.Row
	tick()
	if b.Cur.Row != row {
		t.Fatalf("kept scrolling after release: row %d, want %d", b.Cur.Row, row)
	}

	a.Update(ctrlU)
	tick()
	keys(a, "j") // another key ends the scroll too
	row = b.Cur.Row
	tick()
	if b.Cur.Row != row {
		t.Fatalf("kept scrolling after another key: row %d, want %d", b.Cur.Row, row)
	}
}

func TestHalfPageStopsAtEndOfFile(t *testing.T) {
	a := newTestApp(t, "a\nb\nc\n")
	b := a.curText().buf
	a.Update(tea.KeyboardEnhancementsMsg{Flags: ansi.KittyReportEventTypes})
	a.Update(ctrlD)
	for range 10 {
		a.Update(holdScrollMsg{a.holdSeq})
	}
	if b.Cur.Row != len(b.Lines)-1 || a.hold.buf != nil {
		t.Fatalf("row %d, scrolling %v; want last row and stopped", b.Cur.Row, a.hold.buf != nil)
	}
}

// Some terminals drop IME text while key releases are reported, so they are
// asked for only where Ctrl+D / Ctrl+U scroll.
func TestKeyReleasesOnlyInNormalMode(t *testing.T) {
	a := newTestApp(t, "x\n")
	if !a.View().KeyboardEnhancements.ReportEventTypes {
		t.Fatal("normal mode should ask for key releases")
	}
	keys(a, "i")
	if a.View().KeyboardEnhancements.ReportEventTypes {
		t.Fatal("insert mode should not ask for key releases")
	}
	keys(a, "esc", ":")
	if a.View().KeyboardEnhancements.ReportEventTypes {
		t.Fatal("the command line should not ask for key releases")
	}
}

// Closing the command line with :q would ask for key releases again, which
// makes Bubble Tea query the terminal on the way out; the reply could then
// reach the shell as typed text. So the request stays as it was.
func TestKeyReleasesFrozenWhileQuitting(t *testing.T) {
	a := newTestApp(t, "x\n")
	keys(a, ":")
	if a.View().KeyboardEnhancements.ReportEventTypes {
		t.Fatal("the command line should not ask for key releases")
	}
	keys(a, "q", "enter")
	if !a.quitting {
		t.Fatal(":q should be quitting")
	}
	if a.View().KeyboardEnhancements.ReportEventTypes {
		t.Fatal("quitting must not change the key-release request")
	}
}

var (
	ctrlAltD = Key{Code: 'd', Mod: ModCtrl | ModAlt}
	ctrlAltU = Key{Code: 'u', Mod: ModCtrl | ModAlt}
)

func TestScrollKeepCursorHalfPage(t *testing.T) {
	a := newTestApp(t, strings.Repeat("x\n", 200))
	b := a.curText().buf
	keys(a, "1", "0", "0", "g")
	a.render()
	// Put the cursor near the bottom of the screen (below the margin) so a
	// half page leaves it where it is.
	b.ScrollRow = b.Cur.Row - (a.layout().editH - 1 - 3)
	row, scroll := b.Cur.Row, b.ScrollRow
	a.Update(ctrlAltD)
	a.render()
	if b.Cur.Row != row || b.ScrollRow != scroll+a.pageSize()/2 {
		t.Fatalf("cursor row %d (was %d), scroll %d (was %d)", b.Cur.Row, row, b.ScrollRow, scroll)
	}
	// Keep going: the cursor is pushed along the top edge.
	a.Update(ctrlAltD)
	a.Update(ctrlAltD)
	a.render()
	if b.Cur.Row-b.ScrollRow < 0 || b.Cur.Row-b.ScrollRow >= a.layout().editH {
		t.Fatalf("cursor off screen: row %d, scroll %d", b.Cur.Row, b.ScrollRow)
	}
	a.Update(ctrlAltU)
	a.render()
	if b.Cur.Row-b.ScrollRow >= a.layout().editH {
		t.Fatalf("cursor off screen: row %d, scroll %d", b.Cur.Row, b.ScrollRow)
	}
}

func TestScrollKeepCursorWhileHeld(t *testing.T) {
	a := newTestApp(t, strings.Repeat("x\n", 200))
	b := a.curText().buf
	keys(a, "1", "0", "0", "g")
	a.render()
	b.ScrollRow = b.Cur.Row - a.layout().editH/2
	a.Update(tea.KeyboardEnhancementsMsg{Flags: ansi.KittyReportEventTypes})
	row, scroll := b.Cur.Row, b.ScrollRow
	tick := func() { a.Update(holdScrollMsg{a.holdSeq}); a.render() }

	a.Update(ctrlAltD)
	a.render()
	repeat := ctrlAltD
	repeat.Repeat = true
	for range 3 {
		a.Update(repeat)
		tick()
	}
	if b.Cur.Row != row || b.ScrollRow != scroll+4 {
		t.Fatalf("cursor row %d (want %d), scroll %d (want %d)", b.Cur.Row, row, b.ScrollRow, scroll+4)
	}
	// Past the margin the cursor is pushed along the top edge.
	for range 20 {
		tick()
	}
	if got := b.Cur.Row - b.ScrollRow; got != 3 {
		t.Fatalf("cursor at screen row %d, want 3", got)
	}
	a.Update(tea.KeyReleaseMsg{Code: 'd'})
	scroll = b.ScrollRow
	tick()
	if b.ScrollRow != scroll {
		t.Fatal("kept scrolling after release")
	}
	// Ctrl+D while Ctrl+Alt+D is held is a different scroll.
	a.Update(ctrlAltU)
	if !a.hold.keep {
		t.Fatal("expected keep-cursor scroll")
	}
	a.Update(ctrlD)
	if a.hold.keep {
		t.Fatal("Ctrl+D must replace the keep-cursor scroll")
	}
}

func TestScrollKeepCursorStopsAtEnds(t *testing.T) {
	a := newTestApp(t, strings.Repeat("x\n", 200))
	b := a.curText().buf
	a.render()
	a.Update(tea.KeyboardEnhancementsMsg{Flags: ansi.KittyReportEventTypes})
	a.Update(ctrlAltD)
	for range 400 {
		a.Update(holdScrollMsg{a.holdSeq})
	}
	if a.hold.buf != nil || b.ScrollRow != len(b.Lines)-a.layout().editH {
		t.Fatalf("scroll %d, scrolling %v; want last line at the bottom and stopped", b.ScrollRow, a.hold.buf != nil)
	}
	a.Update(ctrlAltU)
	for range 400 {
		a.Update(holdScrollMsg{a.holdSeq})
	}
	if a.hold.buf != nil || b.ScrollRow != 0 {
		t.Fatalf("scroll %d; want 0 and stopped", b.ScrollRow)
	}
}
