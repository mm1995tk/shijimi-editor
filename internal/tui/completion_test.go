package tui

import "testing"

func TestTypingWithoutLanguageServerIsSilent(t *testing.T) {
	a := newTestApp(t, "")
	a.focus = focusEditor
	keys(a, "i", "a", "b", "c")
	if got := a.curText().buf.Text(); got != "abc" {
		t.Fatalf("text = %q, want %q", got, "abc")
	}
	if a.msgErr {
		t.Fatalf("typing reported an error: %q", a.msg)
	}
}
