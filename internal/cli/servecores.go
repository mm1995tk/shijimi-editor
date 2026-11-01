package cli

import (
	"errors"
	"log/slog"

	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/ctl"
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

var _ server.Inputs = serveCores{}

// update hands in to root's core, if it has one.
func (s serveCores) update(root string, in core.Input) {
	if w := s.ws.Lookup(root); w != nil {
		w.Update(in)
	}
}

// Key is input.key. A release is not an input of its own: the core of `shj
// serve` asks for no releases (ReleasesReported), as a terminal that does not
// report them.
func (s serveCores) Key(root string, in server.KeyInput) {
	if in.Phase == "release" {
		return
	}
	k, ok := core.KeyFromW3C(in.Key, in.Text, core.ModOf(in.Mods))
	if !ok {
		return
	}
	k.Repeat, k.Seq = in.Phase == "repeat", in.Seq
	s.update(root, k)
}

// Composition is input.composition: the caret sits at the end of the range
// the input method marks.
func (s serveCores) Composition(root string, in server.CompositionInput) {
	c := core.Composition{Text: in.Text, Phase: core.CompositionUpdate, Seq: in.Seq}
	if in.Cursor == nil {
		c.Phase = core.CompositionEnd
	} else {
		c.Cursor = in.Cursor[1]
	}
	s.update(root, c)
}

// Mouse is input.mouse. A wheel turn of dy rows (dx columns) is that many
// steps of one row (one column).
func (s serveCores) Mouse(root string, in server.MouseInput) {
	mod := core.ModOf(in.Mods)
	m := core.Mouse{
		Region: core.RegionEditor,
		Pane:   in.Pane,
		Row:    in.Row,
		Col:    in.Col,
		Shift:  mod.Has(core.ModShift),
		Alt:    mod.Has(core.ModAlt),
		Ctrl:   mod.Has(core.ModCtrl),
		Clicks: in.Clicks,
		Seq:    in.Seq,
	}
	if in.Kind == "wheel" {
		steps := func(n int, neg, pos core.MouseButton) {
			b := pos
			if n < 0 {
				n, b = -n, neg
			}
			for range n {
				m.Button = b
				s.update(root, m)
			}
		}
		m.Action = core.MousePress
		steps(in.Dy, core.MouseWheelUp, core.MouseWheelDown)
		steps(in.Dx, core.MouseWheelLeft, core.MouseWheelRight)
		return
	}
	switch in.Kind {
	case "down":
		m.Action = core.MousePress
	case "up":
		m.Action = core.MouseRelease
	default:
		m.Action = core.MouseMotion
	}
	if in.Button != nil {
		switch *in.Button {
		case 0:
			m.Button = core.MouseLeft
		case 1:
			m.Button = core.MouseMiddle
		case 2:
			m.Button = core.MouseRight
		default:
			m.Button = core.MouseOther
		}
	}
	s.update(root, m)
}

// Paste is input.paste.
func (s serveCores) Paste(root string, in server.PasteInput) {
	s.update(root, core.Paste{Content: in.Text, Seq: in.Seq})
}

// Focus is input.focus. Only the files region is the core's: the GUI keeps
// the focus of its other parts (the tree, the terminals) itself.
func (s serveCores) Focus(root string, in server.FocusInput) {
	if in.Region != screen.RegionFiles {
		return
	}
	if w := s.ws.Lookup(root); w != nil {
		w.Apply(func(c *core.Core) []core.Effect { return c.Apply(c.FocusFiles) })
	}
}

// Command is command.run.
func (s serveCores) Command(root, line string) (ok bool, message string) {
	w := s.ws.Lookup(root)
	if w == nil {
		return false, "no core for " + root
	}
	var resp ctl.Response
	w.Apply(func(c *core.Core) []core.Effect {
		return c.Apply(func() []core.Effect {
			r, effs := c.Exec(line)
			resp = r
			return effs
		})
	})
	if !resp.OK {
		return false, resp.Error
	}
	return true, resp.Message
}

// OpenFile is file.open, in root's core (made if it has none).
func (s serveCores) OpenFile(root, path string, at *screen.Pos, pane string) error {
	w := s.ws.Open(root)
	var err error
	w.Apply(func(c *core.Core) []core.Effect {
		return c.Apply(func() []core.Effect {
			effs, ok := c.OpenAt(path, at, pane)
			if !ok {
				err = errors.New(c.Msg)
			}
			return effs
		})
	})
	return err
}
