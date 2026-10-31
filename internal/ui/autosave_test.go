package ui

import (
	"os"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestAutoSave(t *testing.T) {
	a := newTestApp(t, "one\n")
	keys(a, "i", "x")
	tb := a.curText()
	ver := tb.buf.Version
	// a stale timer (buffer edited again afterwards) must not save
	keys(a, "y")
	a.Update(AutoSaveDue{tab: tb, ver: ver})
	if b, _ := os.ReadFile(tb.buf.Path); string(b) != "one\n" {
		t.Fatalf("stale timer saved: %q", b)
	}
	a.Update(AutoSaveDue{tab: tb, ver: tb.buf.Version})
	if b, _ := os.ReadFile(tb.buf.Path); string(b) != "xyone\n" || tb.buf.Dirty {
		t.Fatalf("not saved: %q", b)
	}
}

func TestAutoSaveSkipsUnnamed(t *testing.T) {
	a := newTestApp(t, "one\n")
	a.newFile()
	keys(a, "i", "x")
	tb := a.curText()
	a.Update(AutoSaveDue{tab: tb, ver: tb.buf.Version})
	if !tb.buf.Dirty || tb.buf.Path != "" {
		t.Fatal("unnamed buffer must not be auto-saved")
	}
}

func TestScheduleAutoSave(t *testing.T) {
	a := newTestApp(t, "one\n")
	a.cfg.Editor.AutoSaveDelay = time.Millisecond
	tb := a.curText()

	// no edit: nothing is scheduled
	if _, cmd := a.Update(Key{Code: KeyEscape}); cmd != nil && tb.autoVer != 0 {
		t.Fatal("scheduled without an edit")
	}

	// an edit schedules a timer; running it yields the save message
	keys(a, "i")
	_, cmd := a.Update(textKey("x"))
	if cmd == nil || tb.autoVer != tb.buf.Version {
		t.Fatalf("edit did not schedule auto-save (autoVer=%d, version=%d)", tb.autoVer, tb.buf.Version)
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
	if b, _ := os.ReadFile(tb.buf.Path); string(b) != "xone\n" {
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

// collect runs cmd (flattening batches) and returns the messages produced.
func collect(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if b, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range b {
			out = append(out, collect(c)...)
		}
		return out
	}
	if msg == nil {
		return nil
	}
	return []tea.Msg{msg}
}
