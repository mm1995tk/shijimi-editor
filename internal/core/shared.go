package core

import (
	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/proxy"
)

// Shared is what the whole process has one of, whatever number of cores it
// runs: the port forwards (docs/design/gui-protocol.md, 1 章の表の `proxy`).
// The TUI makes one core and with it one Shared (New); `shj serve` makes one
// Shared and a core per workspace on it (Workspaces).
//
// The theme and the UI language are process-wide too, but they are set where
// they always were: the theme by the UI (Port.SetPalette) and the
// language by i18n.SetLang when a core reads its config.
type Shared struct {
	Buffers *buffer.Pool   // the open files, so that a file open in several cores is one text
	Proxy   *proxy.Manager // the port forwards (:browser), for every core
	Build   Build          // what the UI says about the build, for every core
}

// NewShared makes the process-wide parts, with the build the UI runs in.
func NewShared(b Build) *Shared {
	return &Shared{Proxy: proxy.NewManager(), Buffers: buffer.NewPool(), Build: b}
}

// Close stops what the process holds for all its cores: the port forwards and
// the shell integration files.
func (s *Shared) Close() {
	s.Proxy.StopAll()
	RemoveShellIntegration()
}

// build is the build the UI said it runs in. A core made without Shared has
// none.
func (a *Core) build() Build {
	if a.shared == nil {
		return Build{}
	}
	return a.shared.Build
}

// buffers is the pool of open files. A core made without Shared has none: it
// opens each file for itself.
func (s *Shared) buffers() *buffer.Pool {
	if s == nil {
		return nil
	}
	return s.Buffers
}

// settleViews makes the cursors of the core's buffers fit the text, which
// another core that has the same file open may have edited since.
func (a *Core) settleViews() {
	for _, t := range a.Tabs {
		if t.Kind == TabText {
			t.Buf.Settle()
		}
	}
}
