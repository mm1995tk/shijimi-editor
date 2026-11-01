package core

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/ctl"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
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
	tab *Tab
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

// Resized, KeyReleased and ReleasesReported are the front end telling the App
// about itself. They are not Events: App.update takes them before HandleEvent
// would.
//
// Resized is the size of the whole screen in cells. A GUI reports the size of
// each region instead (view.resize); the TUI has the one screen.
type Resized struct{ W, H int }

// KeyReleased is a key let go. Only a terminal that reports releases sends it.
type KeyReleased struct{ Key Key }

// ReleasesReported says whether the terminal reports key releases.
type ReleasesReported struct{ On bool }

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
type LspEventMsg lsp.Event

func (LspEventMsg) isEvent()           {}
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
	res SearchResult
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

// WaitTerm asks for the next thing the terminal panel's shell did (printed,
// ended) to be delivered as an Event. It is asked again after each one
// (terminal.armed keeps it to one at a time). The PTY itself is
// internal/term's; the Events are termOutputMsg and termDoneMsg.
type WaitTerm struct{ From <-chan Event }

func (WaitTerm) isEffect() {}

// RepoProbed carries what the suggestion panel's background look at the
// repository found (branch, conflicts, pull requests).
type RepoProbed struct{ probe repoProbe }

// RunProbe asks for Run to be called off the main loop and its Event to be
// delivered.
type RunProbe struct{ Run func() Event }

// Run asks for Run to be called off the main loop and its Event (nil for none)
// to be delivered. It is the general form of RunProbe and RunSearch, for the work
// behind the diff and review screens, the formatters and the preview.
type Run struct{ Run func() Event }

func (Run) isEffect() {}

func (RepoProbed) isEvent() {}

// async is a Run of fn.
func async(fn func() Event) []Effect { return []Effect{Run{fn}} }
func (RunProbe) isEffect()           {}

// batch joins Effect lists into one. It is the counterpart of tea.Batch for
// code that returns Effects.
func Batch(lists ...[]Effect) []Effect { return slices.Concat(lists...) }

// The work that used to travel as a tea.Msg: what the diff and review screens,
// the formatters, the highlighter and the preview send back from their Run.
func (changesLoadedMsg) isEvent() {}
func (prFoundMsg) isEvent()       {}
func (branchPRsMsg) isEvent()     {}
func (prCheckedOutMsg) isEvent()  {}
func (prPulledMsg) isEvent()      {}
func (prListMsg) isEvent()        {}
func (prReviewMsg) isEvent()      {}
func (viewedMsg) isEvent()        {}
func (reviewSentMsg) isEvent()    {}
func (formatterDoneMsg) isEvent() {}
func (highlightMsg) isEvent()     {}
func (mermaidStartMsg) isEvent()  {}
func (mermaidDoneMsg) isEvent()   {}
func (mdImageDoneMsg) isEvent()   {}
func (openURLMsg) isEvent()       {}

// TermOutputMsg says the terminal panel's shell printed something and
// TermDoneMsg that it ended. A shell that cannot be started is an error from
// the terminal's start, not an Event. The pump of the PTY (internal/term) sends
// both; WaitTerm hands them to the front end, which gives them to the terminal
// (FrontEnd.HandleTermEvent).
type TermOutputMsg struct{ ID int }

type TermDoneMsg struct {
	ID  int
	Err error
}

func (TermOutputMsg) isEvent() {}
func (TermDoneMsg) isEvent()   {}

// Shell is the shell of the terminal panel as the failure scan sees it.
type Shell interface {
	// ID tells the shell from another one (a failure is kept per shell).
	ID() int
	// TakeCommandEnds returns the exit codes of the commands that ended since
	// the last call.
	TakeCommandEnds() []int
	// NewLines returns the lines printed since the last call.
	NewLines() []string
	// Tail returns the last n lines of the screen.
	Tail(n int) []string
}

// HandleEvent is the core's entry for Events: it takes one and answers with the
// Effects to carry out. The front end calls it from its update loop.
func (a *Core) HandleEvent(ev Event) []Effect {
	switch ev := ev.(type) {
	case AutoSaveDue:
		return a.handleAutoSave(ev)
	case WatchTick:
		return a.handleWatch()
	case HoldScrollTick:
		return a.holdScrollTick(ev.seq)
	case HoverDwell:
		return a.hoverDwell(ev)
	case SearchTick:
		return a.searchTick(ev)
	case RepoProbed:
		a.handleRepoProbed(ev)
		return nil
	case SearchDone:
		a.searchDone(ev)
		return nil
	case TermOutputMsg, TermDoneMsg:
		return a.HandleTermEvent(ev)
	case CtlRequest:
		return a.handleCtl(ev)
	case IDECall:
		return a.handleIDE(ev)
	case IDECancel:
		a.cancelProposal(ev.Reply)
		return nil
	case changesLoadedMsg:
		a.applyChanges(ev)
		return nil
	case highlightMsg:
		ev.h.apply(ev)
		return nil
	case mermaidStartMsg:
		return a.handleMermaidStart(ev)
	case mermaidDoneMsg:
		a.handleMermaidDone(ev)
		return nil
	case mdImageDoneMsg:
		a.handleMDImageDone(ev)
		return nil
	case formatterDoneMsg:
		return a.handleFormatterDone(ev)
	case openURLMsg:
		switch {
		case ev.err != nil && ev.copyOnFail:
			a.CopyText(ev.url)
			a.Info(i18n.T("rv.openfailedcopied"), ev.url)
		case ev.err != nil:
			a.Fail(ev.err)
		default:
			a.Info(i18n.T("app.opened"), ev.url)
		}
		return nil
	case prFoundMsg:
		return a.handlePRFound(ev)
	case branchPRsMsg:
		return a.handleBranchPRs(ev)
	case prCheckedOutMsg:
		return a.handlePRCheckedOut(ev)
	case prPulledMsg:
		return a.handlePRPulled(ev)
	case prReviewMsg:
		return a.handlePRReview(ev)
	case viewedMsg:
		a.handleViewed(ev)
		return nil
	case reviewSentMsg:
		return a.handleReviewSent(ev)
	case prListMsg:
		return a.showPRList(ev)
	case LspEventMsg:
		// Ask for the next one first, so none waits while this is handled.
		return append([]Effect{WaitLSP{a.LspEvents()}}, a.handleLSPEvent(lsp.Event(ev))...)
	}
	if effs, ok := a.handleAgentEvent(ev); ok {
		return effs
	}
	if effs, ok := a.handleLSPReply(ev); ok {
		return effs
	}
	return nil
}
