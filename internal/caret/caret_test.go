package caret

import (
	"bytes"
	"testing"
)

type fakeFile struct{ bytes.Buffer }

func (*fakeFile) Close() error { return nil }
func (*fakeFile) Fd() uintptr  { return 0 }

func TestWriteParksCursor(t *testing.T) {
	f := &fakeFile{}
	w := NewWriter(f)

	if n, err := w.Write([]byte("frame1")); n != 6 || err != nil {
		t.Fatalf("Write = %d, %v", n, err)
	}
	w.Set(4, 2)
	if n, err := w.Write([]byte("frame2")); n != 6 || err != nil {
		t.Fatalf("Write = %d, %v", n, err)
	}
	w.Clear()
	_, _ = w.Write([]byte("frame3"))
	w.Set(-1, 3)
	_, _ = w.Write([]byte("frame4"))

	if got, want := f.String(), "frame1frame2\x1b[3;5Hframe3frame4"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestSendGoesBetweenWrites(t *testing.T) {
	f := &fakeFile{}
	w := NewWriter(f)
	w.Set(0, 0)
	_, _ = w.Write([]byte("frame1"))
	if err := w.Send([]byte("\x1b]52;c;eA==\x07")); err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("frame2"))
	want := "frame1\x1b[1;1H\x1b]52;c;eA==\x07frame2\x1b[1;1H"
	if got := f.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}
