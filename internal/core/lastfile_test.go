package core

import (
	"testing"
)

// ga and Ctrl+Tab go back and forth between the last two files.
func TestSwitchLastFile(t *testing.T) {
	a := newTestApp(t, "one\n")
	ps := writeFiles(t, a.root, "b.txt", "c.txt")
	a.TrackLast()
	for _, p := range ps {
		a.openFile(p)
		a.TrackLast()
	}
	a.switchLast()
	a.TrackLast()
	if a.tabs[a.active].Path() != ps[0] {
		t.Fatalf("active = %q", a.tabs[a.active].Path())
	}
	a.handleKey(Key{Code: KeyTab, Mod: ModCtrl})
	a.TrackLast()
	if a.tabs[a.active].Path() != ps[1] {
		t.Fatalf("active = %q", a.tabs[a.active].Path())
	}
}

// The old tab-switching keys do nothing now.
func TestOldTabKeysGone(t *testing.T) {
	a := newTestApp(t, "one\n")
	a.openFile(writeFiles(t, a.root, "b.txt")[0])
	for _, k := range []Key{
		{Code: KeyPgUp, Mod: ModCtrl},
		{Code: KeyPgDown, Mod: ModCtrl},
		{Code: '1', Mod: ModAlt},
	} {
		a.handleKey(k)
		if a.active != 1 {
			t.Fatalf("%v switched to %d", k, a.active)
		}
	}
}
