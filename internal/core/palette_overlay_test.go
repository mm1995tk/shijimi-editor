package core

import (
	"testing"
)

func TestRuneIndexes(t *testing.T) {
	got := runeIndexes("保存as", []int{3, 6, 7})
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Errorf("got %v, want [1 2 3]", got)
	}
	if runeIndexes("abc", nil) != nil {
		t.Error("no offsets must give nil")
	}
}
