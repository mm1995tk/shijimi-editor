package core

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mm1995tk/shijimi-editor/core/ctl"
	"github.com/mm1995tk/shijimi-editor/core/msg"
)

// handleCtl runs a control-socket request. The answer goes out as a ReplyCtl
// Effect, followed by whatever the command itself asked for.
func (a *Core) handleCtl(m CtlRequest) []Effect {
	var cmd []Effect
	var resp CtlResponse
	switch m.Req.Cmd {
	case "diff":
		cmd = a.diffWithHead()
		resp = CtlResponse{OK: true}
		if m.Req.Path != "" {
			if !a.OpenFile(m.Req.Path) {
				resp = CtlResponse{Error: msg.T("ctl.cantopen").Add(m.Req.Path)}
				break
			}
			cmd = Batch(cmd, a.SetFocus(FocusEditor))
		}
	case "exec":
		resp, cmd = a.ctlExec(m.Req.Line)
	case "command-end":
		if code, err := strconv.Atoi(strings.TrimSpace(m.Req.Line)); err != nil {
			resp = CtlResponse{Error: msg.T("ctl.usage.commandend")}
		} else {
			a.port.CommandEnded(code)
			resp = CtlResponse{OK: true}
		}
	default:
		resp = CtlResponse{Error: msg.T("ctl.unknowncmd").Add(m.Req.Cmd)}
	}
	return append([]Effect{ReplyCtl{To: m.Reply, Resp: resp}}, cmd...)
}

// ReviewToRoot turns a repository-relative review path into one relative to
// the editor root, which is the terminal's working directory.
func (a *Core) ReviewToRoot(p string) string {
	base := a.GitTop
	if base == "" {
		base = a.Root
	}
	return a.Rel(filepath.Join(base, p))
}

// agentForbidden lists ":" commands that an agent may not run: quitting would
// end the agent's own session, and the "!" forms throw away unsaved work.
var agentForbidden = map[string]bool{
	"q": true, "quit": true, "qa": true, "quit-all": true,
	"q!": true, "quit!": true, "qa!": true, "quit-all!": true,
	"wq": true, "x": true, "write-quit": true, "x!": true, "wq!": true,
	"wqa": true, "xa": true, "write-quit-all": true,
	"bc!": true, "bclose!": true, "buffer-close!": true,
}

// agentNeedsHuman reports whether the command, written like this, would stop
// at a picker that only a person can answer.
func agentNeedsHuman(name, arg string) bool {
	switch name {
	case "o", "open", "e", "edit", "theme", "grep", "rg", "replace", "rep":
		return arg == ""
	case "pr":
		_, err := strconv.Atoi(strings.TrimPrefix(arg, "#"))
		return err != nil
	case "symbol", "sym", "symbol-all", "sym-all", "action", "code-action", "ca":
		return true
	}
	return false
}

// ctlExec runs a ":" command for the control socket and reports how it went.
// The command is the same one a person types; failures that ExecCommand only
// shows in the status line are turned into the reply.
func (a *Core) ctlExec(line string) (CtlResponse, []Effect) {
	line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), ":"))
	if line == "" {
		return CtlResponse{Error: msg.T("ctl.emptycmd")}, nil
	}
	name, arg, _ := strings.Cut(line, " ")
	arg = strings.TrimSpace(arg)
	if agentForbidden[name] {
		return CtlResponse{Error: msg.Cat(":", name, msg.T("ctl.noagent"))}, nil
	}
	switch name {
	case "reload", "rl", "bc", "bclose", "buffer-close":
		if t := a.CurText(); t != nil && t.dirty() {
			return CtlResponse{Error: msg.Cat(msg.T("ctl.unsaved1"), name, msg.T("ctl.unsaved2"))}, nil
		}
	}
	if agentNeedsHuman(name, arg) {
		return CtlResponse{Error: msg.Cat(":", name, msg.T("ctl.needarg"))}, nil
	}
	a.Msg, a.MsgErr = msg.Msg{}, false
	cmd := a.ExecCommand(line)
	// Only a key press clears this, so it must not carry over to a person.
	a.confirmClose = false
	if a.Pal.Open || a.CmdOpen {
		a.Pal.Close()
		a.closeCmdline()
		return CtlResponse{Error: msg.Cat(":", name, msg.T("ctl.needhuman"))}, cmd
	}
	if a.MsgErr {
		return CtlResponse{Error: a.Msg}, cmd
	}
	return CtlResponse{OK: true, Message: a.Msg}, cmd
}

// CtlResponse is the answer to a control-socket request. Its texts are
// messages; the adapter of the socket turns them into text with Wire.
type CtlResponse struct {
	OK      bool
	Error   msg.Msg
	Message msg.Msg // exec: the status line after the command
}

// Wire is r as the control socket carries it, with text translated by text.
func (r CtlResponse) Wire(text func(msg.Msg) string) ctl.Response {
	return ctl.Response{OK: r.OK, Error: text(r.Error), Message: text(r.Message)}
}

// ControlHandler adapts the control socket to a running front end: each
// request is sent to the front end's Update loop (through send) as a
// CtlRequest and the reply is awaited; text turns its messages into the UI's language.
func ControlHandler(send func(any), text func(msg.Msg) string) ctl.Handler {
	return func(req ctl.Request) ctl.Response {
		reply := make(chan CtlResponse, 1)
		send(CtlRequest{Req: req, Reply: reply})
		return (<-reply).Wire(text)
	}
}
