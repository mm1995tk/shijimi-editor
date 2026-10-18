package ui

import (
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mm1995tk/gh-editor/internal/ctl"
)

// ctlMsg carries a control-socket request into the Bubble Tea loop.
type ctlMsg struct {
	req   ctl.Request
	reply chan ctl.Response
}

// ControlHandler adapts the control socket to a running program: each request
// is delivered to Update and the reply is awaited.
func ControlHandler(p *tea.Program) ctl.Handler {
	return func(req ctl.Request) ctl.Response {
		reply := make(chan ctl.Response, 1)
		p.Send(ctlMsg{req: req, reply: reply})
		return <-reply
	}
}

func (a *App) handleCtl(m ctlMsg) tea.Cmd {
	var cmd tea.Cmd
	var resp ctl.Response
	switch m.req.Cmd {
	case "diff":
		if !a.gd.on {
			cmd = a.toggleGitDiff()
		} else {
			cmd = a.reloadChanges()
		}
		resp = ctl.Response{OK: true}
		if m.req.Path != "" {
			if !a.openFile(m.req.Path) {
				resp = ctl.Response{Error: "開けませんでした: " + m.req.Path}
				break
			}
			cmd = tea.Batch(cmd, a.setFocus(focusEditor))
		}
	case "exec":
		resp, cmd = a.ctlExec(m.req.Line)
	default:
		resp = ctl.Response{Error: "不明なコマンド: " + m.req.Cmd}
	}
	m.reply <- resp
	return cmd
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
func (a *App) ctlExec(line string) (ctl.Response, tea.Cmd) {
	line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), ":"))
	if line == "" {
		return ctl.Response{Error: "コマンドが空です"}, nil
	}
	name, arg, _ := strings.Cut(line, " ")
	arg = strings.TrimSpace(arg)
	if agentForbidden[name] {
		return ctl.Response{Error: ":" + name + " はエージェントからは実行できません"}, nil
	}
	switch name {
	case "reload", "rl", "bc", "bclose", "buffer-close":
		if t := a.curText(); t != nil && t.dirty() {
			return ctl.Response{Error: "未保存の変更があるため :" + name + " は実行できません（先に :w で保存）"}, nil
		}
	}
	if agentNeedsHuman(name, arg) {
		return ctl.Response{Error: ":" + name + " は引数なしだと人が選ぶ画面になります。引数を付けてください"}, nil
	}
	a.msg, a.msgErr = "", false
	cmd := a.execCommand(line)
	// Only a key press clears this, so it must not carry over to a person.
	a.confirmClose = false
	if a.pal.open || a.cmdOpen {
		a.pal.close()
		a.closeCmdline()
		return ctl.Response{Error: ":" + name + " は人の入力待ちになるため実行できません"}, cmd
	}
	if a.msgErr {
		return ctl.Response{Error: a.msg}, cmd
	}
	return ctl.Response{OK: true, Message: a.msg}, cmd
}
