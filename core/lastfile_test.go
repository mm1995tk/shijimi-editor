package core

import (
	"testing"
)

// ga and Ctrl+Tab go back and forth between the last two files.
func TestSwitchLastFile(t *testing.T) {
	a := newTestApp(t, "one\n")
	ps := writeFiles(t, a.Root, "b.txt", "c.txt")
	a.TrackLast()
	for _, p := range ps {
		a.OpenFile(p)
		a.TrackLast()
	}
	a.SwitchLast()
	a.TrackLast()
	if a.Tabs[a.Active].Path() != ps[0] {
		t.Fatalf("active = %q", a.Tabs[a.Active].Path())
	}
	a.HandleKey(Key{Code: KeyTab, Mod: ModCtrl})
	a.TrackLast()
	if a.Tabs[a.Active].Path() != ps[1] {
		t.Fatalf("active = %q", a.Tabs[a.Active].Path())
	}
}

// The old tab-switching keys do nothing now.
func TestOldTabKeysGone(t *testing.T) {
	a := newTestApp(t, "one\n")
	a.OpenFile(writeFiles(t, a.Root, "b.txt")[0])
	for _, k := range []Key{
		{Code: KeyPgUp, Mod: ModCtrl},
		{Code: KeyPgDown, Mod: ModCtrl},
		{Code: '1', Mod: ModAlt},
	} {
		a.HandleKey(k)
		if a.Active != 1 {
			t.Fatalf("%v switched to %d", k, a.Active)
		}
	}
}
