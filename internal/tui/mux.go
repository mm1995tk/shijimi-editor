package tui

import (
	"fmt"
	"os"

	"github.com/mm1995tk/shijimi-editor/internal/ctl"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/ide"
	"github.com/mm1995tk/shijimi-editor/internal/term"
)

// Inside a terminal multiplexer (herdr, tmux, zellij, cmux), the terminal commands (Ctrl+`, :sh,
// :run, :!, Space t, Space a) use a pane of the multiplexer instead of the
// built-in terminal panel, so that the multiplexer detects, notifies about and
// restores the agents run there. The pane is created below the editor's own
// pane, hidden by zooming the editor's pane, and closed when the editor quits.
// The built-in panel stays in use when [experimental] multiplexer is off,
// [terminal] multiplexer is "none", or the editor does not run in a
// multiplexer that is supported.


// muxBackend is what a multiplexer has to do for the terminal commands.
// Adding a multiplexer means implementing it and adding it to newMuxBackend
// and muxProcessNames.
type muxBackend interface {
	name() string
	// split creates a pane below the editor's pane in cwd with env added to
	// its environment, focuses it and returns its id.
	split(cwd string, env []string) (id string, err error)
	alive(id string) bool
	// show shows the pane again and moves focus to it.
	show(id string) error
	// hide hides the pane (by zooming the editor's pane).
	hide() error
	// send writes text to the pane: a command line that is run (Enter
	// included), or pasted text that is left for the person to confirm.
	send(id, text string, enter bool) error
	// tail returns the last n lines of the pane's screen.
	tail(id string, n int) []string
	close(id string)
	// shellEnv is the environment that makes shell sh report its exit codes
	// through "exe command-end <code>"; nil when the shell is not supported.
	shellEnv(sh, exe string) []string
}

// muxPane is the pane of the multiplexer that stands in for the terminal panel.
type muxPane struct {
	b        muxBackend // nil until resolved, and when the built-in terminal is used
	resolved bool
	id       string // "" until a pane has been created
	visible  bool

	// Tests replace these.
	newBackend func(kind string) muxBackend
	parents    func(pid int) (ppid int, comm string, ok bool)
}

// makeBackend returns the backend of a multiplexer that is supported, or nil.
func (m *muxPane) makeBackend(kind string) muxBackend {
	if m.newBackend != nil {
		return m.newBackend(kind)
	}
	switch kind {
	case "herdr":
		return &herdrBackend{}
	case "tmux":
		return newTmuxBackend()
	case "zellij":
		return newZellijBackend()
	case "cmux":
		return newCmuxBackend()
	}
	return nil
}

// muxUsable reports whether the editor can use the supported multiplexer kind
// in this environment.
func muxUsable(kind string) bool {
	switch kind {
	case "herdr":
		return os.Getenv(envHerdr) == "1"
	case "tmux":
		return os.Getenv(envTmux) != ""
	case "zellij":
		return os.Getenv(envZellij) != ""
	case "cmux":
		return cmuxGOOS == cmuxOS && os.Getenv(envCmuxWorkspace) != "" && os.Getenv(envCmuxSurface) != ""
	}
	return true
}

// resolveMux decides which multiplexer the terminal commands use from the
// current configuration. It runs on first use and again after
// :config-reload. A pane of a multiplexer that is no longer used is closed.
func (a *App) resolveMux() {
	m := &a.mux
	m.resolved = true
	old := m.b
	m.b = nil
	want := a.settings().Terminal.Multiplexer
	explicit := want != "auto" && want != "" && want != "none"
	kind := want
	if want == "auto" || want == "" {
		kind = detectMultiplexer(os.Getpid(), m.parents)
	}
	if kind != "" && kind != "none" {
		switch b := m.makeBackend(kind); {
		case b == nil:
			if explicit {
				a.infoText(fmt.Sprintf(i18n.T("mux.unsupported"), want))
			}
		case !muxUsable(kind):
			if explicit {
				a.infoText(fmt.Sprintf(i18n.T("mux.unavailable"), want))
			}
		default:
			m.b = b
		}
	}
	if old != nil && m.id != "" && (m.b == nil || m.b.name() != old.name()) {
		if old.alive(m.id) {
			old.close(m.id)
		}
		m.id, m.visible = "", false
	}
}

// muxOn reports whether the terminal commands should use a multiplexer pane.
func (a *App) MuxOn() bool {
	if a.singleFile() || a.settings() == nil || !a.settings().Experimental.Multiplexer {
		return false
	}
	if !a.mux.resolved {
		a.resolveMux()
	}
	return a.mux.b != nil
}

// muxError names the multiplexer in an error of one of its commands.
func (a *App) muxError(err error) error {
	if err != nil {
		return fmt.Errorf(i18n.T("mux.failed"), a.mux.b.name(), err)
	}
	return nil
}

// muxPaneEnv is the environment the pane's shell starts with: the IDE
// integration and the control socket of this editor (as set up in cli.Main),
// and the shell integration that reports exit codes through shj.
func (a *App) muxPaneEnv() []string {
	var env []string
	for _, k := range []string{ctl.EnvSocket, ide.EnvPort, ide.EnvEnable, ide.EnvForceTerminal} {
		if v := os.Getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	if exe, err := os.Executable(); err == nil {
		env = append(env, a.mux.b.shellEnv(term.DefaultShell(), exe)...)
	}
	return env
}

// muxCreate splits a new pane off the editor's pane and focuses it.
func (a *App) muxCreate() error {
	m := &a.mux
	id, err := m.b.split(a.rootDir(), a.muxPaneEnv())
	if err != nil {
		return err
	}
	m.id, m.visible = id, true
	return nil
}

// muxOpen shows the pane and moves focus to it, creating it the first time
// and again after it has ended.
func (a *App) MuxOpen() error {
	m := &a.mux
	if m.id == "" || !m.b.alive(m.id) {
		m.id, m.visible = "", false
		return a.muxCreate()
	}
	if err := m.b.show(m.id); err != nil {
		return err
	}
	m.visible = true
	return nil
}

func (a *App) MuxHide() error {
	if err := a.mux.b.hide(); err != nil {
		return err
	}
	a.mux.visible = false
	return nil
}

// muxToggle is the toggle of Ctrl+`, Space t and :sh.
func (a *App) MuxToggle() []Effect {
	var err error
	if a.mux.visible && a.mux.id != "" && a.mux.b.alive(a.mux.id) {
		err = a.MuxHide()
	} else {
		err = a.MuxOpen()
	}
	return a.MuxResult(err)
}

// muxSend opens the pane and sends text to it.
func (a *App) MuxSend(text string, enter bool) []Effect {
	if err := a.MuxOpen(); err != nil {
		return a.MuxResult(err)
	}
	return a.MuxResult(a.mux.b.send(a.mux.id, text, enter))
}

// muxClose closes the pane that this editor created, if it is still there.
func (a *App) MuxClose() {
	m := &a.mux
	if m.b != nil && m.id != "" && m.b.alive(m.id) {
		m.b.close(m.id)
	}
	m.id, m.visible = "", false
}

func (a *App) MuxResult(err error) []Effect {
	if err != nil {
		a.fail(a.muxError(err))
	}
	return nil
}

// commandEnded handles the exit code that the shell in the multiplexer pane
// reports with "shj command-end <code>" (see muxShellIntegration). It
// plays the part of scanTerminals for the built-in terminal.
func (a *App) CommandEnded(code int) {
	switch code {
	case 0:
		a.clearFailure(MuxFailTab)
	case 130:
		// Ctrl+C: the user stopped it on purpose.
	default:
		var tail []string
		if a.mux.b != nil && a.mux.id != "" {
			tail = a.mux.b.tail(a.mux.id, failTail)
		}
		a.addFailureTab(MuxFailTab, 1, code, "", tail)
	}
}
