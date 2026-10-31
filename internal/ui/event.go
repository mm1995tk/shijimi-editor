package ui

import "time"

// Event is something that happened outside the input: a timer firing, later a
// language server's reply or a file changing. The core takes an Event and
// answers with Effects. Nothing here depends on bubbletea, so a front end that
// is not a terminal can feed the same Events.
//
// Only the timers go through this so far (docs/design/gui-protocol.md, P6).
// The other asynchronous pieces (LSP, search, agent, ctl, IDE) still use
// tea.Cmd and move over one subsystem at a time (P7a to P7e).
type Event interface{ isEvent() }

// WatchTick fires every watchInterval: open files are compared with the disk,
// the tree and the CHANGES list are refreshed.
type WatchTick struct{}

// AutoSaveDue fires when a tab has been idle for the auto-save delay. It is
// ignored if the buffer changed again before it fires.
type AutoSaveDue struct {
	tab *tab
	ver int
}

// HoldScrollTick is one frame of a held Ctrl+D / Ctrl+U scroll. seq tells the
// current chain of ticks from a stale one.
type HoldScrollTick struct{ seq int }

// HoverDwell fires hoverDelay after the pointer entered a cell. seq tells it
// from a stale one.
type HoverDwell struct{ seq int }

func (WatchTick) isEvent()      {}
func (AutoSaveDue) isEvent()    {}
func (HoldScrollTick) isEvent() {}
func (HoverDwell) isEvent()     {}

// Effect is what the core asks its front end to do.
type Effect interface{ isEffect() }

// StartTimer asks for Fire to be delivered as an Event after After. Timers are
// never cancelled: the Event carries a sequence number or a buffer version,
// and a stale one is dropped when it arrives.
type StartTimer struct {
	After time.Duration
	Fire  Event
}

// Quit says the program may end.
type Quit struct{}

func (StartTimer) isEffect() {}
func (Quit) isEffect()       {}
