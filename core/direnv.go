package core

import (
	"context"
	"errors"

	"github.com/mm1995tk/shijimi-editor/core/internal/direnv"
	"github.com/mm1995tk/shijimi-editor/core/internal/nixenv"
	"github.com/mm1995tk/shijimi-editor/core/msg"
)

// DirenvLoaded is the answer of reading a workspace's direnv environment, or
// its flake's dev shell.
type DirenvLoaded struct {
	Env []string // the environment in os.Environ form; nil when Err is set
	Err error    // direnv.ErrBlocked when `direnv allow` is needed
	Nix bool     // the environment is the flake's (nix print-dev-env), not direnv's
}

func (DirenvLoaded) isEvent() {}

// startDirenv is for a workspace of `shj serve`, whose process has no
// `nix develop` or direnv PATH: if the folder has an .envrc and direnv is
// installed, it reads the environment direnv makes for the folder. Language
// servers and formatters wait for it, and run in it. Without those, a folder
// with a flake.nix gets the flake's dev shell once the user has said yes (see
// startNix). The TUI does not call this: the user starts it inside the
// environment they want.
func (a *Core) startDirenv() []Effect {
	if !direnv.Wanted(a.Root) {
		return a.startNix()
	}
	a.Lsp.waitEnv = true
	a.Info("direnv.loading")
	root := a.Root
	return async(func() Event {
		env, err := direnv.Load(context.Background(), root, nil)
		return DirenvLoaded{Env: env, Err: err}
	})
}

// startNix is for a folder that direnv does not serve but that has a flake.nix.
// nix has nothing like `direnv allow`, so the first time it asks whether to
// read the folder's dev shell, and keeps the answer. The servers wait until
// the answer is no or the environment is read.
func (a *Core) startNix() []Effect {
	if !nixenv.Wanted(a.Root) {
		return nil
	}
	answers := nixenv.NewAnswers(a.build().StateDir)
	allowed, answered := answers.Get(a.Root)
	if answered && !allowed {
		return nil
	}
	a.Lsp.waitEnv = true
	if answered {
		return a.loadNix()
	}
	root := a.Root
	a.Confirm = &YesNo{
		Prompt: msg.T("nixenv.ask", root),
		yes: func() []Effect {
			if err := answers.Set(root, true); err != nil {
				a.Fail(msg.Err("nixenv.save", err))
			}
			return a.loadNix()
		},
		no: func() []Effect {
			if err := answers.Set(root, false); err != nil {
				a.Fail(msg.Err("nixenv.save", err))
			}
			// the servers are looked for on the process's PATH, as without a flake
			a.handleDirenvLoaded(DirenvLoaded{Nix: true})
			return nil
		},
	}
	return nil
}

func (a *Core) loadNix() []Effect {
	a.Info("nixenv.loading")
	root := a.Root
	return async(func() Event {
		env, err := nixenv.Load(context.Background(), root, nil)
		return DirenvLoaded{Env: env, Err: err, Nix: true}
	})
}

func (a *Core) handleDirenvLoaded(ev DirenvLoaded) {
	a.Lsp.waitEnv = false
	switch {
	case ev.Err == nil:
		a.Lsp.env = ev.Env
		if a.Msg.Key == "direnv.loading" || a.Msg.Key == "nixenv.loading" {
			a.Msg = msg.Msg{}
		}
	case errors.Is(ev.Err, direnv.ErrBlocked):
		// the servers are looked for on the process's PATH, as without direnv
		a.Info("direnv.blocked")
	default:
		key := "direnv.failed"
		if ev.Nix {
			key = "nixenv.failed"
		}
		a.Fail(msg.Err(key, ev.Err))
	}
	for _, t := range a.Tabs {
		if t.Kind == TabText {
			a.attachLSP(t)
		}
	}
}
