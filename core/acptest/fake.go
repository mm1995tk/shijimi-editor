// Package acptest is a scripted ACP agent for tests. A test binary runs it
// as the agent process: TestMain calls Main, which serves stdin and stdout
// when the environment variable Env is set.
//
// For every prompt the agent replies "受け取りました" and then follows
// instructions written in the prompt, one per line:
//
//	READ:<path>   read the file through the client and echo its contents
//	WRITE:<path>  ask permission, then write "written by agent\n" to path
//	SAY:<text>    send text as a message chunk
//	WAIT          wait for session/cancel and end the turn as cancelled
//	HUGE:<n>      write a line of n bytes that is not a message
package acptest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Env marks a process started as the fake agent.
const Env = "SHIJIMI_FAKE_ACP_AGENT"

// Main serves the fake agent and exits when Env is set; otherwise it returns.
func Main() {
	if os.Getenv(Env) == "" {
		return
	}
	Serve(os.Stdin, os.Stdout)
	os.Exit(0)
}

type msg struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type agent struct {
	out     io.Writer
	wmu     sync.Mutex
	mu      sync.Mutex
	next    int
	wait    map[int]chan msg
	cancels chan struct{}
}

// Serve runs the fake agent until r ends.
func Serve(r io.Reader, w io.Writer) {
	a := &agent{out: w, wait: map[int]chan msg{}, cancels: make(chan struct{}, 8)}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 64<<20)
	for sc.Scan() {
		var m msg
		if json.Unmarshal(sc.Bytes(), &m) != nil {
			continue
		}
		switch {
		case m.Method == "session/cancel":
			a.cancels <- struct{}{}
		case m.Method != "" && m.ID != nil:
			go a.handle(m)
		case m.ID != nil:
			var id int
			json.Unmarshal(*m.ID, &id)
			a.mu.Lock()
			ch := a.wait[id]
			a.mu.Unlock()
			if ch != nil {
				ch <- m
			}
		}
	}
}

func (a *agent) send(m msg) {
	m.JSONRPC = "2.0"
	b, _ := json.Marshal(m)
	a.wmu.Lock()
	defer a.wmu.Unlock()
	a.out.Write(append(b, '\n'))
}

func raw(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func (a *agent) reply(m msg, result any) {
	a.send(msg{ID: m.ID, Result: raw(result)})
}

func (a *agent) call(method string, params any) msg {
	a.mu.Lock()
	a.next++
	id := a.next
	ch := make(chan msg, 1)
	a.wait[id] = ch
	a.mu.Unlock()
	rid := json.RawMessage(fmt.Sprint(id))
	a.send(msg{ID: &rid, Method: method, Params: raw(params)})
	return <-ch
}

func (a *agent) say(sid, text string) {
	a.send(msg{Method: "session/update", Params: raw(map[string]any{
		"sessionId": sid,
		"update":    map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": text}},
	})})
}

func (a *agent) handle(m msg) {
	switch m.Method {
	case "initialize":
		a.reply(m, map[string]any{"protocolVersion": 1, "agentCapabilities": map[string]any{}, "authMethods": []any{},
			"agentInfo": map[string]any{"name": "fake", "title": "Fake Agent", "version": "0"}})
	case "session/new":
		a.reply(m, map[string]any{"sessionId": "s1"})
	case "session/prompt":
		var p struct {
			SessionID string `json:"sessionId"`
			Prompt    []struct {
				Text string `json:"text"`
			} `json:"prompt"`
		}
		json.Unmarshal(m.Params, &p)
		var text strings.Builder
		for _, b := range p.Prompt {
			text.WriteString(b.Text)
		}
		a.say(p.SessionID, "受け取りました\n")
		stop := "end_turn"
		for line := range strings.SplitSeq(text.String(), "\n") {
			switch {
			case strings.HasPrefix(line, "READ:"):
				r := a.call("fs/read_text_file", map[string]any{"sessionId": p.SessionID, "path": strings.TrimPrefix(line, "READ:")})
				var res struct {
					Content string `json:"content"`
				}
				json.Unmarshal(r.Result, &res)
				if r.Error != nil {
					a.say(p.SessionID, "読めません: "+r.Error.Message+"\n")
				} else {
					a.say(p.SessionID, "内容: "+res.Content)
				}
			case strings.HasPrefix(line, "WRITE:"):
				path := strings.TrimPrefix(line, "WRITE:")
				a.send(msg{Method: "session/update", Params: raw(map[string]any{"sessionId": p.SessionID, "update": map[string]any{
					"sessionUpdate": "tool_call", "toolCallId": "t1", "title": "Edit " + path, "kind": "edit", "status": "pending"}})})
				r := a.call("session/request_permission", map[string]any{
					"sessionId": p.SessionID,
					"toolCall":  map[string]any{"toolCallId": "t1", "title": "Edit " + path, "kind": "edit"},
					"options": []map[string]any{
						{"optionId": "allow", "name": "Allow", "kind": "allow_once"},
						{"optionId": "reject", "name": "Reject", "kind": "reject_once"},
					},
				})
				var res struct {
					Outcome struct {
						Outcome  string `json:"outcome"`
						OptionID string `json:"optionId"`
					} `json:"outcome"`
				}
				json.Unmarshal(r.Result, &res)
				if res.Outcome.OptionID != "allow" {
					a.say(p.SessionID, "拒否されました\n")
					continue
				}
				w := a.call("fs/write_text_file", map[string]any{"sessionId": p.SessionID, "path": path, "content": "written by agent\n"})
				status := "completed"
				if w.Error != nil {
					status = "failed"
					a.say(p.SessionID, "書けません: "+w.Error.Message+"\n")
				}
				a.send(msg{Method: "session/update", Params: raw(map[string]any{"sessionId": p.SessionID, "update": map[string]any{
					"sessionUpdate": "tool_call_update", "toolCallId": "t1", "status": status}})})
			case strings.HasPrefix(line, "SAY:"):
				a.say(p.SessionID, strings.ReplaceAll(strings.TrimPrefix(line, "SAY:"), `\n`, "\n"))
			case strings.HasPrefix(line, "HUGE:"):
				n, _ := strconv.Atoi(strings.TrimPrefix(line, "HUGE:"))
				a.wmu.Lock()
				a.out.Write([]byte(strings.Repeat("x", n) + "\n"))
				a.wmu.Unlock()
			case line == "WAIT":
				<-a.cancels
				stop = "cancelled"
			}
		}
		a.reply(m, map[string]any{"stopReason": stop})
	default:
		a.send(msg{ID: m.ID, Error: &struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}{-32601, "method not found"}})
	}
}
