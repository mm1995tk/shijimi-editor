package tui

import (
	"testing"
	"time"

	"github.com/mm1995tk/shijimi-editor/core"
)

func TestWheelLock(t *testing.T) {
	clock := time.Unix(1000, 0)
	w := &wheelLock{now: func() time.Time { return clock }}
	ev := func(b core.MouseButton, shift bool) bool {
		return w.allow(core.Mouse{Action: core.MousePress, Button: b}, shift)
	}
	step := func(d time.Duration) { clock = clock.Add(d) }

	if !ev(core.MouseWheelDown, false) {
		t.Fatal("first vertical event must pass")
	}
	step(20 * time.Millisecond)
	if ev(core.MouseWheelRight, false) {
		t.Fatal("horizontal event during a vertical gesture must be dropped")
	}
	step(40 * time.Millisecond)
	if !ev(core.MouseWheelDown, false) {
		t.Fatal("vertical event must pass")
	}
	step(100 * time.Millisecond) // no pause: dropped events keep the gesture
	if ev(core.MouseWheelLeft, false) {
		t.Fatal("still the vertical gesture")
	}
	if !ev(core.MouseWheelUp, false) {
		t.Fatal("vertical event must pass")
	}
	step(wheelIdle)
	if !ev(core.MouseWheelLeft, false) {
		t.Fatal("after a pause the other axis must work")
	}
	step(10 * time.Millisecond)
	if ev(core.MouseWheelDown, false) {
		t.Fatal("vertical event during a horizontal gesture must be dropped")
	}
	step(wheelIdle)
	if !ev(core.MouseWheelDown, true) {
		t.Fatal("Shift+vertical starts a horizontal gesture")
	}
	step(10 * time.Millisecond)
	if !ev(core.MouseWheelRight, false) {
		t.Fatal("Shift+vertical and horizontal are the same axis")
	}
	if !w.allow(core.Mouse{Action: core.MouseMotion, Button: core.MouseNone}, false) || !w.allow(core.Mouse{Action: core.MousePress, Button: core.MouseLeft}, false) {
		t.Fatal("non-wheel events always pass")
	}
}

func TestWheelLockSwitchesAxis(t *testing.T) {
	clock := time.Unix(1000, 0)
	w := &wheelLock{now: func() time.Time { return clock }}
	const step = 10 * time.Millisecond // never a pause
	ev := func(b core.MouseButton) bool {
		clock = clock.Add(step)
		return w.allow(core.Mouse{Action: core.MousePress, Button: b}, false)
	}

	ev(core.MouseWheelRight)
	ev(core.MouseWheelRight)
	// A run of vertical events with no pause: dropped until it has lasted wheelSwitchAfter.
	if ev(core.MouseWheelDown) {
		t.Fatal("the first vertical event of the run must be dropped")
	}
	for i := time.Duration(1); i*step < wheelSwitchAfter; i++ {
		if ev(core.MouseWheelDown) {
			t.Fatalf("vertical event %d of the run must still be dropped", i)
		}
	}
	if !ev(core.MouseWheelDown) {
		t.Fatal("the lock must move to vertical once the run has lasted wheelSwitchAfter")
	}
	if !ev(core.MouseWheelUp) {
		t.Fatal("vertical must keep passing after the switch")
	}
	if ev(core.MouseWheelLeft) {
		t.Fatal("a single horizontal event in a vertical gesture is dropped")
	}
	// And back, the other way round.
	for range wheelSwitchAfter / step {
		ev(core.MouseWheelLeft)
	}
	if !ev(core.MouseWheelLeft) {
		t.Fatal("the lock must move back to horizontal")
	}
}

func TestWheelLockKeepsAxisAgainstStrays(t *testing.T) {
	clock := time.Unix(1000, 0)
	w := &wheelLock{now: func() time.Time { return clock }}
	ev := func(b core.MouseButton) bool {
		clock = clock.Add(10 * time.Millisecond)
		return w.allow(core.Mouse{Action: core.MousePress, Button: b}, false)
	}

	ev(core.MouseWheelDown)
	// Horizontal events in short runs between vertical ones never switch.
	for range 20 {
		for range 4 {
			if ev(core.MouseWheelRight) {
				t.Fatal("stray horizontal events must be dropped")
			}
		}
		if !ev(core.MouseWheelDown) {
			t.Fatal("vertical must keep passing")
		}
	}
}

// A terminal may turn one trackpad event into many buttons that arrive at the
// same moment. However long such a bundle is, it is not a run that lasts.
func TestWheelLockKeepsAxisAgainstBundle(t *testing.T) {
	clock := time.Unix(1000, 0)
	w := &wheelLock{now: func() time.Time { return clock }}
	ev := func(b core.MouseButton) bool {
		return w.allow(core.Mouse{Action: core.MousePress, Button: b}, false)
	}

	ev(core.MouseWheelDown)
	clock = clock.Add(10 * time.Millisecond)
	for range 12 {
		if ev(core.MouseWheelRight) {
			t.Fatal("a bundle of horizontal buttons at one moment must be dropped")
		}
	}
	clock = clock.Add(10 * time.Millisecond)
	if !ev(core.MouseWheelDown) {
		t.Fatal("vertical must keep passing")
	}
}
