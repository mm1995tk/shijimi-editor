package core

import "github.com/mm1995tk/shijimi-editor/internal/proxy"

// Shared is what the whole process has one of, whatever number of cores it
// runs: the port forwards (docs/design/gui-protocol.md, 1 章の表の `proxy`).
// The TUI makes one core and with it one Shared (New); `shj serve` makes one
// Shared and a core per workspace on it (Workspaces).
//
// The theme and the UI language are process-wide too, but they are set where
// they always were: the theme by the front end (FrontEnd.SetPalette) and the
// language by i18n.SetLang when a core reads its config.
type Shared struct {
	Proxy *proxy.Manager
}

// NewShared makes the process-wide parts.
func NewShared() *Shared {
	return &Shared{Proxy: proxy.NewManager()}
}

// Close stops what the process holds for all its cores: the port forwards and
// the shell integration files.
func (s *Shared) Close() {
	s.Proxy.StopAll()
	RemoveShellIntegration()
}
