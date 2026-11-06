package tui

import (
	"errors"

	"github.com/mm1995tk/shijimi-editor/core/ctl"
)

// guiBackend is the muxBackend for the GUI. It is for the shj that runs in the
// primary terminal of a GUI workspace and that "shj serve" accepted as the
// primary: the panes are terminals of the same workspace, which "shj serve"
// starts and the GUI lays out beside the primary (docs/design/gui-protocol.md,
// "Panes of the primary shj"). Every operation is one ctl.PaneRequest.
type guiBackend struct {
	// do sends a request to "shj serve"; tests replace it.
	do func(ctl.PaneRequest) (ctl.PaneReply, error)
}

func (*guiBackend) name() string { return "gui" }

// SetGUIPane makes the GUI a multiplexer that the terminal commands can use.
// do sends a ctl.PaneRequest to "shj serve" (ctlclient.Pane.Do). It is called
// only for the shj that is the primary of a GUI workspace; any other shj has
// no GUI backend and keeps the built-in terminal.
func (a *App) SetGUIPane(do func(ctl.PaneRequest) (ctl.PaneReply, error)) {
	a.mux.gui = &guiBackend{do: do}
}

func (g *guiBackend) split(cwd string, env []string) (string, error) {
	r, err := g.do(ctl.PaneRequest{Op: "split", Cwd: cwd, Env: env})
	if err != nil {
		return "", err
	}
	if r.ID == "" {
		return "", errors.New("no pane id")
	}
	return r.ID, nil
}

func (g *guiBackend) alive(id string) bool {
	r, err := g.do(ctl.PaneRequest{Op: "alive", ID: id})
	return err == nil && r.Alive
}

func (g *guiBackend) show(id string) error {
	_, err := g.do(ctl.PaneRequest{Op: "show", ID: id})
	return err
}

func (g *guiBackend) hide() error {
	_, err := g.do(ctl.PaneRequest{Op: "hide"})
	return err
}

func (g *guiBackend) send(id, text string, enter bool) error {
	_, err := g.do(ctl.PaneRequest{Op: "send", ID: id, Text: text, Enter: enter})
	return err
}

func (g *guiBackend) tail(id string, n int) []string {
	r, err := g.do(ctl.PaneRequest{Op: "tail", ID: id, N: n})
	if err != nil {
		return nil
	}
	return r.Lines
}

func (g *guiBackend) close(id string) { _, _ = g.do(ctl.PaneRequest{Op: "close", ID: id}) }

func (*guiBackend) shellEnv(sh, exe string) []string { return muxShellIntegration(sh, exe) }

func (g *guiBackend) openVideo(path string) error {
	_, err := g.do(ctl.PaneRequest{Op: "open", Kind: "video", Path: path})
	return err
}

// OpenVideo is core.VideoOpener's: the shj of the primary terminal of a GUI
// workspace asks the GUI to play the video in a pane beside the terminals,
// because a terminal cannot. Any other shj has no GUI to ask (ok is false) and
// opens the file as it did before.
func (a *App) OpenVideo(path string) (bool, error) {
	if a.mux.gui == nil {
		return false, nil
	}
	return true, a.mux.gui.openVideo(path)
}
