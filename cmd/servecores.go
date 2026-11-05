package cmd

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/mm1995tk/shijimi-editor/core"
	"github.com/mm1995tk/shijimi-editor/core/screen"
	"github.com/mm1995tk/shijimi-editor/ui/cli"
	"github.com/mm1995tk/shijimi-editor/ui/gui/server"
)

// serveCores hands the workspaces' cores (core.Workspaces) to the server
// (server.Cores), which knows nothing of core.
type serveCores struct {
	ws  *core.Workspaces
	log *slog.Logger // serve.log
}

var _ server.Cores = serveCores{}

func (s serveCores) Open(id, root string) error {
	_, err := s.ws.Open(id, root)
	if err != nil {
		return fmt.Errorf(cli.T("serve.workspaceconfig"), root, cli.ErrText(err))
	}
	return nil
}

func (s serveCores) Files(id string) []string {
	var out []string
	if w := s.ws.Lookup(id); w != nil {
		w.Do(func(c *core.Core) { out = c.OpenPaths() })
	}
	return out
}

func (s serveCores) SaveAll(id string) (failed []string) {
	if w := s.ws.Lookup(id); w != nil {
		w.Do(func(c *core.Core) { failed = c.SaveFiles() })
	}
	return failed
}

// Release saves the files (or throws the edits away, as the configuration says
// when auto-save is off) and stops the core. An edit that cannot be saved goes
// next to its file as <name>.bk like the ones of a server that stops, so that
// switching folders never loses it silently.
func (s serveCores) Release(id string) {
	w := s.ws.Lookup(id)
	if w == nil {
		return
	}
	w.Do(func(c *core.Core) {
		if !c.SavesOnSwitch() {
			return
		}
		if failed := c.SaveFiles(); len(failed) > 0 {
			written, err := c.WriteBackups(server.WriteBackup)
			s.log.Warn("could not save on switching folders", "workspace", id, "files", failed, "backups", written, "err", err)
		}
	})
	s.ws.Close(id)
}

func (s serveCores) Close(id string) { s.ws.Close(id) }

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

// Screen builds the screen of the workspace's core at the regions' sizes v.
func (s serveCores) Screen(id string, v screen.View) (sc screen.Screen, ok bool) {
	w := s.ws.Lookup(id)
	if w == nil {
		return sc, false
	}
	w.Do(func(c *core.Core) { sc = c.BuildScreen(v) })
	return sc, true
}

// Theme is the process's theme. It is read inside the workspace's core when there is
// one, as a theme is switched from inside a core.
func (s serveCores) Theme(id string) (server.Theme, bool) {
	var th server.Theme
	read := func() { th = server.ThemeOf(core.CurTheme.Name, core.CurTheme.Palette()) }
	if w := s.ws.Lookup(id); w != nil {
		w.Do(func(*core.Core) { read() })
	} else {
		read()
	}
	return th, true
}

// Blob is the image a pane of the workspace's core shows.
func (s serveCores) Blob(wsID, id string) (mime string, data []byte, err error) {
	w := s.ws.Lookup(wsID)
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

// update hands in to the workspace's core, if it has one.
func (s serveCores) update(id string, in core.Input) {
	if w := s.ws.Lookup(id); w != nil {
		w.Update(in)
	}
}

// Key is input.key. A release is not an input of its own: the core of `shj
// serve` asks for no releases (ReleasesReported), as a terminal that does not
// report them.
func (s serveCores) Key(id string, in server.KeyInput) {
	if in.Phase == "release" {
		return
	}
	k, ok := core.KeyFromW3C(in.Key, in.Text, core.ModOf(in.Mods))
	if !ok {
		return
	}
	k.Repeat, k.Seq = in.Phase == "repeat", in.Seq
	s.update(id, k)
}

// Composition is input.composition: the caret sits at the end of the range
// the input method marks.
func (s serveCores) Composition(id string, in server.CompositionInput) {
	c := core.Composition{Text: in.Text, Phase: core.CompositionUpdate, Seq: in.Seq}
	if in.Cursor == nil {
		c.Phase = core.CompositionEnd
	} else {
		c.Cursor = in.Cursor[1]
	}
	s.update(id, c)
}

// Mouse is input.mouse. A wheel turn of dy rows (dx columns) is that many
// steps of one row (one column).
func (s serveCores) Mouse(id string, in server.MouseInput) {
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
				s.update(id, m)
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
	s.update(id, m)
}

// Paste is input.paste.
func (s serveCores) Paste(id string, in server.PasteInput) {
	s.update(id, core.Paste{Content: in.Text, Seq: in.Seq})
}

// Focus is input.focus. Only the files region is the core's: the GUI keeps
// the focus of its other parts (the tree, the terminals) itself.
func (s serveCores) Focus(id string, in server.FocusInput) {
	if in.Region != screen.RegionFiles {
		return
	}
	if w := s.ws.Lookup(id); w != nil {
		w.Apply(func(c *core.Core) []core.Effect { return c.Apply(c.FocusFiles) })
	}
}

// Command is command.run.
func (s serveCores) Command(id, line string) (ok bool, message string) {
	w := s.ws.Lookup(id)
	if w == nil {
		return false, "no core for " + id
	}
	var resp core.CtlResponse
	w.Apply(func(c *core.Core) []core.Effect {
		return c.Apply(func() []core.Effect {
			r, effs := c.Exec(line)
			resp = r
			return effs
		})
	})
	if !resp.OK {
		return false, cli.Text(resp.Error)
	}
	return true, cli.Text(resp.Message)
}

// OpenFile is file.open, in the workspace's core (made if it has none).
func (s serveCores) OpenFile(id, root, path string, at *screen.Pos, pane string) error {
	w, err := s.ws.Open(id, root)
	if err != nil {
		return err
	}
	w.Apply(func(c *core.Core) []core.Effect {
		return c.Apply(func() []core.Effect {
			effs, ok := c.OpenAt(path, at, pane)
			if !ok {
				err = errors.New(cli.Text(c.Msg))
			}
			return effs
		})
	})
	return err
}

// Renamed moves the open files of every workspace's core from a renamed path
// to the new one, as the file tree's rename (r) does in the TUI.
func (s serveCores) Renamed(from, to string) {
	for _, w := range s.ws.List() {
		w.Apply(func(c *core.Core) []core.Effect {
			return c.Apply(func() []core.Effect {
				c.MovePaths(from, to)
				return nil
			})
		})
	}
}
