package core

import (
	"os"
	"testing"
)

func TestAutoSave(t *testing.T) {
	a := newTestApp(t, "one\n")
	keys(a, "i", "x")
	tb := a.CurText()
	ver := tb.Buf.Version
	// a stale timer (buffer edited again afterwards) must not save
	keys(a, "y")
	a.Update(AutoSaveDue{tab: tb, ver: ver})
	if b, _ := os.ReadFile(tb.Buf.Path); string(b) != "one\n" {
		t.Fatalf("stale timer saved: %q", b)
	}
	a.Update(AutoSaveDue{tab: tb, ver: tb.Buf.Version})
	if b, _ := os.ReadFile(tb.Buf.Path); string(b) != "xyone\n" || tb.Buf.Dirty {
		t.Fatalf("not saved: %q", b)
	}
}

func TestAutoSaveSkipsUnnamed(t *testing.T) {
	a := newTestApp(t, "one\n")
	a.newFile()
	keys(a, "i", "x")
	tb := a.CurText()
	a.Update(AutoSaveDue{tab: tb, ver: tb.Buf.Version})
	if !tb.Buf.Dirty || tb.Buf.Path != "" {
		t.Fatal("unnamed buffer must not be auto-saved")
	}
}
