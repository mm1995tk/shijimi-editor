package tui

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/ctl"
	"github.com/mm1995tk/shijimi-editor/internal/ide"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
)

// Event is something that happened outside the input: a timer firing, later a
// language server's reply or a file changing. The core takes an Event and
// answers with Effects. Nothing here depends on bubbletea, so a front end that
// is not a terminal can feed the same Events.
//
// The timers (P6), the language servers (P7a), the control socket (P7d) and
// the IDE connection (P7e), the project search (P7b) and the agent (P7c) go
// through this (docs/design/gui-protocol.md).
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

// IDECall is a tool call from Claude Code over the IDE connection. The caller
// waits for the answer, which the core hands back as a ReplyIDE Effect. Most
// calls are answered at once; openDiff is answered when the user decides.
type IDECall struct {
	Tool  string
	Args  json.RawMessage
	Reply chan<- IDEReply
}

// IDECancel withdraws a pending IDECall whose client went away.
type IDECancel struct{ Reply chan<- IDEReply }

func (IDECall) isEvent()   {}
func (IDECancel) isEvent() {}

// IDEReply is the result of an IDE tool call.
type IDEReply struct {
	Content []ide.Content
	Err     error
}

// ReplyIDE hands the answer to an IDECall back to the IDE connection. The
// channel is buffered, so carrying it out never blocks.
type ReplyIDE struct {
	To    chan<- IDEReply
	Reply IDEReply
}

// SearchTick fires searchDebounce after the last key typed into the search
// window. seq tells it from a stale one: it is ignored if more was typed since.
type SearchTick struct{ seq int64 }

// SearchDone carries the result of one project search. It is ignored if a
// newer search was started since (seq no longer matches).
type SearchDone struct {
	seq int64
	res core.SearchResult
}

func (SearchTick) isEvent() {}
func (SearchDone) isEvent() {}

// RunSearch asks for Run to be called off the main loop and its Event to be
// delivered. The search stops by itself once a newer one supersedes it.
type RunSearch struct{ Run func() Event }

// Quit says the program may end.
type Quit struct{}

func (StartTimer) isEffect() {}
func (Quit) isEffect()       {}
func (ReplyCtl) isEffect()   {}
func (ReplyIDE) isEffect()   {}
func (RunLSP) isEffect()     {}
func (WaitLSP) isEffect()    {}
func (RunSearch) isEffect()  {}

// The agent's side (P7c). Its goroutines put these on agentState.ch, in the
// order they happened, and WaitAgent delivers them one at a time. Each one
// carries the generation of the client it came from (agentState.gen), so that
// a late one from a client that was dropped is ignored.
func (agentReadyMsg) isEvent()      {}
func (agentUpdateMsg) isEvent()     {}
func (agentFileMsg) isEvent()       {}
func (agentPermMsg) isEvent()       {}
func (agentPermCancelMsg) isEvent() {}
func (agentTurnMsg) isEvent()       {}
func (agentExitMsg) isEvent()       {}

// WaitAgent asks for the next Event the agent's side produced to be delivered.
// It is asked again after each one (agentState.armed keeps it to one at a time).
type WaitAgent struct{ From <-chan Event }

// RunAgent asks for Run to be called off the main loop: starting the agent
// process and its session, or sending a prompt and waiting for the turn to
// end. Its results reach the core as Events through the channel WaitAgent reads.
type RunAgent struct{ Run func() }

func (WaitAgent) isEffect() {}
func (RunAgent) isEffect()  {}

// RepoProbed carries what the suggestion panel's background look at the
// repository found (branch, conflicts, pull requests).
type RepoProbed struct{ probe repoProbe }

// RunProbe asks for Run to be called off the main loop and its Event to be
// delivered.
type RunProbe struct{ Run func() Event }

func (RepoProbed) isEvent() {}
func (RunProbe) isEffect()  {}

// batch joins Effect lists into one. It is the counterpart of tea.Batch for
// code that returns Effects.
func batch(lists ...[]Effect) []Effect { return slices.Concat(lists...) }
