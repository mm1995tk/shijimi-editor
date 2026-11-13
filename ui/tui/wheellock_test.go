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
	step(100 * time.Millisecond) // 120ms since the dropped one: dropped events keep the gesture
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
