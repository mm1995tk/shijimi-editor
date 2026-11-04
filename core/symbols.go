package core

import (
	"context"
	"fmt"
	"time"

	"github.com/mm1995tk/shijimi-editor/core/lsp"
	"github.com/mm1995tk/shijimi-editor/core/msg"
)

// symbolQueryDelay is how long the input rests before workspace/symbol is
// asked again.
const symbolQueryDelay = 250 * time.Millisecond

type symbolsMsg struct {
	path string
	syms []lsp.Symbol
}

// symbolQueryMsg fires symbolQueryDelay after an edit of the picker's input.
type symbolQueryMsg struct{ seq int }

type workspaceSymbolsMsg struct {
	seq  int
	syms []lsp.Symbol
}

// queryLater schedules a symbolQueryMsg for the current input.
func (p *Palette) queryLater() []Effect {
	p.liveSeq++
	seq := p.liveSeq
	return []Effect{StartTimer{After: symbolQueryDelay, Fire: symbolQueryMsg{seq}}}
}

// documentSymbols lists the functions, types and variables of the current file.
func (a *Core) documentSymbols() []Effect {
	return a.lspRequest("documentSymbolProvider", func(ctx context.Context, c *lsp.Client, t *Tab) Event {
		syms, err := c.DocumentSymbols(ctx, t.Buf.Path)
		if err != nil {
			return lspErrMsg{err}
		}
		return symbolsMsg{t.Buf.Path, syms}
	})
}

func (a *Core) showSymbols(m symbolsMsg) []Effect {
	if len(m.syms) == 0 {
		a.InfoText(msg.T("symui.none"))
		return nil
	}
	items := make([]PaletteItem, len(m.syms))
	for i, s := range m.syms {
		items[i] = PaletteItem{Label: msg.S(s.Name), Detail: symbolDetail(s, "")}
	}
	return a.Pal.Pick(msg.T("symui.title"), items, func(i int) []Effect {
		return a.jumpTo(lsp.Location{URI: m.syms[i].URI, Range: m.syms[i].Range})
	})
}

// symbolDetail is the dim text after a symbol's name: its kind, the symbol it
// is inside, and where it is (the file too, when file is not empty).
func symbolDetail(s lsp.Symbol, file string) msg.Msg {
	d := lsp.SymbolKindName(s.Kind)
	if s.Container != "" {
		d = d.Add("  ", s.Container)
	}
	line := fmt.Sprintf("%d", s.Range.Start.Line+1)
	if file != "" {
		return d.Add("  ", file, ":", line)
	}
	return d.Add("  ", msg.T("symui.line", line))
}

// workspaceSymbols opens a picker over the project's symbols. The language
// server does the matching: the list is asked for again as the input changes.
func (a *Core) workspaceSymbols() []Effect {
	t := a.Cur()
	if t == nil || t.Kind != TabText {
		return nil
	}
	if t.Lang == nil {
		a.Fail(msg.Err("lspui.noserver"))
		return nil
	}
	c := a.Lsp.client(t, "workspaceSymbolProvider")
	if c == nil {
		a.Fail(msg.Err("symui.wsunsupported"))
		return nil
	}
	a.SyncLSP()
	a.wsSyms = nil
	a.wsClient = c
	return a.Pal.pickLive(msg.T("space.wsymbols"), func(i int) []Effect {
		if i >= len(a.wsSyms) {
			return nil
		}
		return a.jumpTo(lsp.Location{URI: a.wsSyms[i].URI, Range: a.wsSyms[i].Range})
	})
}

// runSymbolQuery asks the language server for the picker's current input,
// unless the input changed again since the timer was set.
func (a *Core) runSymbolQuery(m symbolQueryMsg) []Effect {
	p := a.Pal
	c := a.wsClient
	if !p.Open || !p.live || m.seq != p.liveSeq || c == nil {
		return nil
	}
	q := p.Input.Value()
	return a.lspReq(func(ctx context.Context) Event {
		syms, err := c.WorkspaceSymbols(ctx, q)
		if err != nil {
			return lspErrMsg{err}
		}
		return workspaceSymbolsMsg{m.seq, syms}
	})
}

func (a *Core) showWorkspaceSymbols(m workspaceSymbolsMsg) {
	p := a.Pal
	if !p.Open || !p.live || m.seq != p.liveSeq {
		return
	}
	a.wsSyms = m.syms
	items := make([]PaletteItem, len(m.syms))
	for i, s := range m.syms {
		items[i] = PaletteItem{Label: msg.S(s.Name), Detail: symbolDetail(s, a.Rel(lsp.Path(s.URI)))}
	}
	p.setPickItems(items)
}
