package tui

import (
	"os"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/core"
)

func TestScheduleAutoSave(t *testing.T) {
	a := newTestApp(t, "one\n")
	a.Cfg.Editor.AutoSaveDelay = time.Millisecond
	tb := a.CurText()

	// no edit: nothing is scheduled
	if _, cmd := a.Update(core.Key{Code: KeyEscape}); cmd != nil && tb.AutoVer != 0 {
		t.Fatal("scheduled without an edit")
	}

	// an edit schedules a timer; running it yields the save message
	keys(a, "i")
	_, cmd := a.Update(textKey("x"))
	if cmd == nil || tb.AutoVer != tb.Buf.Version {
		t.Fatalf("edit did not schedule auto-save (autoVer=%d, version=%d)", tb.AutoVer, tb.Buf.Version)
	}
	var msg core.AutoSaveDue
	found := false
	for _, m := range collect(cmd) {
		if am, ok := m.(core.AutoSaveDue); ok {
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
	a.Cfg.Editor.AutoSave = false
	keys(a, "i")
	_, cmd := a.Update(textKey("x"))
	for _, m := range collect(cmd) {
		if _, ok := m.(core.AutoSaveDue); ok {
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
