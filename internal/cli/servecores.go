package cli

import (
	"errors"
	"log/slog"

	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
	"github.com/mm1995tk/shijimi-editor/internal/server"
)

// serveCores hands the workspaces' cores (core.Workspaces) to the server
// (server.Cores), which knows nothing of core.
type serveCores struct{ ws *core.Workspaces }

var _ server.Cores = serveCores{}

func (s serveCores) Open(root string) { s.ws.Open(root) }

func (s serveCores) Files(root string) []string {
	var out []string
	if w := s.ws.Lookup(root); w != nil {
		w.Do(func(c *core.Core) { out = c.OpenPaths() })
	}
	return out
}

func (s serveCores) Kept() []string {
	var out []string
	for _, w := range s.ws.List() {
		w.Do(func(c *core.Core) {
			if c.HasOpenFiles() {
				out = append(out, w.Root)
			}
		})
	}
	return out
}

func (s serveCores) CloseFiles(root string) {
	if w := s.ws.Lookup(root); w != nil {
		w.Do(func(c *core.Core) { c.CloseFiles() })
	}
}

// Close stops root's core unless a file was opened in it since the server
// decided the workspace was gone: the core would take its edits with it.
func (s serveCores) Close(root string) {
	w := s.ws.Lookup(root)
	if w == nil {
		return
	}
	open := false
	w.Do(func(c *core.Core) { open = c.HasOpenFiles() })
	if !open {
		s.ws.Close(root)
	}
}

// writeBackups writes the unsaved edits of every workspace next to their files
// (<name>.bk) before the server stops for being idle or for server.shutdown.
func writeBackups(ws *core.Workspaces, log *slog.Logger) {
	written, err := ws.WriteBackups(server.WriteBackup)
	for _, p := range written {
		log.Info("wrote unsaved edits", "path", p)
	}
	if err != nil {
		log.Error("writing unsaved edits", "err", err)
	}
}

var _ server.Screens = serveCores{}

// Screen builds the screen of root's core at the regions' sizes v.
func (s serveCores) Screen(root string, v screen.View) (sc screen.Screen, ok bool) {
	w := s.ws.Lookup(root)
	if w == nil {
		return sc, false
	}
	w.Do(func(c *core.Core) { sc = c.BuildScreen(v) })
	return sc, true
}

// Theme is the process's theme. It is read inside root's core when there is
// one, as a theme is switched from inside a core.
func (s serveCores) Theme(root string) (server.Theme, bool) {
	var th server.Theme
	read := func() { th = server.ThemeOf(core.CurTheme.Name, core.CurTheme.Palette()) }
	if w := s.ws.Lookup(root); w != nil {
		w.Do(func(*core.Core) { read() })
	} else {
		read()
	}
	return th, true
}

// Blob is the image a pane of root's core shows.
func (s serveCores) Blob(root, id string) (mime string, data []byte, err error) {
	w := s.ws.Lookup(root)
	if w == nil {
		return "", nil, server.ErrNoBlob
	}
	w.Do(func(c *core.Core) { mime, data, err = c.BlobData(id) })
	if errors.Is(err, core.ErrNoBlob) {
		err = server.ErrNoBlob
	}
	return mime, data, err
}
