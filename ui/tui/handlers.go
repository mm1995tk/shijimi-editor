package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/core"
	"github.com/mm1995tk/shijimi-editor/core/ctl"
	"github.com/mm1995tk/shijimi-editor/ui/internal/i18n"
)

// ControlHandler adapts the control socket to a running program.
func ControlHandler(p *tea.Program) ctl.Handler {
	return core.ControlHandler(func(m any) { p.Send(m) }, i18n.Text)
}
