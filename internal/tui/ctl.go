package tui

import (
	"path/filepath"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/ctl"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// ControlHandler adapts the control socket to a running program: each request
// is delivered to Update as a CtlRequest and the reply is awaited.
func ControlHandler(p *tea.Program) ctl.Handler {
	return func(req ctl.Request) ctl.Response {
		reply := make(chan ctl.Response, 1)
		p.Send(CtlRequest{Req: req, Reply: reply})
		return <-reply
	}
}

// handleCtl runs a control-socket request. The answer goes out as a ReplyCtl
// Effect, followed by whatever the command itself asked for.
func (a *App) handleCtl(m CtlRequest) []Effect {
	var cmd []Effect
	var resp ctl.Response
	switch m.Req.Cmd {
	case "diff":
		cmd = a.diffWithHead()
		resp = ctl.Response{OK: true}
		if m.Req.Path != "" {
			if !a.openFile(m.Req.Path) {
				resp = ctl.Response{Error: i18n.T("ctl.cantopen") + m.Req.Path}
				break
			}
			cmd = batch(cmd, a.setFocus(focusEditor))
		}
	case "exec":
		resp, cmd = a.ctlExec(m.Req.Line)
	case "command-end":
		if code, err := strconv.Atoi(strings.TrimSpace(m.Req.Line)); err != nil {
			resp = ctl.Response{Error: i18n.T("ctl.usage.commandend")}
		} else {
			a.commandEnded(code)
			resp = ctl.Response{OK: true}
		}
	default:
		resp = ctl.Response{Error: i18n.T("ctl.unknowncmd") + m.Req.Cmd}
	}
	return append([]Effect{ReplyCtl{To: m.Reply, Resp: resp}}, cmd...)
}

// reviewToRoot turns a repository-relative review path into one relative to
// the editor root, which is the terminal's working directory.
func (a *App) reviewToRoot(p string) string {
	base := a.gitTop
	if base == "" {
		base = a.root
	}
	return a.rel(filepath.Join(base, p))
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
// The command is the same one a person types; failures that execCommand only
// shows in the status line are turned into the reply.
func (a *App) ctlExec(line string) (ctl.Response, []Effect) {
	line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), ":"))
	if line == "" {
		return ctl.Response{Error: i18n.T("ctl.emptycmd")}, nil
	}
	name, arg, _ := strings.Cut(line, " ")
	arg = strings.TrimSpace(arg)
	if agentForbidden[name] {
		return ctl.Response{Error: ":" + name + i18n.T("ctl.noagent")}, nil
	}
	switch name {
	case "reload", "rl", "bc", "bclose", "buffer-close":
		if t := a.curText(); t != nil && t.dirty() {
			return ctl.Response{Error: i18n.T("ctl.unsaved1") + name + i18n.T("ctl.unsaved2")}, nil
		}
	}
	if agentNeedsHuman(name, arg) {
		return ctl.Response{Error: ":" + name + i18n.T("ctl.needarg")}, nil
	}
	a.msg, a.msgErr = "", false
	cmd := a.execCommand(line)
	// Only a key press clears this, so it must not carry over to a person.
	a.confirmClose = false
	if a.pal.open || a.cmdOpen {
		a.pal.close()
		a.closeCmdline()
		return ctl.Response{Error: ":" + name + i18n.T("ctl.needhuman")}, cmd
	}
	if a.msgErr {
		return ctl.Response{Error: a.msg}, cmd
	}
	return ctl.Response{OK: true, Message: a.msg}, cmd
}
