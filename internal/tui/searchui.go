package tui

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/config"
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// searchDebounce is how long the inputs must be idle before a search starts.
const searchDebounce = 150 * time.Millisecond

// Condition fields, in Tab order.
const (
	fQuery = iota
	fRepl
	fInclude
	fExclude
	numFields
)

func searchLabels() [numFields]string {
	return [numFields]string{i18n.T("app.find"), i18n.T("search.replace"), i18n.T("search.include"), i18n.T("search.exclude")}
}

func searchPlaceholders() [numFields]string {
	return [numFields]string{i18n.T("app.find"), i18n.T("search.replace"), i18n.T("search.includeph"), i18n.T("search.excludeph")}
}

// searchView caches the file shown in the View area.
type searchView struct {
	path   string
	lines  [][]rune
	colors [][]string
}

// searchPanel is the floating project search / replace window: conditions on
// top, matched lines at the lower left, a preview of the selected one at the
// lower right.
type searchPanel struct {
	open  bool
	in    [numFields]inputField
	field int

	caseSens, word, regex, keepCase bool

	seq     atomic.Int64 // id of the latest requested search
	busy    bool
	res     core.SearchResult
	skipped map[string]bool // lines left out of the replacement, by path and row

	cursor, scroll int
	restore        bool // after the next search, put the cursor at restoreAt
	restoreAt      int
	restoreKey     string // the replaced line; if it still matches, move past it
	bodyH          int
	rect           [4]int // x, y, w, h on screen
	view           searchView
}

func newSearchPanel(def config.Search) *searchPanel {
	s := &searchPanel{skipped: map[string]bool{}}
	for i := range s.in {
		s.in[i] = newInputField(4096)
	}
	s.in[fInclude].SetValue(strings.Join(def.Include, ","))
	s.in[fExclude].SetValue(strings.Join(def.Exclude, ","))
	return s
}

func (s *searchPanel) opts() core.SearchOpts {
	return core.SearchOpts{
		Query:    s.in[fQuery].Value(),
		Repl:     s.in[fRepl].Value(),
		Include:  splitGlobs(s.in[fInclude].Value()),
		Exclude:  splitGlobs(s.in[fExclude].Value()),
		CaseSens: s.caseSens,
		Word:     s.word,
		Regex:    s.regex,
		KeepCase: s.keepCase,
	}
}

func (s *searchPanel) focusField(f int) []Effect {
	s.field = (f + numFields) % numFields
	for i := range s.in {
		if i == s.field {
			s.in[i].Focus()
		} else {
			s.in[i].Blur()
		}
	}
	return nil
}

func (s *searchPanel) close() {
	s.open = false
	for i := range s.in {
		s.in[i].Blur()
	}
}

func (s *searchPanel) cur() *core.SearchHit {
	if s.cursor < 0 || s.cursor >= len(s.res.Hits) {
		return nil
	}
	return s.res.Hits[s.cursor]
}

func (s *searchPanel) move(d int) {
	s.cursor = clamp(s.cursor+d, 0, max(0, len(s.res.Hits)-1))
}

func skipKey(h *core.SearchHit) string { return fmt.Sprintf("%s\x00%d", h.Path, h.Row) }

func (s *searchPanel) toggleSkip() {
	if h := s.cur(); h != nil {
		h.Skip = !h.Skip
		if h.Skip {
			s.skipped[skipKey(h)] = true
		} else {
			delete(s.skipped, skipKey(h))
		}
	}
}

// counts returns the number of lines that will be replaced and the number
// left out.
func (s *searchPanel) counts() (target, skipped int) {
	for _, h := range s.res.Hits {
		if h.Skip {
			skipped++
		} else {
			target++
		}
	}
	return
}

// ---- opening ----

// openSearch shows the window with the conditions of the last use.
func (a *App) openSearch(field int) []Effect {
	if a.single {
		return nil
	}
	s := a.search
	s.open = true
	cmds := [][]Effect{s.focusField(field)}
	if s.in[fQuery].Value() != "" {
		cmds = append(cmds, a.searchNow())
	}
	return batch(cmds...)
}

// openSearchArgs opens the window for `:grep [-x globs] [-i globs] query` or
// (replace) `:replace [-x globs] [-i globs] query => replacement`.
func (a *App) openSearchArgs(arg string, replace bool) []Effect {
	field := fQuery
	if replace {
		field = fRepl
	}
	if strings.TrimSpace(arg) == "" {
		return a.openSearch(field)
	}
	s := a.search
	body, repl := arg, ""
	if replace {
		body, repl, _ = core.SplitReplaceArgs(arg)
	}
	q, include, exclude := parseGrepArgs(body, a.cfg.Search)
	s.in[fQuery].SetValue(q)
	s.in[fRepl].SetValue(repl)
	s.in[fInclude].SetValue(strings.Join(include, ","))
	s.in[fExclude].SetValue(strings.Join(exclude, ","))
	s.regex = false // the command line always searches for the text itself
	s.skipped = map[string]bool{}
	return a.openSearch(field)
}

// ---- searching ----

// openBufferLines returns the lines of the open text buffers; they may differ
// from the files on disk.
func (a *App) openBufferLines() map[string][]string {
	open := map[string][]string{}
	for _, t := range a.tabs {
		if t.kind == tabText && t.buf.Path != "" {
			lines := make([]string, len(t.buf.Lines))
			for i, l := range t.buf.Lines {
				lines[i] = string(l)
			}
			open[t.buf.Path] = lines
		}
	}
	return open
}

// searchNow starts a search immediately.
func (a *App) searchNow() []Effect {
	return a.searchEffects(a.search.seq.Add(1))
}

// scheduleSearch starts a search once the inputs have been idle.
func (a *App) scheduleSearch() []Effect {
	s := a.search
	seq := s.seq.Add(1)
	s.busy = true
	return []Effect{StartTimer{After: searchDebounce, Fire: SearchTick{seq}}}
}

// searchTick starts the search a debounce timer was waiting for, unless more
// was typed since or the window was closed.
func (a *App) searchTick(m SearchTick) []Effect {
	if !a.search.open || m.seq != a.search.seq.Load() {
		return nil
	}
	return a.searchEffects(m.seq)
}

func (a *App) searchEffects(seq int64) []Effect {
	s := a.search
	s.busy = true
	o, root, open := s.opts(), a.root, a.openBufferLines()
	cancelled := func() bool { return s.seq.Load() != seq }
	return []Effect{RunSearch{Run: func() Event {
		return SearchDone{seq: seq, res: core.RunSearch(root, o, open, cancelled)}
	}}}
}

func (a *App) searchDone(m SearchDone) {
	s := a.search
	if m.seq != s.seq.Load() {
		return
	}
	var keep string
	if h := s.cur(); h != nil {
		keep = skipKey(h)
	}
	s.busy = false
	s.res = m.res
	s.cursor = 0
	for i, h := range s.res.Hits {
		h.Skip = s.skipped[skipKey(h)]
		if keep != "" && skipKey(h) == keep {
			s.cursor = i
		}
	}
	if s.restore {
		at := s.restoreAt
		if at < len(s.res.Hits) && skipKey(s.res.Hits[at]) == s.restoreKey {
			at++
		}
		s.cursor = clamp(at, 0, max(0, len(s.res.Hits)-1))
		s.restore = false
	}
	s.view.path = ""
}

// ---- keys ----

func (a *App) handleSearchKey(msg Key) []Effect {
	s := a.search
	switch key := msg.String(); key {
	case "esc", "ctrl+c":
		s.close()
		return nil
	case "tab":
		return s.focusField(s.field + 1)
	case "shift+tab":
		return s.focusField(s.field - 1)
	case "up", "ctrl+p":
		s.move(-1)
		return nil
	case "down", "ctrl+n":
		s.move(1)
		return nil
	case "pgup":
		s.move(-max(1, s.bodyH-1))
		return nil
	case "pgdown":
		s.move(max(1, s.bodyH-1))
		return nil
	case "ctrl+x", "alt+x":
		s.toggleSkip()
		return nil
	case "alt+c":
		s.caseSens = !s.caseSens
		return a.searchNow()
	case "alt+w":
		s.word = !s.word
		return a.searchNow()
	case "alt+r":
		s.regex = !s.regex
		return a.searchNow()
	case "alt+p":
		s.keepCase = !s.keepCase
		return a.searchNow()
	case "enter":
		return a.searchOpenHit()
	case "alt+enter":
		return a.searchReplaceAll()
	case "alt+1":
		return a.searchReplaceOne()
	}
	return a.searchInput(msg)
}

// searchInput passes a key or a paste to the focused search field.
func (a *App) searchInput(msg Input) []Effect {
	s := a.search
	before := s.in[s.field].Value()
	a.lineInputUpdate(&s.in[s.field], msg)
	if s.in[s.field].Value() != before {
		return a.scheduleSearch()
	}
	return nil
}

func (a *App) searchMouse(msg Mouse) []Effect {
	s := a.search
	if msg.Action != MousePress {
		return nil
	}
	switch msg.Button {
	case MouseWheelUp:
		s.move(-3)
	case MouseWheelDown:
		s.move(3)
	case MouseLeft:
		r := s.rect
		if msg.X < r[0] || msg.X >= r[0]+r[2] || msg.Y < r[1] || msg.Y >= r[1]+r[3] {
			s.close()
		}
	}
	return nil
}

// searchOpenHit opens the selected line in the editor.
func (a *App) searchOpenHit() []Effect {
	s := a.search
	h := s.cur()
	if h == nil {
		return nil
	}
	q, regex := s.in[fQuery].Value(), s.regex
	s.close()
	a.pushJump()
	if !a.openFile(h.Path) {
		return nil
	}
	if t := a.curText(); t != nil {
		t.buf.GotoLine(h.Row + 1)
		t.buf.Cur.Col = min(h.Spans[0][0], len(t.buf.Lines[t.buf.Cur.Row]))
		if !regex {
			a.lastSearch = q
		}
	}
	a.follow = true
	return a.setFocus(focusEditor)
}

// searchReplaceAll replaces the checked lines. Every affected file is opened
// as a buffer and edited in memory as one undo step; nothing is written to
// disk until the user saves, so Ctrl+Z / u reverts the whole replacement.
func (a *App) searchReplaceAll() []Effect {
	s := a.search
	if s.busy {
		a.fail(errors.New(i18n.T("search.busy")))
		return nil
	}
	if s.res.Truncated {
		a.fail(fmt.Errorf(i18n.T("search.toomany"), core.MaxSearchHits))
		return nil
	}
	if n, _ := s.counts(); n == 0 {
		a.fail(errors.New(i18n.T("search.nothingtoreplace")))
		return nil
	}
	var hits []*core.SearchHit
	for _, h := range s.res.Hits {
		if !h.Skip {
			hits = append(hits, h)
		}
	}
	s.close()
	a.replaceHits(hits)
	return nil
}

// searchReplaceOne replaces only the selected line and keeps the panel open,
// re-running the search so the cursor lands on the next hit.
func (a *App) searchReplaceOne() []Effect {
	s := a.search
	if s.busy {
		a.fail(errors.New(i18n.T("search.busy")))
		return nil
	}
	h := s.cur()
	if h == nil {
		a.fail(errors.New(i18n.T("search.nothingtoreplace")))
		return nil
	}
	if h.Skip {
		// picking a line to replace is explicit, so it overrides the exclusion
		h.Skip = false
		delete(s.skipped, skipKey(h))
	}
	idx, key := s.cursor, skipKey(h)
	if !a.replaceHits([]*core.SearchHit{h}) {
		return nil
	}
	s.restore, s.restoreAt, s.restoreKey = true, idx, key
	return a.searchNow()
}

// replaceHits applies the replacement to hits, opening each affected file as
// a buffer. It reports whether every file could be opened.
func (a *App) replaceHits(hits []*core.SearchHit) bool {
	var paths []string
	byPath := map[string][]*core.SearchHit{}
	for _, h := range hits {
		if _, ok := byPath[h.Path]; !ok {
			paths = append(paths, h.Path)
		}
		byPath[h.Path] = append(byPath[h.Path], h)
	}
	var failed []string
	done, stale, changed := 0, 0, 0
	for _, p := range paths {
		if !a.openFile(p) {
			failed = append(failed, a.rel(p))
			continue
		}
		t := a.curText()
		if t == nil {
			continue
		}
		edits, n, st := core.HitEdits(byPath[p], t.buf.Lines)
		stale += st
		if len(edits) > 0 {
			t.buf.ApplyEdits(edits)
			done += n
			changed++
		}
	}
	if len(failed) > 0 {
		a.fail(fmt.Errorf(i18n.T("search.openfail"), strings.Join(failed, ", ")))
		return false
	}
	msg := fmt.Sprintf(i18n.T("search.replaced"), done, changed)
	if stale > 0 {
		a.msg, a.msgErr = fmt.Sprintf(i18n.T("search.skipped"), msg, stale), true
		return true
	}
	a.info("%s", msg)
	return true
}

// ---- rendering ----

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

// firstColumn picks the first visible column so that the first match is in
// view within w cells.
func firstColumn(spans [][2]int, from, w int) int {
	if len(spans) == 0 || spans[0][1]-from < w {
		return from
	}
	return max(from, spans[0][0]-w/3)
}

func digits(n int) int { return len(fmt.Sprint(n)) }

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
	body := s.in[i].view(inputStyleOn(cPaletteBg))
	if s.in[i].Value() == "" {
		body = s.placeholderView(i)
	}
	return [3]string{
		bd.Render("┌─" + title + strings.Repeat("─", max(0, w-3-ansi.StringWidth(title))) + "┐"),
		bd.Render("│") + stPalette.Render(" ") + fitStyled(body, w-4, stPalette) + stPalette.Render(" ") + bd.Render("│"),
		bd.Render("└" + strings.Repeat("─", max(0, w-2)) + "┘"),
	}
}

// searchCaret is the screen cell where text typed into the focused field
// goes. It follows the layout of renderSearch and fieldBox.
func (a *App) searchCaret() (x, y int, ok bool) {
	s := a.search
	if s.rect[2] == 0 {
		return 0, 0, false
	}
	// window border, gap, field border, padding
	x, y = s.rect[0]+4, s.rect[1]+2
	if s.field == fRepl || s.field == fExclude {
		x += (s.rect[2]-5)/2 + 1
	}
	if s.field == fInclude || s.field == fExclude {
		y += 3
	}
	if s.in[s.field].Value() != "" {
		x += s.in[s.field].Caret()
	}
	return x, y, true
}

// renderSearch draws the window and returns its screen position. The window
// fills the screen except for a one-cell margin on every side.
func (a *App) renderSearch() (x, y int, box []string) {
	s := a.search
	g := a.searchGeom()
	pw, inner, lw, rw := g.pw, g.inner, g.lw, g.rw
	x, y = g.x, g.y

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

	matched := s.matchedRows(a, lw, s.bodyH)
	preview := s.viewRows(a, rw, s.bodyH)
	for i := 0; i < s.bodyH; i++ {
		box = append(box, bs("│")+matched[i]+bs("│")+preview[i]+bs("│"))
	}
	box = append(box, bs("└"+strings.Repeat("─", lw)+"┴"+strings.Repeat("─", rw)+"┘"))
	return x, y, box
}

// searchGeom is where the parts of the search window sit.
type searchGeom struct {
	x, y, pw, ph int // the window on screen
	inner        int // cells between its borders
	lw, rw       int // the matched lines and the preview
	bl, br       int // the left and the right input field
	bodyH        int // rows of the matched lines and the preview
}

func (a *App) searchGeom() searchGeom {
	var g searchGeom
	g.pw = min(a.w, max(30, a.w-2))
	g.ph = min(max(a.h-2, 14), a.h)
	g.inner = g.pw - 2
	g.lw = (g.inner - 1) / 2
	g.rw = g.inner - 1 - g.lw
	avail := g.inner - 2
	g.bl = (avail - 1) / 2
	g.br = avail - 1 - g.bl
	const fixedRows = 11 // title, 2x3 field rows, toggles, status, divider, bottom
	g.bodyH = max(1, g.ph-fixedRows)
	g.x, g.y = (a.w-g.pw)/2, min(1, max(0, a.h-g.ph))
	return g
}

// fit brings the window up to date for the frame about to be drawn: where it
// is, the widths of the inputs, the scroll that follows the cursor and the
// file of the preview. renderSearch only reads them.
func (s *searchPanel) fit(a *App) {
	g := a.searchGeom()
	s.bodyH = g.bodyH
	s.rect = [4]int{g.x, g.y, g.pw, g.ph}
	// View() is one cell wider than Width when the cursor sits at the end.
	for i := range s.in {
		s.in[i].SetWidth(max(1, g.fieldW(i)-5))
	}
	if s.cursor < s.scroll {
		s.scroll = s.cursor
	}
	if s.cursor >= s.scroll+g.bodyH {
		s.scroll = s.cursor - g.bodyH + 1
	}
	s.scroll = clamp(s.scroll, 0, max(0, len(s.res.Hits)-g.bodyH))
	if hit := s.cur(); hit != nil {
		a.loadView(hit.Path)
	}
}

func (s *searchPanel) matchedRows(a *App, w, h int) []string {
	var out []string
	for i := s.scroll; i < len(s.res.Hits) && len(out) < h; i++ {
		out = append(out, a.matchedRow(s.res.Hits[i], w, i == s.cursor))
	}
	if len(s.res.Hits) == 0 && s.in[fQuery].Value() != "" && !s.busy && s.res.Err == nil {
		out = append(out, stPaletteDim.Render(fit(i18n.T("search.nolines"), w)))
	}
	for len(out) < h {
		out = append(out, stPalette.Render(fit("", w)))
	}
	return out
}

func (a *App) matchedRow(h *core.SearchHit, w int, selected bool) string {
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
	label := fmt.Sprintf("%s:%d", a.rel(h.Path), h.Row+1)
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

// loadView reads the file for the View area (from the open buffer if there is
// one) and its syntax colors.
func (a *App) loadView(path string) {
	v := &a.search.view
	if v.path == path {
		return
	}
	*v = searchView{path: path}
	for _, t := range a.tabs {
		if t.kind == tabText && t.buf.Path == path {
			v.lines, v.colors = t.buf.Lines, t.hl.Colors(t.buf)
			return
		}
	}
	if b, err := buffer.Open(path); err == nil {
		v.lines, v.colors = b.Lines, highlightNow(path, b)
	}
}

func (s *searchPanel) viewRows(a *App, w, h int) []string {
	blank := stPalette.Render(fit("", w))
	var out []string
	if hit := s.cur(); hit != nil && h > 0 {
		v := s.view // loaded in fit
		out = append(out, bgfg(cPaletteBg, curTheme.Heading).Bold(true).Render(fit(fmt.Sprintf(" %s:%d", a.rel(hit.Path), hit.Row+1), w)), blank)

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

// fieldW is the width of input field i: the left column holds the search and
// include fields, the right one the replace and exclude fields.
func (g searchGeom) fieldW(i int) int {
	if i == fRepl || i == fExclude {
		return g.br
	}
	return g.bl
}
