package core

import (
	"fmt"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/core/msg"
	"github.com/mm1995tk/shijimi-editor/core/screen"
)

// searchscreen.go is the search window as a screen.Overlay for the GUI. The
// TUI draws the same state with ui/tui's drawSearch; both read the geometry
// of SearchGeom, which the window's fit (FitPalette) has applied.

// SearchOverlay is the open search window as a screen.Overlay. It changes
// nothing; the scroll, the widths and the file of the preview are the ones
// fit left.
func (a *Core) SearchOverlay() screen.Overlay {
	s, g := a.Search, a.SearchGeom()
	sr := &screen.Search{Field: s.Field, BodyRows: s.BodyH, Scroll: s.Scroll, Cursor: s.Cursor, Total: len(s.Res.Hits), Busy: s.Busy}
	labels, hints := SearchLabels(), SearchPlaceholders()
	for i := range s.In {
		sr.Fields[i] = screen.SearchField{Label: labels[i], Text: s.In[i].Value(), Cursor: s.In[i].Position(), Placeholder: hints[i]}
	}
	sr.Options = [4]screen.SearchOption{
		{Label: msg.T("search.opt.case"), On: s.CaseSens},
		{Label: msg.T("search.opt.word"), On: s.Word},
		{Label: msg.T("search.opt.regex"), On: s.Regex},
		{Label: msg.T("search.opt.preserve"), On: s.KeepCase},
	}
	sr.Status, sr.Err = s.statusMsg()
	if len(s.Res.Hits) == 0 && s.In[FQuery].Value() != "" && !s.Busy && s.Res.Err == nil {
		sr.Empty = msg.T("search.nolines")
	}
	for i := s.Scroll; i < len(s.Res.Hits) && len(sr.Hits) < s.BodyH; i++ {
		sr.Hits = append(sr.Hits, a.searchHitRow(s.Res.Hits[i], g.Lw))
	}
	if hit := s.Cur(); hit != nil {
		sr.Preview = a.searchPreview(hit, g.Rw)
	}
	return screen.Overlay{Kind: screen.OverlaySearch, Title: msg.T("search.title"), Hint: msg.T("search.keys"), Search: sr}
}

// statusMsg is the status row: the error, what to do, or the counts.
func (s *SearchPanel) statusMsg() (m msg.Msg, isErr bool) {
	switch {
	case s.Res.Err != nil:
		return msg.S(FirstLine(s.Res.Err.Error())), true
	case s.In[FQuery].Value() == "":
		return msg.T("search.needquery"), false
	case s.Busy:
		return msg.T("search.searching"), false
	}
	more := msg.S("")
	if s.Res.Truncated {
		more = msg.T("search.more")
	}
	m = msg.T("search.summary", s.Res.Files, len(s.Res.Hits), more)
	if target, skipped := s.Counts(); s.In[FRepl].Value() != "" || skipped > 0 {
		m = msg.Cat(m, msg.T("search.replacetargets", target, skipped))
	}
	return m, false
}

func (a *Core) searchHitRow(h *SearchHit, w int) screen.SearchHit {
	label := fmt.Sprintf("%s:%d", a.Rel(h.Path), h.Row+1)
	if maxL := max(12, w*3/5); ansi.StringWidth(label) > maxL {
		label = "…" + ansi.TruncateLeft(label, ansi.StringWidth(label)-maxL+1, "")
	}
	rs := []rune(h.Line)
	lead := 0
	for lead < len(rs) && (rs[lead] == ' ' || rs[lead] == '\t') {
		lead++
	}
	if len(h.Spans) > 0 {
		lead = min(lead, h.Spans[0][0])
	}
	avail := max(1, w-ansi.StringWidth(label)-4)
	from := FirstColumn(h.Spans, lead, avail)
	return screen.SearchHit{Label: label, Line: screen.Line{No: h.Row, Spans: matchSpans(rs, nil, from, h.Spans)}, Skip: h.Skip}
}

// searchPreview is the lines around hit in the file of the preview, as the
// TUI's viewRows shows them within w cells.
func (a *Core) searchPreview(hit *SearchHit, w int) *screen.SearchPreview {
	v := a.Search.View
	p := &screen.SearchPreview{Title: fmt.Sprintf("%s:%d", a.Rel(hit.Path), hit.Row+1), Hit: hit.Row, Total: len(v.Lines)}
	cw := max(1, w-max(3, Digits(len(v.Lines)))-3)
	xoff := FirstColumn(hit.Spans, 0, cw)
	diff := a.Search.In[FRepl].Value() != "" && hit.New != hit.Line
	budget := max(1, a.Search.BodyH-2)
	if diff {
		budget = max(1, budget-2)
	}
	start := max(0, hit.Row-budget/2)
	end := min(len(v.Lines), start+budget)
	start = max(0, end-budget)
	for r := start; r < end; r++ {
		var colors []string
		if r < len(v.Colors) {
			colors = v.Colors[r]
		}
		var spans [][2]int
		if r == hit.Row {
			spans = hit.Spans
		}
		p.Lines = append(p.Lines, screen.Line{No: r, Spans: matchSpans(v.Lines[r], colors, xoff, spans)})
	}
	if diff {
		p.Del = &screen.Line{No: hit.Row, Spans: wholeSpans([]rune(hit.Line), xoff, screen.StyleDiffDelBg)}
		p.Add = &screen.Line{No: hit.Row, Spans: wholeSpans([]rune(hit.New), xoff, screen.StyleDiffAddBg)}
	}
	return p
}

// matchSpans splits rs[from:] into spans by syntax color (colors may be nil)
// and by matches (rune ranges of rs), which get the background StyleMatch.
// A tab is one space, as in the TUI.
func matchSpans(rs []rune, colors []string, from int, matches [][2]int) []screen.Span {
	var out []screen.Span
	var cur screen.Span
	for i := max(0, from); i < len(rs); i++ {
		sp := screen.Span{Style: styleAt(colors, i)}
		for _, m := range matches {
			if i >= m[0] && i < m[1] {
				sp.Bg = screen.StyleMatch
			}
		}
		if cur.Text != "" && (sp.Style != cur.Style || sp.Bg != cur.Bg) {
			out = append(out, cur)
			cur.Text = ""
		}
		if cur.Text == "" {
			cur = sp
		}
		r := rs[i]
		if r == '\t' {
			r = ' ' // as the TUI shows it, so that FirstColumn counts the same cells
		}
		cur.Text += string(r)
	}
	if cur.Text != "" {
		out = append(out, cur)
	}
	return out
}

// wholeSpans is rs[from:] as one span with the background bg.
func wholeSpans(rs []rune, from int, bg screen.Style) []screen.Span {
	if from >= len(rs) {
		return nil
	}
	return []screen.Span{{Text: string(rs[from:]), Bg: bg}}
}
