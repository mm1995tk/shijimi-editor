package tui

import (
	"time"

	"github.com/mm1995tk/shijimi-editor/core"
)

// wheelIdle is how long the wheel must be quiet for a gesture to be over.
const wheelIdle = 150 * time.Millisecond

// wheelLock keeps one gesture of the wheel on one axis. A terminal reports the
// wheel as buttons with no amount, and a trackpad swiped a little off the
// straight sends both vertical and horizontal buttons for one swipe. The axis
// of the first event of a gesture is the axis of the whole gesture; events of
// the other axis are dropped. The gesture ends when no wheel event has come
// for wheelIdle. Dropped events count as events, so a diagonal swipe does not
// end the gesture in the middle.
type wheelLock struct {
	last       time.Time
	horizontal bool
	now        func() time.Time // time.Now unless a test sets it
}

// allow reports whether the event may go on. Anything that is not a wheel
// press is always allowed. Shift turns the vertical wheel sideways (see
// HandleMouse), so it counts as the horizontal axis.
func (w *wheelLock) allow(msg core.Mouse) bool {
	if msg.Action != core.MousePress {
		return true
	}
	var horizontal bool
	switch msg.Button {
	case core.MouseWheelUp, core.MouseWheelDown:
		horizontal = msg.Shift
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
	return w.horizontal == horizontal
}
