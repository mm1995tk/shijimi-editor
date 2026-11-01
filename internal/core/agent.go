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
	client  *acp.Client
	session string
	name    string // the agent's name for display
	gen     int    // bumped when a client is dropped; its late messages are ignored
	ch      chan Event
	armed   bool // a WaitAgent effect is outstanding
	run     *agentRun
	log     []logEntry
	perms   []*permAsk // permission requests waiting for y / n
	scroll  int        // log lines scrolled back from the bottom; 0 follows
	view    agentView
	sel     agentSel
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
	logAgent logKind = 'a' // the agent's reply (Markdown)
	logTool  logKind = 't' // a tool call
	logPlan  logKind = 'p'
	logInfo  logKind = 'i'
	logError logKind = 'e'
)

type logEntry struct {
	kind   logKind
	text   string
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

func (s *agentState) add(e logEntry) {
	if e.kind == logAgent && len(s.log) > 0 && s.log[len(s.log)-1].kind == logAgent {
		s.log[len(s.log)-1].text += e.text
		return
	}
	s.log = append(s.log, e)
	if len(s.log) > maxLog {
		s.log = s.log[len(s.log)-maxLog:]
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

// runAgent sends a prompt built by build for the suggestion of kind and key,
// starting the agent first if needed.
func (a *Core) runAgent(kind suggestKind, key, title string, build func() string) []Effect {
	s := a.ag
	if s.run != nil {
		a.fail(errors.New(i18n.T("agent.busy")))
		return nil
	}
	if a.cfg.Agent.Command == "" {
		dir, _ := config.UserDir()
		a.fail(fmt.Errorf(i18n.T("agent.notconfigured"), filepath.Join(dir, "config.toml")))
		return nil
	}
	s.run = &agentRun{kind: kind, key: key, build: build}
	if a.prShown() {
		s.run.pr = a.gd.pr.Number
	}
	s.scroll = 0
	s.add(logEntry{kind: logUser, text: title})
	if s.client == nil {
		s.add(logEntry{kind: logInfo, text: i18n.T("agent.starting") + a.cfg.Agent.Command})
		return append([]Effect{a.startAgent()}, s.wait()...)
	}
	return append([]Effect{a.sendPrompt()}, s.wait()...)
}

// startAgent launches the agent and opens a session in the background.
func (a *Core) startAgent() Effect {
	s := a.ag
	s.gen++
	gen, ch, root := s.gen, s.ch, a.root
	o := acp.Options{Command: a.cfg.Agent.Command, Args: a.cfg.Agent.Args, Env: a.cfg.Agent.Env, Dir: root, Version: versionLabel()}
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
	s := a.ag
	c, sid, gen, ch, build := s.client, s.session, s.gen, s.ch, s.run.build
	return RunAgent{Run: func() {
		stop, err := c.Prompt(context.Background(), sid, acp.Text(build()))
		ch <- agentTurnMsg{gen: gen, stop: stop, err: err}
	}}
}

// cancelAgent asks the agent to stop the running turn.
func (a *Core) cancelAgent() {
	s := a.ag
	if s.run == nil || s.client == nil {
		a.infoText(i18n.T("agent.idle"))
		return
	}
	if err := s.client.Cancel(s.session); err != nil {
		a.fail(err)
		return
	}
	a.infoText(i18n.T("agent.cancelrequested"))
}

// stopAgent ends the agent process; the next suggestion starts a new one.
func (a *Core) stopAgent() {
	s := a.ag
	if s.client != nil {
		s.client.Close()
	}
	s.client, s.session, s.name = nil, "", ""
	s.gen++
	s.run = nil
	// the fix it proposed lives in the session that is gone
	a.sg.fixOffer = false
	for _, p := range s.perms {
		p.reply <- ""
	}
	s.perms = nil
}

// handleAgentEvent handles every Event from the agent. It reports whether
// ev was one.
func (a *Core) handleAgentEvent(ev Event) ([]Effect, bool) {
	s := a.ag
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
			s.add(logEntry{kind: logError, text: i18n.T("agent.startfail") + m.err.Error()})
			a.fail(fmt.Errorf(i18n.T("agent.startfailw"), m.err))
			s.run = nil
			return s.wait(), true
		}
		s.client, s.session, s.name = m.client, m.session, m.client.Info().Name()
		if s.run == nil {
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
		if a.gd.on {
			effs = append(effs, a.reloadChanges()...)
		}
		return effs, true
	case agentPermMsg:
		if m.ask.gen != s.gen {
			m.ask.reply <- ""
			return s.wait(), true
		}
		s.perms = append(s.perms, m.ask)
		if !a.AgentOpen() || a.focus != focusAgent {
			a.SetAgent(true)
			a.infoText(i18n.T("agent.permask"))
		}
		return s.wait(), true
	case agentPermCancelMsg:
		for i, p := range s.perms {
			if p.reply == m.reply {
				s.perms = append(s.perms[:i], s.perms[i+1:]...)
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
		if st := strings.TrimSpace(s.client.Stderr()); st != "" {
			text += "\n" + lastLines(st, 5)
		}
		s.add(logEntry{kind: logError, text: text})
		s.client = nil
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
	s := a.ag
	switch u.Kind {
	case "agent_message_chunk":
		text := u.Text()
		if s.run != nil {
			s.run.text.WriteString(text)
		}
		s.add(logEntry{kind: logAgent, text: text})
	case "tool_call":
		s.add(logEntry{kind: logTool, id: u.ToolCallID, text: u.Title, status: u.Status})
	case "tool_call_update":
		for i := len(s.log) - 1; i >= 0; i-- {
			if e := &s.log[i]; e.kind == logTool && e.id == u.ToolCallID {
				if u.Title != "" {
					e.text = u.Title
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
		if n := len(s.log); n > 0 && s.log[n-1].kind == logPlan {
			s.log[n-1].text = sb.String()
			return
		}
		s.add(logEntry{kind: logPlan, text: sb.String()})
	}
}

// agentPath turns a path from the agent into an absolute, clean one.
func (a *Core) agentPath(p string) string {
	if !filepath.IsAbs(p) {
		p = filepath.Join(a.root, p)
	}
	return filepath.Clean(p)
}

// agentRead answers fs/read_text_file: an open file is read from its buffer,
// unsaved edits included.
func (a *Core) agentRead(r acp.ReadRequest) (string, error) {
	p := a.agentPath(r.Path)
	var text string
	if t := a.tabFor(p); t != nil {
		text = t.buf.Text()
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
	if run := a.ag.run; run != nil && (run.kind == sgAIReview || run.kind == sgFailure) {
		// These only read: a pull request or a log could carry instructions.
		return errors.New(i18n.T("agent.nowrite"))
	}
	if t := a.tabFor(p); t != nil && t.dirty() {
		return fmt.Errorf(i18n.T("agent.unsaved"), a.rel(p))
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
	a.ag.add(logEntry{kind: logInfo, text: "✎ " + a.rel(p)})
	if run := a.ag.run; run != nil {
		run.written = append(run.written, p)
	}
	a.checkDiskChanges()
	a.explorer.refresh()
	return nil
}

// answerPermission answers the oldest permission request: allow takes the
// first "allow" option, reject the first "reject" one.
func (a *Core) answerPermission(allow bool) {
	s := a.ag
	if len(s.perms) == 0 {
		return
	}
	p := s.perms[0]
	s.perms = s.perms[1:]
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
	s.add(logEntry{kind: logInfo, text: verb + ": " + permTitle(p.req)})
}

func permTitle(r acp.PermissionRequest) string {
	if r.ToolCall.Title != "" {
		return r.ToolCall.Title
	}
	return r.ToolCall.Kind
}

// endTurn finishes a run and does what its suggestion promised.
func (a *Core) endTurn(m agentTurnMsg) []Effect {
	s := a.ag
	run := s.run
	s.run = nil
	for _, p := range s.perms {
		p.reply <- ""
	}
	s.perms = nil
	if m.err != nil {
		s.add(logEntry{kind: logError, text: m.err.Error()})
		a.fail(m.err)
		return nil
	}
	if m.stop != "end_turn" {
		s.add(logEntry{kind: logInfo, text: stopText(m.stop)})
		a.info(i18n.T("agent.prefix"), stopText(m.stop))
		return nil
	}
	if run == nil {
		return nil
	}
	switch run.kind {
	case sgFailure:
		a.sg.fixOffer = true
		a.sg.fixSeq++
		a.infoText(i18n.T("agent.diagnosed"))
		return nil
	case sgAIReview:
		return a.placeReview(run)
	}
	s.add(logEntry{kind: logInfo, text: i18n.T("agent.donereview")})
	a.infoText(i18n.T("agent.workdone"))
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
	if a.gd.on {
		return a.reloadChanges()
	}
	return a.showDiff(baseHead, nil)
}
