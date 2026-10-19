package ui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mm1995tk/dageki/internal/lsp"
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
func (p *palette) queryLater() tea.Cmd {
	p.liveSeq++
	seq := p.liveSeq
	return tea.Tick(symbolQueryDelay, func(time.Time) tea.Msg { return symbolQueryMsg{seq} })
}

// documentSymbols lists the functions, types and variables of the current file.
func (a *App) documentSymbols() tea.Cmd {
	return a.lspRequest("documentSymbolProvider", func(ctx context.Context, c *lsp.Client, t *tab) tea.Msg {
		syms, err := c.DocumentSymbols(ctx, t.buf.Path)
		if err != nil {
			return lspErrMsg{err}
		}
		return symbolsMsg{t.buf.Path, syms}
	})
}

func (a *App) showSymbols(m symbolsMsg) tea.Cmd {
	if len(m.syms) == 0 {
		a.info("シンボルがありません")
		return nil
	}
	items := make([]paletteItem, len(m.syms))
	for i, s := range m.syms {
		items[i] = paletteItem{label: s.Name, detail: symbolDetail(s, "")}
	}
	return a.pal.pick("シンボル", items, func(i int) tea.Cmd {
		return a.jumpTo(lsp.Location{URI: m.syms[i].URI, Range: m.syms[i].Range})
	})
}

// symbolDetail is the dim text after a symbol's name: its kind, the symbol it
// is inside, and where it is (the file too, when file is not empty).
func symbolDetail(s lsp.Symbol, file string) string {
	d := lsp.SymbolKindName(s.Kind)
	if s.Container != "" {
		d += "  " + s.Container
	}
	line := fmt.Sprintf("%d", s.Range.Start.Line+1)
	if file != "" {
		return d + "  " + file + ":" + line
	}
	return d + "  " + line + "行目"
}

// workspaceSymbols opens a picker over the project's symbols. The language
// server does the matching: the list is asked for again as the input changes.
func (a *App) workspaceSymbols() tea.Cmd {
	t := a.cur()
	if t == nil || t.kind != tabText {
		return nil
	}
	if t.lang == nil {
		a.fail(fmt.Errorf("この言語の言語サーバーは設定されていません"))
		return nil
	}
	c := a.lsp.client(t, "workspaceSymbolProvider")
	if c == nil {
		a.fail(fmt.Errorf("言語サーバーが workspaceSymbolProvider に対応していないか、起動中です"))
		return nil
	}
	a.syncLSP()
	a.wsSyms = nil
	a.wsClient = c
	return a.pal.pickLive("プロジェクトのシンボル", func(i int) tea.Cmd {
		if i >= len(a.wsSyms) {
			return nil
		}
		return a.jumpTo(lsp.Location{URI: a.wsSyms[i].URI, Range: a.wsSyms[i].Range})
	})
}

// runSymbolQuery asks the language server for the picker's current input,
// unless the input changed again since the timer was set.
func (a *App) runSymbolQuery(m symbolQueryMsg) tea.Cmd {
	p := a.pal
	c := a.wsClient
	if !p.open || !p.live || m.seq != p.liveSeq || c == nil {
		return nil
	}
	q := p.input.Value()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), lspTimeout)
		defer cancel()
		syms, err := c.WorkspaceSymbols(ctx, q)
		if err != nil {
			return lspErrMsg{err}
		}
		return workspaceSymbolsMsg{m.seq, syms}
	}
}

func (a *App) showWorkspaceSymbols(m workspaceSymbolsMsg) {
	p := a.pal
	if !p.open || !p.live || m.seq != p.liveSeq {
		return
	}
	a.wsSyms = m.syms
	items := make([]paletteItem, len(m.syms))
	for i, s := range m.syms {
		items[i] = paletteItem{label: s.Name, detail: symbolDetail(s, a.rel(lsp.Path(s.URI)))}
	}
	p.setPickItems(items)
}
