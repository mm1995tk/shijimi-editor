package tui

import (
	"os"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestScheduleAutoSave(t *testing.T) {
	a := newTestApp(t, "one\n")
	a.cfg.Editor.AutoSaveDelay = time.Millisecond
	tb := a.CurText()

	// no edit: nothing is scheduled
	if _, cmd := a.Update(Key{Code: KeyEscape}); cmd != nil && tb.autoVer != 0 {
		t.Fatal("scheduled without an edit")
	}

	// an edit schedules a timer; running it yields the save message
	keys(a, "i")
	_, cmd := a.Update(textKey("x"))
	if cmd == nil || tb.autoVer != tb.Buf.Version {
		t.Fatalf("edit did not schedule auto-save (autoVer=%d, version=%d)", tb.autoVer, tb.Buf.Version)
	}
	var msg AutoSaveDue
	found := false
	for _, m := range collect(cmd) {
		if am, ok := m.(AutoSaveDue); ok {
			msg, found = am, true
		}
	}
	if !found {
		t.Fatal("no AutoSaveDue produced")
	}

	// same Version: not scheduled twice
	if _, cmd := a.Update(tea.WindowSizeMsg{Width: 100, Height: 30}); len(collect(cmd)) != 0 {
		t.Fatal("timer re-registered for an unchanged version")
	}

	a.Update(msg)
	if b, _ := os.ReadFile(tb.Buf.Path); string(b) != "xone\n" {
		t.Fatalf("not saved: %q", b)
	}
}

func TestAutoSaveDisabled(t *testing.T) {
	a := newTestApp(t, "one\n")
	a.cfg.Editor.AutoSave = false
	keys(a, "i")
	_, cmd := a.Update(textKey("x"))
	for _, m := range collect(cmd) {
		if _, ok := m.(AutoSaveDue); ok {
			t.Fatal("scheduled while disabled")
		}
	}
}
