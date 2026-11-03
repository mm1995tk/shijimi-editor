package core

import (
	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"os"
	"path/filepath"
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
	a.addTab(NewTab(buffer.New()))
	keys(a, "i", "x")
	tb := a.CurText()
	a.Update(AutoSaveDue{tab: tb, ver: tb.Buf.Version})
	if !tb.Buf.Dirty || tb.Buf.Path != "" {
		t.Fatal("unnamed buffer must not be auto-saved")
	}
}

func TestNewRequiresName(t *testing.T) {
	a := newTestApp(t, "one\n")
	n := len(a.Tabs)
	a.ExecCommand("new")
	if len(a.Tabs) != n || !a.MsgErr {
		t.Fatalf(":new without a name must fail: tabs=%d err=%v", len(a.Tabs), a.MsgErr)
	}
	a.ExecCommand("new sub/foo.go")
	tb := a.CurText()
	want := filepath.Join(a.Root, "sub", "foo.go")
	if len(a.Tabs) != n+1 || tb.Buf.Path != want {
		t.Fatalf("path = %q, want %q", tb.Buf.Path, want)
	}
	keys(a, "i", "x")
	a.Update(AutoSaveDue{tab: tb, ver: tb.Buf.Version})
	if b, _ := os.ReadFile(want); string(b) != "x" {
		t.Fatalf("not saved: %q", b)
	}
}
