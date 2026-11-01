package core

import (
	"testing"
)

func TestUrlAt(t *testing.T) {
	line := []rune("see (https://example.com/a?b=1). ok")
	for col, want := range map[int]string{5: "https://example.com/a?b=1", 30: "", 0: ""} {
		got, ok := urlAt(line, col)
		if got != want || ok != (want != "") {
			t.Errorf("col %d: got %q,%v want %q", col, got, ok, want)
		}
	}
}

func TestOpenExternalMissingCommand(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := OpenExternal("https://example.com"); err == nil {
		t.Fatal("expected error when opener is missing")
	}
}
