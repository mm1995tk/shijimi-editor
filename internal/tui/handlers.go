package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/ctl"
)

// ControlHandler adapts the control socket to a running program: each request
// is delivered to Update as a core.CtlRequest and the reply is awaited.
func ControlHandler(p *tea.Program) ctl.Handler {
	return func(req ctl.Request) ctl.Response {
		reply := make(chan ctl.Response, 1)
		p.Send(core.CtlRequest{Req: req, Reply: reply})
		return <-reply
	}
}
