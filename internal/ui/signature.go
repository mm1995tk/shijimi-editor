package ui

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// signaturePopup shows the parameters of the call the cursor is in, just above
// the cursor. It lives while the cursor stays on the row it was opened on.
type signaturePopup struct {
	tab  *tab
	row  int
	help *lsp.SignatureHelp
}

type signatureMsg struct {
	tab    *tab
	pos    buffer.Pos // where the request was made
	seq    int        // a.sigSeq at the time; a closed popup bumps it
	manual bool       // asked for with :signature; say so when there is nothing
	help   *lsp.SignatureHelp
}

// signatureHelp asks the server for the signature at the cursor. Typing must
// not report a missing or starting server; only an explicit request does.
func (a *App) signatureHelp(manual bool) tea.Cmd {
	t := a.curText()
	if t == nil {
		return nil
	}
	pos, seq := t.buf.Cur, a.sigSeq+1
	a.sigSeq = seq
	lp := lspPos(t.buf, pos)
	req := func(ctx context.Context, c *lsp.Client, t *tab) Event {
		h, err := c.SignatureHelp(ctx, t.buf.Path, lp)
		if err != nil {
			if !manual {
				return nil
			}
			return lspErrMsg{err}
		}
		return signatureMsg{tab: t, pos: pos, seq: seq, manual: manual, help: h}
	}
	if manual {
		return a.lspRequest("signatureHelpProvider", req)
	}
	c := a.lsp.client(t, "signatureHelpProvider")
	if c == nil {
		return nil
	}
	a.syncLSP()
	return a.lspReq(func(ctx context.Context) Event { return req(ctx, c, t) })
}

func (a *App) showSignature(m signatureMsg) {
	t := a.curText()
	if t == nil || t != m.tab || m.seq != a.sigSeq || t.buf.Cur.Row != m.pos.Row || a.mode != modeInsert && a.helix && !m.manual {
		return
	}
	if m.help == nil {
		a.sig = nil
		if m.manual {
			a.infoText(i18n.T("sig.none"))
		}
		return
	}
	a.sig = &signaturePopup{tab: t, row: m.pos.Row, help: m.help}
}

// signatureTyped keeps the popup in step with a typed rune: ")" closes it, a
// trigger character (re)opens it.
func (a *App) signatureTyped(t *tab, r rune) tea.Cmd {
	if r == ')' {
		a.closeSignature()
		return nil
	}
	cs, _ := a.lsp.clientsFor(t)
	for _, c := range cs {
		if c.IsSignatureTrigger(r) {
			return a.signatureHelp(false)
		}
	}
	return nil
}

// overlay is the popup as a screen.Overlay: one line, the signature with the
// active parameter in its own span, anchored at the cursor. It changes
// nothing. Drawing it as cells is drawSignature's work.
func (s *signaturePopup) overlay() screen.Overlay {
	rs := []rune(s.help.Label)
	pre, hi, post := rs, []rune(nil), []rune(nil)
	if h := s.help; h.ParamStart >= 0 && h.ParamEnd <= len(rs) && h.ParamStart < h.ParamEnd {
		pre, hi, post = rs[:h.ParamStart], rs[h.ParamStart:h.ParamEnd], rs[h.ParamEnd:]
	}
	var ln screen.Line
	for _, sp := range []screen.Span{{Text: string(pre)}, {Text: string(hi), Style: screen.StylePaletteHi}, {Text: string(post)}} {
		if sp.Text != "" {
			ln.Spans = append(ln.Spans, sp)
		}
	}
	ov := screen.Overlay{Kind: screen.OverlaySignature, Lines: []screen.Line{ln}}
	if s.tab != nil {
		at := toScreenPos(s.tab.buf.Cur)
		ov.Pane, ov.At = s.tab.path(), &at
	}
	return ov
}

// drawSignature draws a signature Overlay as a box of cells, cut to maxW.
func drawSignature(ov screen.Overlay, maxW int) []string {
	var sb strings.Builder
	sb.WriteString(stPalette.Render(" "))
	for _, sp := range ov.Lines[0].Spans {
		st := stPalette
		if sp.Style == screen.StylePaletteHi {
			st = stPaletteHi
		}
		sb.WriteString(st.Render(sp.Text))
	}
	sb.WriteString(stPalette.Render(" "))
	line := sb.String()
	if ansi.StringWidth(line) > maxW {
		line = fit(line, maxW)
	}
	return []string{line}
}

// render is the popup as a box of cells.
func (s *signaturePopup) render(maxW int) []string { return drawSignature(s.overlay(), maxW) }

// sigVisible reports whether the popup still belongs to the cursor's row.
func (a *App) sigVisible() bool {
	t := a.curText()
	return a.sig != nil && t == a.sig.tab && !a.fullPreview(t) && t.buf.Cur.Row == a.sig.row
}

// signaturePos puts the box on the row above the cursor, or below it when the
// cursor is on the top row of the editor.
func signaturePos(l layout, w, cx, cy int, box []string) (x, y int) {
	bw := 0
	for _, s := range box {
		bw = max(bw, ansi.StringWidth(s))
	}
	x = clamp(cx, l.rightX, max(l.rightX, w-bw))
	y = cy - len(box)
	if y < l.editY {
		y = cy + 1
	}
	return x, y
}

// closeSignature hides the popup and drops replies to requests still in flight.
func (a *App) closeSignature() {
	a.sig = nil
	a.sigSeq++
}
