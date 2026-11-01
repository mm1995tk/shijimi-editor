package server

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/mm1995tk/shijimi-editor/internal/ctl"
	"github.com/mm1995tk/shijimi-editor/internal/ide"
)

// Workspace is what the control socket and the Claude Code connection reach:
// the core of one workspace. The server holds one control socket and one IDE
// connection for all workspaces and hands each request to the workspace that
// has the focus (docs/design/gui-protocol.md, 3 章「サーバーモードの中身」).
// The workspaces' cores are internal/core.Workspaces (see Cores); none of them
// is a Workspace yet, so ctl and ide get errNoWorkspace until one is.
type Workspace interface {
	// Control runs a request from the control socket (shj diff, exec, ...).
	Control(req ctl.Request) ctl.Response
	// IDE runs a tool call from Claude Code. It may block until the user
	// decides (openDiff); ctx ends when the connection closes.
	IDE(ctx context.Context, tool string, args json.RawMessage) ([]ide.Content, error)
}

// errNoWorkspace is what CLIs and Claude Code get while no workspace has the focus.
var errNoWorkspace = errors.New("no workspace has the focus")

// SetFocus makes w the workspace that ctl and ide requests reach. nil means none.
func (s *Server) SetFocus(w Workspace) {
	s.mu.Lock()
	s.focus = w
	s.mu.Unlock()
}

func (s *Server) focused() Workspace {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.focus
}

// StartControl opens the control socket at path (see ctl.SocketPath). Requests
// go to the focused workspace.
func (s *Server) StartControl(path string) error {
	cs, err := ctl.Listen(path, func(req ctl.Request) ctl.Response {
		w := s.focused()
		if w == nil {
			return ctl.Response{Error: errNoWorkspace.Error()}
		}
		return w.Control(req)
	})
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.ctlSrv = cs
	s.mu.Unlock()
	return nil
}

// StartIDE starts the Claude Code connection and writes its lock file for root.
// Tool calls go to the focused workspace.
func (s *Server) StartIDE(root string, tools []ide.Tool) (*ide.Server, error) {
	is, err := ide.Start(root, tools, func(ctx context.Context, tool string, args json.RawMessage) ([]ide.Content, error) {
		w := s.focused()
		if w == nil {
			return nil, errNoWorkspace
		}
		return w.IDE(ctx, tool, args)
	})
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.ideSrv = is
	s.mu.Unlock()
	return is, nil
}

// WriteClipboard tells the GUI to put text on its OS clipboard (clipboard.write).
// With no GUI connected it is dropped.
func (s *Server) WriteClipboard(text string) {
	if !s.notifyOwner("clipboard.write", map[string]any{"text": text}) {
		s.log.Info("no GUI to notify", "method", "clipboard.write")
	}
}

// OpenURL tells the GUI to open url in its browser (url.open).
func (s *Server) OpenURL(url string) {
	if !s.notifyOwner("url.open", map[string]any{"url": url}) {
		s.log.Info("no GUI to notify", "method", "url.open")
	}
}
