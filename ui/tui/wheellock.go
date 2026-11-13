package tui

import (
	"time"

	"github.com/mm1995tk/shijimi-editor/core"
)

// wheelIdle is how long the wheel must be quiet for a gesture to be over.
const wheelIdle = 150 * time.Millisecond

// wheelSwitch is how many events of the other axis in a row, with none of the
// locked axis between them, move the lock to that axis. A swipe a little off
// the straight mixes the other axis in as single events or short runs, so a few
// in a row are not a change of direction; a trackpad that is moved the other way
// sends a long unbroken run. Without this, a swipe that turns from horizontal to
// vertical without a pause never reaches wheelIdle and stays locked.
const wheelSwitch = 5

// wheelLock keeps one gesture of the wheel on one axis. A terminal reports the
// wheel as buttons with no amount, and a trackpad swiped a little off the
// straight sends both vertical and horizontal buttons for one swipe. The axis
// of the first event of a gesture is the axis of the whole gesture; events of
// the other axis are dropped. The gesture ends when no wheel event has come
// for wheelIdle. Dropped events count as events, so a diagonal swipe does not
// end the gesture in the middle. The lock moves to the other axis early when
// wheelSwitch events of that axis come in a row.
type wheelLock struct {
	last       time.Time
	horizontal bool
	other      int              // events of the other axis in a row since the last of the locked one
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
		w.other = 0
	}
	w.last = t
	if w.horizontal == horizontal {
		w.other = 0
		return true
	}
	w.other++
	if w.other >= wheelSwitch {
		w.horizontal = horizontal // the swipe has turned
		w.other = 0
		return true
	}
	return false
}
