package caret

import (
	"bytes"
	"testing"
)

type fakeFile struct{ bytes.Buffer }

func (*fakeFile) Close() error { return nil }
func (*fakeFile) Fd() uintptr  { return 0 }

func TestWriteKeepsCursorHidden(t *testing.T) {
	f := &fakeFile{}
	w := NewWriter(f)
	// bubbletea's first frame only shows the cursor; it never hides it.
	frame := "frame\x1b[3;5H\x1b[?25h"
	if n, err := w.Write([]byte(frame)); n != len(frame) || err != nil {
		t.Fatalf("Write = %d, %v", n, err)
	}
	if got, want := f.String(), "frame\x1b[3;5H\x1b[?25l"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
	if err := w.ShowCursor(); err != nil {
		t.Fatal(err)
	}
	if got, want := f.String(), "frame\x1b[3;5H\x1b[?25l\x1b[?25h"; got != want {
		t.Errorf("after ShowCursor = %q, want %q", got, want)
	}
}

func TestSendWritesAsIs(t *testing.T) {
	f := &fakeFile{}
	w := NewWriter(f)
	if err := w.Send([]byte("\x1b]52;c;eA==\x07")); err != nil {
		t.Fatal(err)
	}
	if got, want := f.String(), "\x1b]52;c;eA==\x07"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}
