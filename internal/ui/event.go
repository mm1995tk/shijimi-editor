package ui

import (
	"context"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/ctl"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
)

// Event is something that happened outside the input: a timer firing, later a
// language server's reply or a file changing. The core takes an Event and
// answers with Effects. Nothing here depends on bubbletea, so a front end that
// is not a terminal can feed the same Events.
//
// The timers (P6), the language servers (P7a) and the control socket
// (P7d) go through this so far (docs/design/gui-protocol.md). The other
// asynchronous pieces (search, agent, IDE) still use tea.Cmd and move over
// one subsystem at a time (P7b, P7c, P7e).
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

// CtlRequest is a request that came in on the control socket. The caller waits
// for the answer, which the core hands back as a ReplyCtl Effect.
type CtlRequest struct {
	Req   ctl.Request
	Reply chan<- ctl.Response
}

func (CtlRequest) isEvent() {}

func (WatchTick) isEvent()      {}
func (AutoSaveDue) isEvent()    {}
func (HoldScrollTick) isEvent() {}
func (HoverDwell) isEvent()     {}

// The language servers' side. A reply carries what the request was made for
// (the tab, a sequence number, the buffer version) so that a stale one can be
// dropped when it arrives: signatureMsg.seq against App.sigSeq, completionMsg
// against the tab, workspaceSymbolsMsg.seq and symbolQueryMsg.seq against the
// picker, mouseHover.seq against App.hoverSeq.

// lspEventMsg is something a server sent on its own: a diagnostic, a message,
// an edit it wants applied, a document it wants shown.
type lspEventMsg lsp.Event

func (lspEventMsg) isEvent()           {}
func (hoverMsg) isEvent()              {}
func (locationsMsg) isEvent()          {}
func (completionMsg) isEvent()         {}
func (signatureMsg) isEvent()          {}
func (workspaceEditMsg) isEvent()      {}
func (symbolsMsg) isEvent()            {}
func (symbolQueryMsg) isEvent()        {}
func (workspaceSymbolsMsg) isEvent()   {}
func (codeActionsMsg) isEvent()        {}
func (codeActionResolvedMsg) isEvent() {}
func (formatMsg) isEvent()             {}
func (lspErrMsg) isEvent()             {}
func (semanticMsg) isEvent()           {}

// Effect is what the core asks its front end to do.
type Effect interface{ isEffect() }

// StartTimer asks for Fire to be delivered as an Event after After. Timers are
// never cancelled: the Event carries a sequence number or a buffer version,
// and a stale one is dropped when it arrives.
type StartTimer struct {
	After time.Duration
	Fire  Event
}

// RunLSP asks for Run to be called off the main loop with a context that ends
// after lspTimeout, and its Event (nil for none) to be delivered.
type RunLSP struct {
	Run func(ctx context.Context) Event
}

// WaitLSP asks for the next event a language server sends on its own to be
// delivered as an Event. It is asked again after each one.
type WaitLSP struct{ From <-chan lsp.Event }

// ReplyCtl hands the answer to a CtlRequest back to the control socket. The
// channel is buffered, so carrying it out never blocks.
type ReplyCtl struct {
	To   chan<- ctl.Response
	Resp ctl.Response
}

// Quit says the program may end.
type Quit struct{}

func (StartTimer) isEffect() {}
func (Quit) isEffect()       {}
func (ReplyCtl) isEffect()   {}
func (RunLSP) isEffect()     {}
func (WaitLSP) isEffect()    {}
