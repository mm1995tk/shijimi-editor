package core

import "testing"

func TestTypingWithoutLanguageServerIsSilent(t *testing.T) {
	a := newTestApp(t, "")
	a.Focus = FocusEditor
	keys(a, "i", "a", "b", "c")
	if got := a.CurText().Buf.Text(); got != "abc" {
		t.Fatalf("text = %q, want %q", got, "abc")
	}
	if a.MsgErr {
		t.Fatalf("typing reported an error: %q", a.Msg)
	}
}
