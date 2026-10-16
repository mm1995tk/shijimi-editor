// Package lsp is a minimal Language Server Protocol client.
package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/mm1995tk/gh-editor/internal/config"
)

// Position is an LSP position (UTF-16 character offset).
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range is an LSP range.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location is an LSP location.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// Diagnostic is an LSP diagnostic.
type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Message  string `json:"message"`
	Source   string `json:"source"`
}

// TextEdit is an LSP text edit.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// CompletionItem is the subset of an LSP completion item we use.
type CompletionItem struct {
	Label      string    `json:"label"`
	Detail     string    `json:"detail"`
	Kind       int       `json:"kind"`
	InsertText string    `json:"insertText"`
	FilterText string    `json:"filterText"`
	SortText   string    `json:"sortText"`
	TextEdit   *TextEdit `json:"textEdit"`
	// InsertTextFormat 2 means snippet syntax.
	InsertTextFormat int `json:"insertTextFormat"`
}

// WorkspaceEdit maps file URIs to edits.
type WorkspaceEdit map[string][]TextEdit

// Event is an asynchronous notification from a server.
type Event struct {
	Server      string
	URI         string
	Diagnostics []Diagnostic
	Message     string
	Err         error
}

type response struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Client talks to one language server process.
type Client struct {
	Name   string
	Root   string
	server *config.Server

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	wmu    sync.Mutex
	mu     sync.Mutex
	nextID int
	wait   map[int]chan response
	events chan<- Event
	caps   map[string]any
	ready  chan struct{}
	dead   chan struct{}

	completionTriggers string
}

// Start launches a server and performs the initialize handshake in the
// background. Events (diagnostics, messages) are sent to events.
func Start(s *config.Server, root string, events chan<- Event) (*Client, error) {
	if s.Command == "" {
		return nil, fmt.Errorf("%s: command が設定されていません", s.Name)
	}
	path, err := exec.LookPath(s.Command)
	if err != nil {
		return nil, fmt.Errorf("%s が見つかりません", s.Command)
	}
	cmd := exec.Command(path, s.Args...)
	cmd.Dir = root
	cmd.Env = os.Environ()
	for k, v := range s.Env {
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
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	c := &Client{
		Name: s.Name, Root: root, server: s, cmd: cmd, stdin: stdin,
		wait: map[int]chan response{}, events: events,
		ready: make(chan struct{}), dead: make(chan struct{}),
	}
	go c.readLoop(bufio.NewReader(stdout))
	go func() {
		cmd.Wait()
		close(c.dead)
	}()
	go c.initialize()
	return c, nil
}

func (c *Client) initialize() {
	params := map[string]any{
		"processId": os.Getpid(),
		"rootUri":   URI(c.Root),
		"rootPath":  c.Root,
		"workspaceFolders": []map[string]any{
			{"uri": URI(c.Root), "name": filepath.Base(c.Root)},
		},
		"initializationOptions": c.server.Config,
		"capabilities": map[string]any{
			"general": map[string]any{"positionEncodings": []string{"utf-16"}},
			"workspace": map[string]any{
				"configuration":    true,
				"workspaceFolders": true,
				"applyEdit":        false,
			},
			"textDocument": map[string]any{
				"synchronization": map[string]any{"didSave": true},
				"hover":           map[string]any{"contentFormat": []string{"plaintext", "markdown"}},
				"completion": map[string]any{
					"completionItem": map[string]any{"snippetSupport": false},
				},
				"definition":         map[string]any{},
				"references":         map[string]any{},
				"rename":             map[string]any{},
				"formatting":         map[string]any{},
				"publishDiagnostics": map[string]any{},
			},
		},
	}
	var res struct {
		Capabilities map[string]any `json:"capabilities"`
	}
	if err := c.Call(context.Background(), "initialize", params, &res); err != nil {
		c.events <- Event{Server: c.Name, Err: fmt.Errorf("%s の初期化に失敗: %w", c.Name, err)}
		return
	}
	c.caps = res.Capabilities
	if cp, ok := c.caps["completionProvider"].(map[string]any); ok {
		if tc, ok := cp["triggerCharacters"].([]any); ok {
			for _, t := range tc {
				if s, ok := t.(string); ok {
					c.completionTriggers += s
				}
			}
		}
	}
	c.Notify("initialized", map[string]any{})
	if c.server.Config != nil {
		c.Notify("workspace/didChangeConfiguration", map[string]any{"settings": c.server.Config})
	}
	close(c.ready)
}

// IsCompletionTrigger reports whether typing r should open completion.
func (c *Client) IsCompletionTrigger(r rune) bool {
	select {
	case <-c.ready:
	default:
		return false
	}
	return strings.ContainsRune(c.completionTriggers, r)
}

// Supports reports whether the server advertised a capability, e.g.
// "hoverProvider".
func (c *Client) Supports(cap string) bool {
	select {
	case <-c.ready:
	default:
		return false
	}
	v, ok := c.caps[cap]
	if !ok || v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return true
}

// Ready is closed once the server is initialized.
func (c *Client) Ready() <-chan struct{} { return c.ready }

// Stop shuts the server down.
func (c *Client) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = c.Call(ctx, "shutdown", nil, nil)
	c.Notify("exit", nil)
	select {
	case <-c.dead:
	case <-time.After(time.Second):
		c.cmd.Process.Kill()
	}
}

func (c *Client) write(msg map[string]any) error {
	msg["jsonrpc"] = "2.0"
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if _, err := fmt.Fprintf(c.stdin, "Content-Length: %d\r\n\r\n", len(data)); err != nil {
		return err
	}
	_, err = c.stdin.Write(data)
	return err
}

// Notify sends a notification.
func (c *Client) Notify(method string, params any) {
	m := map[string]any{"method": method}
	if params != nil {
		m["params"] = params
	}
	_ = c.write(m)
}

// Call sends a request and decodes the result into out (if non-nil).
func (c *Client) Call(ctx context.Context, method string, params any, out any) error {
	if method != "initialize" && method != "shutdown" {
		select {
		case <-c.ready:
		case <-ctx.Done():
			return ctx.Err()
		case <-c.dead:
			return errors.New("サーバーが終了しました")
		}
	}
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
	m := map[string]any{"id": id, "method": method}
	if params != nil {
		m["params"] = params
	}
	if err := c.write(m); err != nil {
		return err
	}
	select {
	case r := <-ch:
		if r.Error != nil {
			return errors.New(r.Error.Message)
		}
		if out != nil && len(r.Result) > 0 {
			return json.Unmarshal(r.Result, out)
		}
		return nil
	case <-ctx.Done():
		c.Notify("$/cancelRequest", map[string]any{"id": id})
		return ctx.Err()
	case <-c.dead:
		return errors.New("サーバーが終了しました")
	}
}

func (c *Client) readLoop(r *bufio.Reader) {
	for {
		length := 0
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			if v, ok := strings.CutPrefix(line, "Content-Length:"); ok {
				length, _ = strconv.Atoi(strings.TrimSpace(v))
			}
		}
		body := make([]byte, length)
		if _, err := io.ReadFull(r, body); err != nil {
			return
		}
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			response
		}
		if json.Unmarshal(body, &msg) != nil {
			continue
		}
		switch {
		case msg.Method != "" && len(msg.ID) > 0:
			c.handleRequest(msg.ID, msg.Method, msg.Params)
		case msg.Method != "":
			c.handleNotification(msg.Method, msg.Params)
		default:
			id, err := strconv.Atoi(string(msg.ID))
			if err != nil {
				continue
			}
			c.mu.Lock()
			ch := c.wait[id]
			c.mu.Unlock()
			if ch != nil {
				ch <- msg.response
			}
		}
	}
}

func (c *Client) handleRequest(id json.RawMessage, method string, params json.RawMessage) {
	var result any
	if method == "workspace/configuration" {
		var p struct {
			Items []struct {
				Section string `json:"section"`
			} `json:"items"`
		}
		json.Unmarshal(params, &p)
		res := make([]any, len(p.Items))
		for i, it := range p.Items {
			res[i] = lookupSection(c.server.Config, it.Section)
		}
		result = res
	}
	if method == "workspace/workspaceFolders" {
		result = []map[string]any{{"uri": URI(c.Root), "name": filepath.Base(c.Root)}}
	}
	_ = c.write(map[string]any{"id": id, "result": result})
}

func lookupSection(cfg any, section string) any {
	if section == "" {
		return cfg
	}
	cur := cfg
	for _, part := range strings.Split(section, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[part]
	}
	return cur
}

func (c *Client) handleNotification(method string, params json.RawMessage) {
	switch method {
	case "textDocument/publishDiagnostics":
		var p struct {
			URI         string       `json:"uri"`
			Diagnostics []Diagnostic `json:"diagnostics"`
		}
		if json.Unmarshal(params, &p) == nil {
			c.events <- Event{Server: c.Name, URI: p.URI, Diagnostics: p.Diagnostics}
		}
	case "window/showMessage":
		var p struct {
			Type    int    `json:"type"`
			Message string `json:"message"`
		}
		if json.Unmarshal(params, &p) == nil && p.Type <= 2 {
			c.events <- Event{Server: c.Name, Message: p.Message}
		}
	}
}

// ---- document sync ----

// DidOpen notifies the server about an opened document.
func (c *Client) DidOpen(path, languageID, text string, version int) {
	go func() {
		<-c.ready
		c.Notify("textDocument/didOpen", map[string]any{
			"textDocument": map[string]any{"uri": URI(path), "languageId": languageID, "version": version, "text": text},
		})
	}()
}

// DidChange sends the full new text of a document.
func (c *Client) DidChange(path, text string, version int) {
	select {
	case <-c.ready:
	default:
		return
	}
	c.Notify("textDocument/didChange", map[string]any{
		"textDocument":   map[string]any{"uri": URI(path), "version": version},
		"contentChanges": []map[string]any{{"text": text}},
	})
}

// DidSave notifies the server that a document was saved.
func (c *Client) DidSave(path string) {
	c.Notify("textDocument/didSave", map[string]any{"textDocument": map[string]any{"uri": URI(path)}})
}

// DidClose notifies the server that a document was closed.
func (c *Client) DidClose(path string) {
	c.Notify("textDocument/didClose", map[string]any{"textDocument": map[string]any{"uri": URI(path)}})
}

// ---- requests ----

func docPos(path string, pos Position) map[string]any {
	return map[string]any{"textDocument": map[string]any{"uri": URI(path)}, "position": pos}
}

// Hover returns hover text at pos.
func (c *Client) Hover(ctx context.Context, path string, pos Position) (string, error) {
	var res struct {
		Contents json.RawMessage `json:"contents"`
	}
	if err := c.Call(ctx, "textDocument/hover", docPos(path, pos), &res); err != nil {
		return "", err
	}
	return hoverText(res.Contents), nil
}

func hoverText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var mc struct {
		Kind     string `json:"kind"`
		Language string `json:"language"`
		Value    string `json:"value"`
	}
	if json.Unmarshal(raw, &mc) == nil && mc.Value != "" {
		return mc.Value
	}
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil {
		var parts []string
		for _, a := range arr {
			if t := hoverText(a); t != "" {
				parts = append(parts, t)
			}
		}
		return strings.Join(parts, "\n\n")
	}
	return ""
}

// Definition returns the definition locations of the symbol at pos.
func (c *Client) Definition(ctx context.Context, path string, pos Position) ([]Location, error) {
	var raw json.RawMessage
	if err := c.Call(ctx, "textDocument/definition", docPos(path, pos), &raw); err != nil {
		return nil, err
	}
	return parseLocations(raw), nil
}

// References returns all references to the symbol at pos.
func (c *Client) References(ctx context.Context, path string, pos Position) ([]Location, error) {
	p := docPos(path, pos)
	p["context"] = map[string]any{"includeDeclaration": true}
	var raw json.RawMessage
	if err := c.Call(ctx, "textDocument/references", p, &raw); err != nil {
		return nil, err
	}
	return parseLocations(raw), nil
}

func parseLocations(raw json.RawMessage) []Location {
	var one Location
	if json.Unmarshal(raw, &one) == nil && one.URI != "" {
		return []Location{one}
	}
	var many []struct {
		Location
		TargetURI            string `json:"targetUri"`
		TargetSelectionRange Range  `json:"targetSelectionRange"`
	}
	json.Unmarshal(raw, &many)
	out := make([]Location, 0, len(many))
	for _, l := range many {
		if l.TargetURI != "" {
			out = append(out, Location{URI: l.TargetURI, Range: l.TargetSelectionRange})
		} else {
			out = append(out, l.Location)
		}
	}
	return out
}

// Completion returns completion items at pos.
func (c *Client) Completion(ctx context.Context, path string, pos Position) ([]CompletionItem, error) {
	var raw json.RawMessage
	if err := c.Call(ctx, "textDocument/completion", docPos(path, pos), &raw); err != nil {
		return nil, err
	}
	var list struct {
		Items []CompletionItem `json:"items"`
	}
	if json.Unmarshal(raw, &list) == nil && list.Items != nil {
		return list.Items, nil
	}
	var items []CompletionItem
	json.Unmarshal(raw, &items)
	return items, nil
}

// Rename returns the edits needed to rename the symbol at pos.
func (c *Client) Rename(ctx context.Context, path string, pos Position, newName string) (WorkspaceEdit, error) {
	p := docPos(path, pos)
	p["newName"] = newName
	var res struct {
		Changes         map[string][]TextEdit `json:"changes"`
		DocumentChanges []struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Edits []TextEdit `json:"edits"`
		} `json:"documentChanges"`
	}
	if err := c.Call(ctx, "textDocument/rename", p, &res); err != nil {
		return nil, err
	}
	we := WorkspaceEdit{}
	for uri, edits := range res.Changes {
		we[uri] = append(we[uri], edits...)
	}
	for _, dc := range res.DocumentChanges {
		if dc.TextDocument.URI != "" {
			we[dc.TextDocument.URI] = append(we[dc.TextDocument.URI], dc.Edits...)
		}
	}
	return we, nil
}

// Format returns formatting edits for a document.
func (c *Client) Format(ctx context.Context, path string, tabSize int, spaces bool) ([]TextEdit, error) {
	var edits []TextEdit
	err := c.Call(ctx, "textDocument/formatting", map[string]any{
		"textDocument": map[string]any{"uri": URI(path)},
		"options":      map[string]any{"tabSize": tabSize, "insertSpaces": spaces},
	}, &edits)
	return edits, err
}

// ---- conversions ----

// URI converts a path into a file:// URI.
func URI(path string) string {
	abs, _ := filepath.Abs(path)
	return (&url.URL{Scheme: "file", Path: abs}).String()
}

// Path converts a file:// URI into a path.
func Path(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return uri
	}
	return u.Path
}

// ToUTF16 converts a rune column into a UTF-16 offset.
func ToUTF16(line []rune, col int) int {
	n := 0
	for i := 0; i < col && i < len(line); i++ {
		n += len(utf16.Encode([]rune{line[i]}))
	}
	return n
}

// FromUTF16 converts a UTF-16 offset into a rune column.
func FromUTF16(line []rune, off int) int {
	n := 0
	for i, r := range line {
		if n >= off {
			return i
		}
		n += len(utf16.Encode([]rune{r}))
	}
	return len(line)
}
