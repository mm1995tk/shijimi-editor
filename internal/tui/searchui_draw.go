package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// styledRunes renders rs[from:] in exactly w cells. colors gives per-rune
// foregrounds ("" = fg); spans are highlighted as matches.
func styledRunes(rs []rune, colors []string, from, w int, bg, fg string, spans [][2]int) string {
	type look struct{ fg, bg string }
	var sb, run strings.Builder
	var cur look
	used := 0
	flush := func() {
		if run.Len() > 0 {
			sb.WriteString(bgfg(cur.bg, cur.fg).Render(run.String()))
			run.Reset()
		}
	}
	for i := max(0, from); i < len(rs); i++ {
		r := rs[i]
		if r == '\t' {
			r = ' '
		}
		if r < ' ' {
			continue
		}
		rw := ansi.StringWidth(string(r))
		if used+rw > w {
			break
		}
		l := look{fg, bg}
		if i < len(colors) && colors[i] != "" {
			l.fg = colors[i]
		}
		for _, sp := range spans {
			if i >= sp[0] && i < sp[1] {
				l.bg = cMatch
			}
		}
		if l != cur {
			flush()
			cur = l
		}
		run.WriteRune(r)
		used += rw
	}
	flush()
	if used < w {
		sb.WriteString(bgfg(bg, fg).Render(strings.Repeat(" ", w-used)))
	}
	return sb.String()
}

func (s *searchPanel) statusLine() string {
	switch {
	case s.res.Err != nil:
		return bgfg(cPaletteBg, cError).Render(" " + firstLine(s.res.Err.Error()))
	case s.in[fQuery].Value() == "":
		return stPaletteDim.Render(i18n.T("search.needquery"))
	case s.busy:
		return stPaletteDim.Render(i18n.T("search.searching"))
	}
	more := ""
	if s.res.Truncated {
		more = i18n.T("search.more")
	}
	out := fmt.Sprintf(i18n.T("search.summary"), s.res.Files, len(s.res.Hits), more)
	if target, skipped := s.counts(); s.in[fRepl].Value() != "" || skipped > 0 {
		out += fmt.Sprintf(i18n.T("search.replacetargets"), target, skipped)
	}
	return stPalette.Render(out)
}

func (s *searchPanel) togglesLine() string {
	t := func(on bool, text string) string {
		if on {
			return stPaletteSel.Render(text)
		}
		return stPaletteDim.Render(text)
	}
	sp := stPalette.Render("  ")
	return stPalette.Render(" ") + t(s.caseSens, i18n.T("search.opt.case")) + sp + t(s.word, i18n.T("search.opt.word")) + sp +
		t(s.regex, i18n.T("search.opt.regex")) + sp + t(s.keepCase, i18n.T("search.opt.preserve"))
}

// fieldBox draws input field i as a three-row box whose top edge carries the
// label. The focused field gets a bold border in the strong colour.
func (s *searchPanel) fieldBox(i, w int) [3]string {
	bd := bgfg(cPaletteBg, cBorder)
	if i == s.field {
		bd = bgfg(cPaletteBg, curTheme.Strong).Bold(true)
	}
	title := " " + searchLabels()[i] + " "
	// The width of the input is set in fit.
	body := viewField(&s.in[i], inputStyleOn(cPaletteBg))
	if s.in[i].Value() == "" {
		body = s.placeholderView(i)
	}
	return [3]string{
		bd.Render("┌─" + title + strings.Repeat("─", max(0, w-3-ansi.StringWidth(title))) + "┐"),
		bd.Render("│") + stPalette.Render(" ") + fitStyled(body, w-4, stPalette) + stPalette.Render(" ") + bd.Render("│"),
		bd.Render("└" + strings.Repeat("─", max(0, w-2)) + "┘"),
	}
}

func (s *searchPanel) matchedRows(rel func(string) string, w, h int) []string {
	var out []string
	for i := s.scroll; i < len(s.res.Hits) && len(out) < h; i++ {
		out = append(out, matchedRow(rel, s.res.Hits[i], w, i == s.cursor))
	}
	if len(s.res.Hits) == 0 && s.in[fQuery].Value() != "" && !s.busy && s.res.Err == nil {
		out = append(out, stPaletteDim.Render(fit(i18n.T("search.nolines"), w)))
	}
	for len(out) < h {
		out = append(out, stPalette.Render(fit("", w)))
	}
	return out
}

func matchedRow(rel func(string) string, h *core.SearchHit, w int, selected bool) string {
	bg, fg, dim := cPaletteBg, cEditorFg, cLineNo
	if selected {
		bg, fg, dim = cListSel, curTheme.Strong, curTheme.SubSel
	}
	marker := "  "
	switch {
	case h.Skip:
		marker = "x "
	case selected:
		marker = "> "
	}
	label := fmt.Sprintf("%s:%d", rel(h.Path), h.Row+1)
	if maxL := max(12, w*3/5); ansi.StringWidth(label) > maxL {
		label = "…" + ansi.TruncateLeft(label, ansi.StringWidth(label)-maxL+1, "")
	}
	prefix := marker + label + "  "
	rs := []rune(h.Line)
	lead := 0
	for lead < len(rs) && (rs[lead] == ' ' || rs[lead] == '\t') {
		lead++
	}
	if len(h.Spans) > 0 {
		lead = min(lead, h.Spans[0][0])
	}
	avail := max(1, w-ansi.StringWidth(prefix))
	return bgfg(bg, dim).Render(prefix) + styledRunes(rs, nil, firstColumn(h.Spans, lead, avail), avail, bg, fg, h.Spans)
}

func (s *searchPanel) viewRows(rel func(string) string, w, h int) []string {
	blank := stPalette.Render(fit("", w))
	var out []string
	if hit := s.cur(); hit != nil && h > 0 {
		v := s.view // loaded in fit
		out = append(out, bgfg(cPaletteBg, curTheme.Heading).Bold(true).Render(fit(fmt.Sprintf(" %s:%d", rel(hit.Path), hit.Row+1), w)), blank)

		nd := max(3, digits(len(v.lines)))
		cw := max(1, w-nd-3)
		gutter := bgfg(cPaletteBg, cLineNo)
		xoff := firstColumn(hit.Spans, 0, cw)
		diff := s.in[fRepl].Value() != "" && hit.New != hit.Line
		budget := max(1, h-2)
		if diff {
			budget = max(1, budget-2)
		}
		start := max(0, hit.Row-budget/2)
		end := min(len(v.lines), start+budget)
		start = max(0, end-budget)
		for r := start; r < end; r++ {
			var colors []string
			if r < len(v.colors) {
				colors = v.colors[r]
			}
			var spans [][2]int
			if r == hit.Row {
				spans = hit.Spans
			}
			out = append(out, gutter.Render(fmt.Sprintf("%*d │ ", nd, r+1))+styledRunes(v.lines[r], colors, xoff, cw, cPaletteBg, cEditorFg, spans))
			if r == hit.Row && diff {
				out = append(out,
					gutter.Render(fmt.Sprintf("%*s - ", nd, ""))+styledRunes([]rune(hit.Line), nil, xoff, cw, cDelBg, cEditorFg, nil),
					gutter.Render(fmt.Sprintf("%*s + ", nd, ""))+styledRunes([]rune(hit.New), nil, xoff, cw, cAddBg, cEditorFg, nil))
			}
		}
	}
	for len(out) < h {
		out = append(out, blank)
	}
	return out[:h]
}

// placeholderView draws the hint of an empty field with the cursor on its
// first character, in the panel's colours.
func (s *searchPanel) placeholderView(i int) string {
	rs := []rune(searchPlaceholders()[i])
	first := stPaletteDim.Render(string(rs[0]))
	if s.in[i].Focused() {
		first = bgfg(curTheme.Cursor, curTheme.CursorFg).Render(string(rs[0]))
	}
	return first + stPaletteDim.Render(string(rs[1:]))
}

// drawSearch draws the window that g places, as a box of cells. The window
// fills the screen except for a one-cell margin on every side. rel names a
// path as the editor shows it.
func drawSearch(s *searchPanel, g searchGeom, rel func(string) string) (box []string) {
	pw, inner, lw, rw := g.pw, g.inner, g.lw, g.rw

	bs := func(t string) string { return stPaletteBox.Render(t) }
	line := func(content string) string { return bs("│") + content + bs("│") }

	title := i18n.T("search.title")
	box = append(box, bs("┌─"+title+strings.Repeat("─", max(0, pw-3-ansi.StringWidth(title)))+"┐"))

	// Two columns: search | replace, then include | exclude.
	bl, br := g.bl, g.br
	sp := stPalette.Render(" ")
	for _, pair := range [][2]int{{fQuery, fRepl}, {fInclude, fExclude}} {
		l, r := s.fieldBox(pair[0], bl), s.fieldBox(pair[1], br)
		for k := range l {
			box = append(box, line(sp+l[k]+sp+r[k]+sp))
		}
	}
	box = append(box, line(fitStyled(s.togglesLine(), inner, stPalette)))

	left, help := s.statusLine(), stPaletteDim.Render(i18n.T("search.keys"))
	if gap := inner - ansi.StringWidth(left) - ansi.StringWidth(help) - 1; gap > 0 {
		left += stPalette.Render(strings.Repeat(" ", gap)) + help
	}
	box = append(box, line(fitStyled(left, inner, stPalette)))
	box = append(box, bs("├"+strings.Repeat("─", lw)+"┬"+strings.Repeat("─", rw)+"┤"))

	matched := s.matchedRows(rel, lw, s.bodyH)
	preview := s.viewRows(rel, rw, s.bodyH)
	for i := 0; i < s.bodyH; i++ {
		box = append(box, bs("│")+matched[i]+bs("│")+preview[i]+bs("│"))
	}
	box = append(box, bs("└"+strings.Repeat("─", lw)+"┴"+strings.Repeat("─", rw)+"┘"))
	return box
}
