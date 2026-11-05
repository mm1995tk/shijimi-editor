package core

import (
	"context"
	"errors"

	"github.com/mm1995tk/shijimi-editor/core/internal/direnv"
	"github.com/mm1995tk/shijimi-editor/core/msg"
)

// DirenvLoaded is the answer of reading a workspace's direnv environment.
type DirenvLoaded struct {
	Env []string // the environment in os.Environ form; nil when Err is set
	Err error    // direnv.ErrBlocked when `direnv allow` is needed
}

func (DirenvLoaded) isEvent() {}

// startDirenv is for a workspace of `shj serve`, whose process has no
// `nix develop` or direnv PATH: if the folder has an .envrc and direnv is
// installed, it reads the environment direnv makes for the folder. Language
// servers and formatters wait for it, and run in it. The TUI does not call
// this: the user starts it inside the environment they want.
func (a *Core) startDirenv() []Effect {
	if !direnv.Wanted(a.Root) {
		return nil
	}
	a.Lsp.waitEnv = true
	a.Info("direnv.loading")
	root := a.Root
	return async(func() Event {
		env, err := direnv.Load(context.Background(), root, nil)
		return DirenvLoaded{Env: env, Err: err}
	})
}

func (a *Core) handleDirenvLoaded(ev DirenvLoaded) {
	a.Lsp.waitEnv = false
	switch {
	case ev.Err == nil:
		a.Lsp.env = ev.Env
		if a.Msg.Key == "direnv.loading" {
			a.Msg = msg.Msg{}
		}
	case errors.Is(ev.Err, direnv.ErrBlocked):
		// the servers are looked for on the process's PATH, as without direnv
		a.Info("direnv.blocked")
	default:
		a.Fail(msg.Err("direnv.failed", ev.Err))
	}
	for _, t := range a.Tabs {
		if t.Kind == TabText {
			a.attachLSP(t)
		}
	}
}
