package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/ctl"
)

// ControlHandler adapts the control socket to a running program.
func ControlHandler(p *tea.Program) ctl.Handler {
	return core.ControlHandler(func(m any) { p.Send(m) })
}
