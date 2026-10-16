package ui

import (
	"fmt"
	"path/filepath"

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
	case "open":
		if m.req.Path == "" {
			resp = ctl.Response{Error: "path がありません"}
			break
		}
		if !a.openFile(m.req.Path) {
			resp = ctl.Response{Error: "開けませんでした: " + m.req.Path}
			break
		}
		if t := a.curText(); t != nil && m.req.Line > 0 {
			t.buf.GotoLine(m.req.Line)
		}
		a.follow = true
		cmd = a.setFocus(focusEditor)
		resp = ctl.Response{OK: true}
	case "diff":
		cmd = a.openLocalReview()
		if m.req.Path != "" {
			if t := a.tabs[a.active]; t.kind == tabReview {
				t.rev.want = a.reviewPath(m.req.Path)
				t.rev.selectWanted()
			}
		}
		resp = ctl.Response{OK: true}
	case "status":
		resp = ctl.Response{OK: true, Status: a.ctlStatus()}
	default:
		resp = ctl.Response{Error: "不明なコマンド: " + m.req.Cmd}
	}
	m.reply <- resp
	return cmd
}

// reviewPath converts an absolute path to the repository-relative form used
// by diff files.
func (a *App) reviewPath(abs string) string {
	base := a.gitTop
	if base == "" {
		base = a.root
	}
	if rel, err := filepath.Rel(base, abs); err == nil {
		return filepath.ToSlash(rel)
	}
	return abs
}

func (a *App) ctlStatus() *ctl.Status {
	st := &ctl.Status{Root: a.root, Branch: a.branch, Tabs: []string{}}
	for _, t := range a.tabs {
		if t.kind == tabText {
			st.Tabs = append(st.Tabs, t.path())
		} else {
			st.Tabs = append(st.Tabs, fmt.Sprintf("(%s)", t.title()))
		}
	}
	if t := a.curText(); t != nil {
		st.Active = t.path()
		st.Line = t.buf.Cur.Row + 1
	}
	return st
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
