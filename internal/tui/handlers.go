package tui

import (
	"context"
	"encoding/json"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/ctl"
	"github.com/mm1995tk/shijimi-editor/internal/ide"
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

// IDEHandler adapts the MCP server to a running program: each call is delivered
// to Update as a core.IDECall and the reply is awaited. Most calls are answered at
// once; openDiff is answered when the user decides.
func IDEHandler(p *tea.Program) ide.Handler {
	return func(ctx context.Context, tool string, args json.RawMessage) ([]ide.Content, error) {
		reply := make(chan core.IDEReply, 1)
		p.Send(core.IDECall{Tool: tool, Args: args, Reply: reply})
		select {
		case r := <-reply:
			return r.Content, r.Err
		case <-ctx.Done():
			p.Send(core.IDECancel{Reply: reply})
			return nil, ctx.Err()
		}
	}
}
