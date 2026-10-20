// Package acp is a minimal Agent Client Protocol client
// (https://agentclientprotocol.com). The editor starts an agent as a
// subprocess and talks JSON-RPC 2.0 over its stdin and stdout, one message
// per line. The agent asks the editor to read and write files and for
// permission to run tools; the editor sends prompts and receives progress.
package acp

import (
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ProtocolVersion is the ACP major version this client speaks.
const ProtocolVersion = 1

// ContentBlock is a piece of a prompt or of a message. Only text is used.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Text returns a text content block.
func Text(s string) ContentBlock { return ContentBlock{Type: "text", Text: s} }

// PlanEntry is one step of the agent's plan.
type PlanEntry struct {
	Content  string `json:"content"`
	Priority string `json:"priority"`
	Status   string `json:"status"` // pending, in_progress, completed
}

// Location is a file a tool call works on.
type Location struct {
	Path string `json:"path"`
	Line *int   `json:"line,omitempty"`
}

// Update is the payload of a session/update notification. Kind tells which
// fields are set: agent_message_chunk, agent_thought_chunk and
// user_message_chunk carry Content (a content block); tool_call and
// tool_call_update carry the tool call fields; plan carries Entries.
type Update struct {
	Kind       string          `json:"sessionUpdate"`
	Content    json.RawMessage `json:"content,omitempty"`
	ToolCallID string          `json:"toolCallId,omitempty"`
	Title      string          `json:"title,omitempty"`
	ToolKind   string          `json:"kind,omitempty"`
	Status     string          `json:"status,omitempty"` // pending, in_progress, completed, failed
	Locations  []Location      `json:"locations,omitempty"`
	Entries    []PlanEntry     `json:"entries,omitempty"`
}

// Text returns the text of a message chunk ("" for other updates).
func (u Update) Text() string {
	var b ContentBlock
	if json.Unmarshal(u.Content, &b) != nil || b.Type != "text" {
		return ""
	}
	return b.Text
}

// ToolCall describes the tool an agent asks permission for.
type ToolCall struct {
	ToolCallID string     `json:"toolCallId"`
	Title      string     `json:"title,omitempty"`
	Kind       string     `json:"kind,omitempty"`
	Locations  []Location `json:"locations,omitempty"`
}

// PermissionOption is one answer the user can give to a permission request.
type PermissionOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"` // allow_once, allow_always, reject_once, reject_always
}

// PermissionRequest is session/request_permission.
type PermissionRequest struct {
	SessionID string             `json:"sessionId"`
	ToolCall  ToolCall           `json:"toolCall"`
	Options   []PermissionOption `json:"options"`
}

// ReadRequest is fs/read_text_file. Line is 1-based; Line and Limit are 0
// when absent.
type ReadRequest struct {
	SessionID string `json:"sessionId"`
	Path      string `json:"path"`
	Line      int    `json:"line,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

// WriteRequest is fs/write_text_file.
type WriteRequest struct {
	SessionID string `json:"sessionId"`
	Path      string `json:"path"`
	Content   string `json:"content"`
}

// Handler answers what the agent asks of the editor. The methods are called
// from the client's goroutines; requests may block until the user decides.
type Handler interface {
	ReadTextFile(ctx context.Context, r ReadRequest) (string, error)
	WriteTextFile(ctx context.Context, r WriteRequest) error
	// RequestPermission returns the chosen option, or "" when the request
	// was cancelled.
	RequestPermission(ctx context.Context, r PermissionRequest) (string, error)
	SessionUpdate(sessionID string, u Update)
}

// Error is a JSON-RPC error returned by the agent.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return fmt.Sprintf("%s (%d)", e.Message, e.Code) }

// codeAuthRequired is the error session/new returns before authenticate.
const codeAuthRequired = -32000

type message struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *Error           `json:"error,omitempty"`
}

type response struct {
	result json.RawMessage
	err    error
}

// Options says how to start the agent.
type Options struct {
	Command string
	Args    []string
	Env     map[string]string
	Dir     string
	// Version is the editor's version, sent as clientInfo.
	Version string
}

// AuthMethod is a way to sign in that the agent offers.
type AuthMethod struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Info is what the agent says about itself in initialize.
type Info struct {
	ProtocolVersion int          `json:"protocolVersion"`
	AuthMethods     []AuthMethod `json:"authMethods"`
	AgentInfo       *struct {
		Name    string `json:"name"`
		Title   string `json:"title"`
		Version string `json:"version"`
	} `json:"agentInfo"`
}

// Name is the agent's name for display, "" if it gave none.
func (i Info) Name() string {
	if i.AgentInfo == nil {
		return ""
	}
	if i.AgentInfo.Title != "" {
		return i.AgentInfo.Title
	}
	return i.AgentInfo.Name
}

// Client talks to one agent process.
type Client struct {
	cmd     *exec.Cmd
	version string
	stdin   io.WriteCloser
	handler Handler

	wmu    sync.Mutex
	mu     sync.Mutex
	nextID int
	wait   map[int]chan response
	info   Info

	stderr tail
	dead   chan struct{}
	err    error // why the process ended; read after dead is closed
}

// Start launches the agent. Call Initialize before anything else.
func Start(o Options, h Handler) (*Client, error) {
	if o.Command == "" {
		return nil, errors.New("エージェントのコマンドが設定されていません")
	}
	path, err := exec.LookPath(o.Command)
	if err != nil {
		return nil, fmt.Errorf("%s が見つかりません", o.Command)
	}
	cmd := exec.Command(path, o.Args...)
	cmd.Dir = o.Dir
	cmd.Env = os.Environ()
	for k, v := range o.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	c := &Client{cmd: cmd, version: o.Version, stdin: stdin, handler: h, wait: map[int]chan response{}, dead: make(chan struct{})}
	cmd.Stderr = &c.stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go c.readLoop(stdout)
	return c, nil
}

// Done is closed when the agent process has ended.
func (c *Client) Done() <-chan struct{} { return c.dead }

// Err says why the process ended. Valid once Done is closed.
func (c *Client) Err() error { return c.err }

// Stderr returns the last lines the agent wrote to its standard error.
func (c *Client) Stderr() string { return c.stderr.String() }

// Info returns what the agent said in initialize.
func (c *Client) Info() Info { return c.info }

// Close stops the agent.
func (c *Client) Close() {
	c.stdin.Close()
	if c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}
	select {
	case <-c.dead:
	case <-time.After(2 * time.Second): // a child still holds the pipe
	}
}

// Initialize negotiates the protocol and tells the agent that the editor
// reads and writes files for it.
func (c *Client) Initialize(ctx context.Context) error {
	params := map[string]any{
		"protocolVersion": ProtocolVersion,
		"clientCapabilities": map[string]any{
			"fs":       map[string]any{"readTextFile": true, "writeTextFile": true},
			"terminal": false,
		},
		"clientInfo": map[string]any{"name": "shijimi", "title": "shijimi", "version": cmp.Or(c.version, "dev")},
	}
	var info Info
	if err := c.call(ctx, "initialize", params, &info); err != nil {
		return err
	}
	c.info = info
	return nil
}

// NewSession starts a conversation in directory cwd and returns its id. An
// agent that asks to sign in first is sent its first auth method once.
func (c *Client) NewSession(ctx context.Context, cwd string) (string, error) {
	params := map[string]any{"cwd": cwd, "mcpServers": []any{}}
	var res struct {
		SessionID string `json:"sessionId"`
	}
	err := c.call(ctx, "session/new", params, &res)
	var rpc *Error
	if errors.As(err, &rpc) && rpc.Code == codeAuthRequired && len(c.info.AuthMethods) > 0 {
		if err := c.call(ctx, "authenticate", map[string]any{"methodId": c.info.AuthMethods[0].ID}, nil); err != nil {
			return "", err
		}
		err = c.call(ctx, "session/new", params, &res)
	}
	if err != nil {
		return "", err
	}
	return res.SessionID, nil
}

// Prompt sends a user turn and waits until the agent ends it. Progress
// arrives through Handler.SessionUpdate before Prompt returns.
func (c *Client) Prompt(ctx context.Context, sessionID string, blocks ...ContentBlock) (stopReason string, err error) {
	var res struct {
		StopReason string `json:"stopReason"`
	}
	err = c.call(ctx, "session/prompt", map[string]any{"sessionId": sessionID, "prompt": blocks}, &res)
	return res.StopReason, err
}

// Cancel asks the agent to stop the running turn; Prompt then returns with
// stop reason "cancelled".
func (c *Client) Cancel(sessionID string) error {
	return c.send(message{Method: "session/cancel", Params: mustJSON(map[string]any{"sessionId": sessionID})})
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func (c *Client) send(m message) error {
	m.JSONRPC = "2.0"
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_, err = c.stdin.Write(append(b, '\n'))
	return err
}

func (c *Client) call(ctx context.Context, method string, params, out any) error {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	ch := make(chan response, 1)
	c.wait[id] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.wait, id)
		c.mu.Unlock()
	}()
	raw := json.RawMessage(fmt.Sprint(id))
	if err := c.send(message{ID: &raw, Method: method, Params: mustJSON(params)}); err != nil {
		return err
	}
	select {
	case r := <-ch:
		if r.err != nil {
			return r.err
		}
		if out != nil && len(r.result) > 0 && string(r.result) != "null" {
			return json.Unmarshal(r.result, out)
		}
		return nil
	case <-c.dead:
		return c.exitError()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) exitError() error {
	msg := "エージェントが終了しました"
	if s := strings.TrimSpace(c.Stderr()); s != "" {
		msg += ": " + lastLine(s)
	}
	return errors.New(msg)
}

func lastLine(s string) string {
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return s[i+1:]
	}
	return s
}

// maxLine bounds one message from the agent (a whole file may be written).
const maxLine = 64 << 20

func (c *Client) readLoop(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), maxLine)
	ctx, cancel := context.WithCancel(context.Background())
	for sc.Scan() {
		line := sc.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var m message
		if err := json.Unmarshal(line, &m); err != nil {
			continue // not a message (some agents log to stdout)
		}
		switch {
		case m.Method != "" && m.ID != nil:
			// Requests may wait for the user, so each runs on its own.
			go c.serve(ctx, m)
		case m.Method != "":
			c.notify(m)
		case m.ID != nil:
			var id int
			if json.Unmarshal(*m.ID, &id) != nil {
				continue
			}
			c.mu.Lock()
			ch := c.wait[id]
			c.mu.Unlock()
			if ch == nil {
				continue
			}
			if m.Error != nil {
				ch <- response{err: m.Error}
			} else {
				ch <- response{result: m.Result}
			}
		}
	}
	cancel()
	c.err = c.cmd.Wait()
	close(c.dead)
}

func (c *Client) notify(m message) {
	if m.Method != "session/update" {
		return
	}
	var p struct {
		SessionID string `json:"sessionId"`
		Update    Update `json:"update"`
	}
	if json.Unmarshal(m.Params, &p) == nil {
		c.handler.SessionUpdate(p.SessionID, p.Update)
	}
}

func (c *Client) serve(ctx context.Context, m message) {
	result, err := c.dispatch(ctx, m)
	reply := message{ID: m.ID}
	if err != nil {
		var rpc *Error
		if !errors.As(err, &rpc) {
			rpc = &Error{Code: -32603, Message: err.Error()}
		}
		reply.Error = rpc
	} else {
		reply.Result = mustJSON(result)
	}
	c.send(reply)
}

func (c *Client) dispatch(ctx context.Context, m message) (any, error) {
	switch m.Method {
	case "fs/read_text_file":
		var r ReadRequest
		if err := json.Unmarshal(m.Params, &r); err != nil {
			return nil, &Error{Code: -32602, Message: err.Error()}
		}
		text, err := c.handler.ReadTextFile(ctx, r)
		if err != nil {
			return nil, err
		}
		return map[string]any{"content": text}, nil
	case "fs/write_text_file":
		var r WriteRequest
		if err := json.Unmarshal(m.Params, &r); err != nil {
			return nil, &Error{Code: -32602, Message: err.Error()}
		}
		return nil, c.handler.WriteTextFile(ctx, r)
	case "session/request_permission":
		var r PermissionRequest
		if err := json.Unmarshal(m.Params, &r); err != nil {
			return nil, &Error{Code: -32602, Message: err.Error()}
		}
		id, err := c.handler.RequestPermission(ctx, r)
		if err != nil {
			return nil, err
		}
		if id == "" {
			return map[string]any{"outcome": map[string]any{"outcome": "cancelled"}}, nil
		}
		return map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": id}}, nil
	}
	return nil, &Error{Code: -32601, Message: "method not found: " + m.Method}
}

// tail keeps the last few KB written to it.
type tail struct {
	mu  sync.Mutex
	buf []byte
}

const tailMax = 4096

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > tailMax {
		t.buf = t.buf[len(t.buf)-tailMax:]
	}
	return len(p), nil
}

func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}
