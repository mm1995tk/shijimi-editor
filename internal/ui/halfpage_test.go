package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

var (
	ctrlD = tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}
	ctrlU = tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}
)

func TestHalfPageCentersCursorWithoutKeyReleases(t *testing.T) {
	a := newTestApp(t, strings.Repeat("x\n", 200))
	b := a.curText().buf
	keys(a, "1", "0", "0", "g")
	mid := a.layout().editH / 2
	for _, k := range []tea.KeyPressMsg{ctrlD, ctrlD, ctrlU} {
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
	repeat.IsRepeat = true
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
