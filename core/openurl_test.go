package core

import (
	"testing"
)

func TestOpenExternalMissingCommand(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := OpenExternal("https://example.com"); err == nil {
		t.Fatal("expected error when opener is missing")
	}
}
