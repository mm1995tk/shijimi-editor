package tui

import (
	"time"

	"github.com/mm1995tk/shijimi-editor/core"
)

// wheelIdle is how long the wheel must be quiet for a gesture to be over.
const wheelIdle = 150 * time.Millisecond

// The lock moves to the other axis when events of that axis come with none of
// the locked axis between them, and both of these hold. A swipe a little off
// the straight mixes the other axis in as single events or short runs, so a
// short run is not a change of direction; a trackpad moved the other way sends
// an unbroken run that lasts. Without this, a swipe that turns from horizontal
// to vertical without a pause never reaches wheelIdle and stays locked.
//
//   - wheelSwitchAfter: the run has lasted this long. A terminal may turn one
//     trackpad event into a bundle of buttons that arrive at the same moment, so
//     a count alone would let a fast flick make a long run in no time.
//   - wheelSwitchCount: the run has this many events that came at different
//     moments. Buttons within wheelSameMoment of the one that began a moment
//     count with it (the terminal sends a bundle at once, but each button gets
//     its own time when it is read, a little apart). A time alone would let two slow
//     events, 50ms or more apart, switch the lock in a slow or ending swipe.
const (
	wheelSwitchAfter = 50 * time.Millisecond
	wheelSwitchCount = 3
	// wheelSameMoment is shorter than the gap between trackpad events (10 to
	// 20ms) and longer than reading a bundle of buttons takes.
	wheelSameMoment = 5 * time.Millisecond
)

// wheelLock keeps one gesture of the wheel on one axis. A terminal reports the
// wheel as buttons with no amount, and a trackpad swiped a little off the
// straight sends both vertical and horizontal buttons for one swipe. The axis
// of the first event of a gesture is the axis of the whole gesture; events of
// the other axis are dropped. The gesture ends when no wheel event has come
// for wheelIdle. Dropped events count as events, so a diagonal swipe does not
// end the gesture in the middle. The lock moves to the other axis early when
// events of that axis have come long enough and often enough (see wheelSwitchAfter).
type wheelLock struct {
	last       time.Time
	horizontal bool
	otherSince time.Time        // when the run of other-axis events began; zero if none
	otherLast  time.Time        // time of the event that began the latest moment
	otherCount int              // moments of the run
	now        func() time.Time // time.Now unless a test sets it
}

// allow reports whether the event may go on. Anything that is not a wheel
// press is always allowed. Shift turns the vertical wheel sideways only over
// the editor text (see HandleMouse); shiftSideways says that is the case, and
// then it counts as the horizontal axis.
func (w *wheelLock) allow(msg core.Mouse, shiftSideways bool) bool {
	if msg.Action != core.MousePress {
		return true
	}
	var horizontal bool
	switch msg.Button {
	case core.MouseWheelUp, core.MouseWheelDown:
		horizontal = shiftSideways
	case core.MouseWheelLeft, core.MouseWheelRight:
		horizontal = true
	default:
		return true
	}
	now := time.Now
	if w.now != nil {
		now = w.now
	}
	t := now()
	if w.last.IsZero() || t.Sub(w.last) >= wheelIdle {
		w.horizontal = horizontal // a new gesture
	}
	w.last = t
	if w.horizontal == horizontal {
		w.otherSince, w.otherLast, w.otherCount = time.Time{}, time.Time{}, 0
		return true
	}
	if w.otherSince.IsZero() {
		w.otherSince = t
	}
	if t.Sub(w.otherLast) >= wheelSameMoment {
		w.otherLast = t
		w.otherCount++
	}
	if w.otherCount >= wheelSwitchCount && t.Sub(w.otherSince) >= wheelSwitchAfter {
		w.horizontal = horizontal // the swipe has turned
		w.otherSince, w.otherLast, w.otherCount = time.Time{}, time.Time{}, 0
		return true
	}
	return false
}
