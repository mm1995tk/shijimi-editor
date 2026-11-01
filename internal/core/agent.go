package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mm1995tk/shijimi-editor/internal/acp"
	"github.com/mm1995tk/shijimi-editor/internal/config"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// The agent behind the suggestion panel, spoken to with the Agent Client
// Protocol. It starts on the first suggestion that needs it and keeps one
// session. Files it reads and writes go through the editor: a read sees the
// open buffer, and a write lands on disk, where the file watcher reloads the
// buffer and diff mode shows the change.

// agentState is the connection to the agent and what the panel shows of it.
type agentState struct {
	Client  *acp.Client
	session string
	name    string // the agent's name for display
	gen     int    // bumped when a client is dropped; its late messages are ignored
	ch      chan Event
	armed   bool // a WaitAgent effect is outstanding
	Run     *agentRun
	Log     []LogEntry
	Perms   []*permAsk // permission requests waiting for y / n
	Scroll  int        // log lines scrolled back from the bottom; 0 follows
	View    agentView
	Sel     AgentSel
}

// agentRun is the turn in progress.
type agentRun struct {
	kind    suggestKind
	key     string
	build   func() string // builds the prompt; runs off the UI goroutine
	text    strings.Builder
	written []string // files the agent wrote, absolute
	pr      int      // the pull request an AI review is for
}

type logKind byte

const (
	logUser  logKind = 'u' // what was asked
	LogAgent logKind = 'a' // the agent's reply (Markdown)
	logTool  logKind = 't' // a tool call
	logPlan  logKind = 'p'
	LogInfo  logKind = 'i'
	logError logKind = 'e'
)

// LogEntry is one entry of the agent's log: what was asked, the reply, a tool
// call, the plan or an error.
type LogEntry struct {
	Kind   logKind
	Text   string
	id     string // tool call id
	status string // tool call status
}

// permAsk is a permission request shown as a card in the panel.
type permAsk struct {
	gen   int
	req   acp.PermissionRequest
	reply chan string
}

// maxLog bounds the panel log.
const maxLog = 500

func newAgentState() *agentState { return &agentState{ch: make(chan Event, 256)} }

// wait returns the Effect that delivers the next Event from the agent, or nil
// if one is already outstanding.
func (s *agentState) wait() []Effect {
	if s.armed {
		return nil
	}
	s.armed = true
	return []Effect{WaitAgent{From: s.ch}}
}

func (s *agentState) add(e LogEntry) {
	if e.Kind == LogAgent && len(s.Log) > 0 && s.Log[len(s.Log)-1].Kind == LogAgent {
		s.Log[len(s.Log)-1].Text += e.Text
		return
	}
	s.Log = append(s.Log, e)
	if len(s.Log) > maxLog {
		s.Log = s.Log[len(s.Log)-maxLog:]
	}
}

// ---- messages from the agent's goroutines ----

type agentReadyMsg struct {
	gen     int
	client  *acp.Client
	session string
	err     error
}

type agentUpdateMsg struct {
	gen int
	u   acp.Update
}

type agentFileReply struct {
	text string
	err  error
}

type agentFileMsg struct {
	gen   int
	read  *acp.ReadRequest
	write *acp.WriteRequest
	reply chan agentFileReply
}

type agentPermMsg struct{ ask *permAsk }

// agentPermCancelMsg withdraws a permission request the agent gave up on.
type agentPermCancelMsg struct{ reply chan string }

type agentTurnMsg struct {
	gen  int
	stop string
	err  error
}

type agentExitMsg struct {
	gen int
	err error
}

// agentHandler forwards the agent's requests to the core as Events on
// agentState.ch.
type agentHandler struct {
	ch  chan Event
	gen int
}

func (h agentHandler) SessionUpdate(_ string, u acp.Update) { h.ch <- agentUpdateMsg{h.gen, u} }

func (h agentHandler) file(ctx context.Context, m agentFileMsg) (string, error) {
	m.gen, m.reply = h.gen, make(chan agentFileReply, 1)
	h.ch <- m
	select {
	case r := <-m.reply:
		return r.text, r.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (h agentHandler) ReadTextFile(ctx context.Context, r acp.ReadRequest) (string, error) {
	return h.file(ctx, agentFileMsg{read: &r})
}

func (h agentHandler) WriteTextFile(ctx context.Context, r acp.WriteRequest) error {
	_, err := h.file(ctx, agentFileMsg{write: &r})
	return err
}

func (h agentHandler) RequestPermission(ctx context.Context, r acp.PermissionRequest) (string, error) {
	ask := &permAsk{gen: h.gen, req: r, reply: make(chan string, 1)}
	h.ch <- agentPermMsg{ask}
	select {
	case id := <-ask.reply:
		return id, nil
	case <-ctx.Done():
		h.ch <- agentPermCancelMsg{ask.reply}
		return "", ctx.Err()
	}
}

// ---- running a turn ----

// RunAgent sends a prompt built by build for the suggestion of kind and key,
// starting the agent first if needed.
func (a *Core) RunAgent(kind suggestKind, key, title string, build func() string) []Effect {
	s := a.Ag
	if s.Run != nil {
		a.Fail(errors.New(i18n.T("agent.busy")))
		return nil
	}
	if a.Cfg.Agent.Command == "" {
		dir, _ := config.UserDir()
		a.Fail(fmt.Errorf(i18n.T("agent.notconfigured"), filepath.Join(dir, "config.toml")))
		return nil
	}
	s.Run = &agentRun{kind: kind, key: key, build: build}
	if a.PrShown() {
		s.Run.pr = a.Gd.Pr.Number
	}
	s.Scroll = 0
	s.add(LogEntry{Kind: logUser, Text: title})
	if s.Client == nil {
		s.add(LogEntry{Kind: LogInfo, Text: i18n.T("agent.starting") + a.Cfg.Agent.Command})
		return append([]Effect{a.startAgent()}, s.wait()...)
	}
	return append([]Effect{a.sendPrompt()}, s.wait()...)
}

// startAgent launches the agent and opens a session in the background.
func (a *Core) startAgent() Effect {
	s := a.Ag
	s.gen++
	gen, ch, root := s.gen, s.ch, a.Root
	o := acp.Options{Command: a.Cfg.Agent.Command, Args: a.Cfg.Agent.Args, Env: a.Cfg.Agent.Env, Dir: root, Version: VersionLabel()}
	return RunAgent{Run: func() {
		c, err := acp.Start(o, agentHandler{ch: ch, gen: gen})
		if err != nil {
			ch <- agentReadyMsg{gen: gen, err: err}
			return
		}
		ctx := context.Background()
		sid := ""
		if err = c.Initialize(ctx); err == nil {
			sid, err = c.NewSession(ctx, root)
		}
		if err != nil {
			c.Close()
			ch <- agentReadyMsg{gen: gen, err: err}
			return
		}
		ch <- agentReadyMsg{gen: gen, client: c, session: sid}
		go func() {
			<-c.Done()
			ch <- agentExitMsg{gen: gen, err: c.Err()}
		}()
	}}
}

// sendPrompt sends the current run's prompt. The answer goes through the
// channel after the updates the agent sent before it.
func (a *Core) sendPrompt() Effect {
	s := a.Ag
	c, sid, gen, ch, build := s.Client, s.session, s.gen, s.ch, s.Run.build
	return RunAgent{Run: func() {
		stop, err := c.Prompt(context.Background(), sid, acp.Text(build()))
		ch <- agentTurnMsg{gen: gen, stop: stop, err: err}
	}}
}

// cancelAgent asks the agent to stop the running turn.
func (a *Core) cancelAgent() {
	s := a.Ag
	if s.Run == nil || s.Client == nil {
		a.InfoText(i18n.T("agent.idle"))
		return
	}
	if err := s.Client.Cancel(s.session); err != nil {
		a.Fail(err)
		return
	}
	a.InfoText(i18n.T("agent.cancelrequested"))
}

// stopAgent ends the agent process; the next suggestion starts a new one.
func (a *Core) stopAgent() {
	s := a.Ag
	if s.Client != nil {
		s.Client.Close()
	}
	s.Client, s.session, s.name = nil, "", ""
	s.gen++
	s.Run = nil
	// the fix it proposed lives in the session that is gone
	a.Sg.FixOffer = false
	for _, p := range s.Perms {
		p.reply <- ""
	}
	s.Perms = nil
}

// handleAgentEvent handles every Event from the agent. It reports whether
// ev was one.
func (a *Core) handleAgentEvent(ev Event) ([]Effect, bool) {
	s := a.Ag
	switch ev.(type) {
	case agentReadyMsg, agentUpdateMsg, agentFileMsg, agentPermMsg, agentPermCancelMsg, agentTurnMsg, agentExitMsg:
		// The WaitAgent that delivered ev is done; each case asks for the next.
		s.armed = false
	default:
		return nil, false
	}
	switch m := ev.(type) {
	case agentReadyMsg:
		if m.gen != s.gen {
			if m.client != nil {
				m.client.Close()
			}
			return s.wait(), true
		}
		if m.err != nil {
			s.add(LogEntry{Kind: logError, Text: i18n.T("agent.startfail") + m.err.Error()})
			a.Fail(fmt.Errorf(i18n.T("agent.startfailw"), m.err))
			s.Run = nil
			return s.wait(), true
		}
		s.Client, s.session, s.name = m.client, m.session, m.client.Info().Name()
		if s.Run == nil {
			return s.wait(), true
		}
		return append([]Effect{a.sendPrompt()}, s.wait()...), true
	case agentUpdateMsg:
		if m.gen == s.gen {
			a.agentUpdate(m.u)
		}
		return s.wait(), true
	case agentFileMsg:
		if m.gen != s.gen {
			m.reply <- agentFileReply{err: errors.New(i18n.T("agent.sessionended"))}
			return s.wait(), true
		}
		if m.read != nil {
			text, err := a.agentRead(*m.read)
			m.reply <- agentFileReply{text: text, err: err}
			return s.wait(), true
		}
		err := a.agentWrite(*m.write)
		m.reply <- agentFileReply{err: err}
		if err != nil {
			return s.wait(), true
		}
		effs := s.wait()
		if a.Gd.On {
			effs = append(effs, a.ReloadChanges()...)
		}
		return effs, true
	case agentPermMsg:
		if m.ask.gen != s.gen {
			m.ask.reply <- ""
			return s.wait(), true
		}
		s.Perms = append(s.Perms, m.ask)
		if !a.AgentOpen() || a.Focus != FocusAgent {
			a.SetAgent(true)
			a.InfoText(i18n.T("agent.permask"))
		}
		return s.wait(), true
	case agentPermCancelMsg:
		for i, p := range s.Perms {
			if p.reply == m.reply {
				s.Perms = append(s.Perms[:i], s.Perms[i+1:]...)
				break
			}
		}
		return s.wait(), true
	case agentTurnMsg:
		if m.gen != s.gen {
			return s.wait(), true
		}
		return append(a.endTurn(m), s.wait()...), true
	case agentExitMsg:
		if m.gen != s.gen {
			return s.wait(), true
		}
		text := i18n.T("acp.exited")
		if m.err != nil {
			text += ": " + m.err.Error()
		}
		if st := strings.TrimSpace(s.Client.Stderr()); st != "" {
			text += "\n" + lastLines(st, 5)
		}
		s.add(LogEntry{Kind: logError, Text: text})
		s.Client = nil
		a.stopAgent()
		return s.wait(), true
	}
	return nil, false
}

func lastLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// agentUpdate adds the agent's progress to the log.
func (a *Core) agentUpdate(u acp.Update) {
	s := a.Ag
	switch u.Kind {
	case "agent_message_chunk":
		text := u.Text()
		if s.Run != nil {
			s.Run.text.WriteString(text)
		}
		s.add(LogEntry{Kind: LogAgent, Text: text})
	case "tool_call":
		s.add(LogEntry{Kind: logTool, id: u.ToolCallID, Text: u.Title, status: u.Status})
	case "tool_call_update":
		for i := len(s.Log) - 1; i >= 0; i-- {
			if e := &s.Log[i]; e.Kind == logTool && e.id == u.ToolCallID {
				if u.Title != "" {
					e.Text = u.Title
				}
				if u.Status != "" {
					e.status = u.Status
				}
				return
			}
		}
	case "plan":
		var sb strings.Builder
		for _, e := range u.Entries {
			mark := "○"
			switch e.Status {
			case "in_progress":
				mark = "◐"
			case "completed":
				mark = "●"
			}
			fmt.Fprintf(&sb, "%s %s\n", mark, e.Content)
		}
		// A plan is sent whole each time; the newest replaces the last one.
		if n := len(s.Log); n > 0 && s.Log[n-1].Kind == logPlan {
			s.Log[n-1].Text = sb.String()
			return
		}
		s.add(LogEntry{Kind: logPlan, Text: sb.String()})
	}
}

// agentPath turns a path from the agent into an absolute, clean one.
func (a *Core) agentPath(p string) string {
	if !filepath.IsAbs(p) {
		p = filepath.Join(a.Root, p)
	}
	return filepath.Clean(p)
}

// agentRead answers fs/read_text_file: an open file is read from its buffer,
// unsaved edits included.
func (a *Core) agentRead(r acp.ReadRequest) (string, error) {
	p := a.agentPath(r.Path)
	var text string
	if t := a.tabFor(p); t != nil {
		text = t.Buf.Text()
	} else {
		b, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		text = string(b)
	}
	if r.Line <= 0 && r.Limit <= 0 {
		return text, nil
	}
	lines := strings.SplitAfter(text, "\n")
	from := max(0, r.Line-1)
	if from >= len(lines) {
		return "", nil
	}
	to := len(lines)
	if r.Limit > 0 {
		to = min(to, from+r.Limit)
	}
	return strings.Join(lines[from:to], ""), nil
}

// agentWrite answers fs/write_text_file. The file is written on disk and the
// open buffer follows as for any outside change. A buffer with unsaved edits
// is never overwritten.
func (a *Core) agentWrite(r acp.WriteRequest) error {
	p := a.agentPath(r.Path)
	if run := a.Ag.Run; run != nil && (run.kind == SgAIReview || run.kind == SgFailure) {
		// These only read: a pull request or a log could carry instructions.
		return errors.New(i18n.T("agent.nowrite"))
	}
	if t := a.tabFor(p); t != nil && t.dirty() {
		return fmt.Errorf(i18n.T("agent.unsaved"), a.Rel(p))
	}
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(p); err == nil {
		mode = fi.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(p, []byte(r.Content), mode); err != nil {
		return err
	}
	a.Ag.add(LogEntry{Kind: LogInfo, Text: "✎ " + a.Rel(p)})
	if run := a.Ag.Run; run != nil {
		run.written = append(run.written, p)
	}
	a.checkDiskChanges()
	a.Explorer.Refresh()
	return nil
}

// AnswerPermission answers the oldest permission request: allow takes the
// first "allow" option, reject the first "reject" one.
func (a *Core) AnswerPermission(allow bool) {
	s := a.Ag
	if len(s.Perms) == 0 {
		return
	}
	p := s.Perms[0]
	s.Perms = s.Perms[1:]
	want := []string{"reject_once", "reject_always"}
	if allow {
		want = []string{"allow_once", "allow_always"}
	}
	id := ""
	for _, k := range want {
		for _, o := range p.req.Options {
			if id == "" && o.Kind == k {
				id = o.OptionID
			}
		}
	}
	p.reply <- id
	verb := i18n.T("agent.denied")
	if allow {
		verb = i18n.T("agent.allowed")
	}
	s.add(LogEntry{Kind: LogInfo, Text: verb + ": " + permTitle(p.req)})
}

func permTitle(r acp.PermissionRequest) string {
	if r.ToolCall.Title != "" {
		return r.ToolCall.Title
	}
	return r.ToolCall.Kind
}

// endTurn finishes a run and does what its suggestion promised.
func (a *Core) endTurn(m agentTurnMsg) []Effect {
	s := a.Ag
	run := s.Run
	s.Run = nil
	for _, p := range s.Perms {
		p.reply <- ""
	}
	s.Perms = nil
	if m.err != nil {
		s.add(LogEntry{Kind: logError, Text: m.err.Error()})
		a.Fail(m.err)
		return nil
	}
	if m.stop != "end_turn" {
		s.add(LogEntry{Kind: LogInfo, Text: stopText(m.stop)})
		a.Info(i18n.T("agent.prefix"), stopText(m.stop))
		return nil
	}
	if run == nil {
		return nil
	}
	switch run.kind {
	case SgFailure:
		a.Sg.FixOffer = true
		a.Sg.fixSeq++
		a.InfoText(i18n.T("agent.diagnosed"))
		return nil
	case SgAIReview:
		return a.placeReview(run)
	}
	s.add(LogEntry{Kind: LogInfo, Text: i18n.T("agent.donereview")})
	a.InfoText(i18n.T("agent.workdone"))
	return a.showAgentChanges()
}

func stopText(stop string) string {
	switch stop {
	case "cancelled":
		return i18n.T("agent.cancelled")
	case "max_tokens":
		return i18n.T("agent.maxtokens")
	case "max_turn_requests":
		return i18n.T("agent.maxturns")
	case "refusal":
		return i18n.T("agent.refused")
	}
	return fmt.Sprintf(i18n.T("agent.stopped"), stop)
}

// showAgentChanges opens diff mode so the agent's edits can be checked.
func (a *Core) showAgentChanges() []Effect {
	if a.Gd.On {
		return a.ReloadChanges()
	}
	return a.ShowDiff(BaseHead, nil)
}
