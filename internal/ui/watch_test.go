package ui

import (
	"os"
	"testing"
	"time"
)

func touchLater(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Minute)
	os.Chtimes(path, future, future)
}

func TestWatchReloadsCleanBuffer(t *testing.T) {
	a := newTestApp(t, "one\n")
	tb := a.curText()
	touchLater(t, tb.buf.Path, "agent\n")
	a.Update(watchMsg{})
	if got := tb.buf.Text(); got != "agent\n" || tb.dirty() {
		t.Fatalf("not reloaded: %q dirty=%v", got, tb.dirty())
	}
	if a.msgErr || a.msg == "" {
		t.Fatalf("no info message: %q", a.msg)
	}
}

func TestWatchKeepsUnsavedEdits(t *testing.T) {
	a := newTestApp(t, "one\n")
	tb := a.curText()
	keys(a, "i", "x")
	touchLater(t, tb.buf.Path, "agent\n")
	a.Update(watchMsg{})
	if got := tb.buf.Text(); got != "xone\n" || !tb.conflict || !a.msgErr {
		t.Fatalf("edits lost or no warning: %q conflict=%v", got, tb.conflict)
	}
	// auto-save must not overwrite the other process's change
	a.Update(autoSaveMsg{tab: tb, ver: tb.buf.Version})
	if b, _ := os.ReadFile(tb.buf.Path); string(b) != "agent\n" {
		t.Fatalf("auto-save overwrote disk: %q", b)
	}
	// :reload takes the disk version
	a.execCommand("reload")
	if got := tb.buf.Text(); got != "agent\n" || tb.buf.DiskChanged() {
		t.Fatalf("reload: %q", got)
	}
	a.Update(watchMsg{})
	if tb.conflict {
		t.Fatal("conflict not cleared")
	}
}

func TestWatchDisabled(t *testing.T) {
	a := newTestApp(t, "one\n")
	a.cfg.Editor.WatchFiles = false
	tb := a.curText()
	touchLater(t, tb.buf.Path, "agent\n")
	a.Update(watchMsg{})
	if tb.buf.Text() != "one\n" {
		t.Fatal("reloaded while disabled")
	}
}
