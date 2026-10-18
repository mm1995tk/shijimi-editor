package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/gh-editor/internal/buffer"
	"github.com/mm1995tk/gh-editor/internal/config"
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

var searchLabels = [numFields]string{"検索", "置換", "含める", "除外"}

var searchPlaceholders = [numFields]string{"検索", "置換", "含めるファイル（例: *.go）", "除外するファイル（例: vendor/**）"}

type searchTickMsg struct{ seq int64 }

type searchDoneMsg struct {
	seq int64
	res searchResult
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
	in    [numFields]textinput.Model
	field int

	caseSens, word, regex, keepCase bool

	seq     atomic.Int64 // id of the latest requested search
	busy    bool
	res     searchResult
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
		ti := textinput.New()
		ti.Prompt = ""
		ti.CharLimit = 4096
		s.in[i] = ti
	}
	s.in[fInclude].SetValue(strings.Join(def.Include, ","))
	s.in[fExclude].SetValue(strings.Join(def.Exclude, ","))
	return s
}

func (s *searchPanel) restyle() {
	for i := range s.in {
		styleInput(&s.in[i], cPaletteBg)
	}
}

func (s *searchPanel) opts() searchOpts {
	return searchOpts{
		query:    s.in[fQuery].Value(),
		repl:     s.in[fRepl].Value(),
		include:  splitGlobs(s.in[fInclude].Value()),
		exclude:  splitGlobs(s.in[fExclude].Value()),
		caseSens: s.caseSens,
		word:     s.word,
		regex:    s.regex,
		keepCase: s.keepCase,
	}
}

func (s *searchPanel) focusField(f int) tea.Cmd {
	s.field = (f + numFields) % numFields
	var cmd tea.Cmd
	for i := range s.in {
		if i == s.field {
			cmd = s.in[i].Focus()
		} else {
			s.in[i].Blur()
		}
	}
	return cmd
}

func (s *searchPanel) close() {
	s.open = false
	for i := range s.in {
		s.in[i].Blur()
	}
}

func (s *searchPanel) cur() *searchHit {
	if s.cursor < 0 || s.cursor >= len(s.res.hits) {
		return nil
	}
	return s.res.hits[s.cursor]
}

func (s *searchPanel) move(d int) {
	s.cursor = clamp(s.cursor+d, 0, max(0, len(s.res.hits)-1))
}

func skipKey(h *searchHit) string { return fmt.Sprintf("%s\x00%d", h.path, h.row) }

func (s *searchPanel) toggleSkip() {
	if h := s.cur(); h != nil {
		h.skip = !h.skip
		if h.skip {
			s.skipped[skipKey(h)] = true
		} else {
			delete(s.skipped, skipKey(h))
		}
	}
}

// counts returns the number of lines that will be replaced and the number
// left out.
func (s *searchPanel) counts() (target, skipped int) {
	for _, h := range s.res.hits {
		if h.skip {
			skipped++
		} else {
			target++
		}
	}
	return
}

// ---- opening ----

// openSearch shows the window with the conditions of the last use.
func (a *App) openSearch(field int) tea.Cmd {
	s := a.search
	s.open = true
	cmds := []tea.Cmd{s.focusField(field)}
	if s.in[fQuery].Value() != "" {
		cmds = append(cmds, a.searchNow())
	}
	return tea.Batch(cmds...)
}

// openSearchArgs opens the window for `:grep [-x globs] [-i globs] query` or
// (replace) `:replace [-x globs] [-i globs] query => replacement`.
func (a *App) openSearchArgs(arg string, replace bool) tea.Cmd {
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
		body, repl, _ = splitReplaceArgs(arg)
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
func (a *App) searchNow() tea.Cmd {
	s := a.search
	return a.searchCmd(s.seq.Add(1))
}

// scheduleSearch starts a search once the inputs have been idle.
func (a *App) scheduleSearch() tea.Cmd {
	s := a.search
	seq := s.seq.Add(1)
	s.busy = true
	return tea.Tick(searchDebounce, func(time.Time) tea.Msg { return searchTickMsg{seq} })
}

func (a *App) searchTick(m searchTickMsg) tea.Cmd {
	if !a.search.open || m.seq != a.search.seq.Load() {
		return nil
	}
	return a.searchCmd(m.seq)
}

func (a *App) searchCmd(seq int64) tea.Cmd {
	s := a.search
	s.busy = true
	o, root, open := s.opts(), a.root, a.openBufferLines()
	cancelled := func() bool { return s.seq.Load() != seq }
	return func() tea.Msg {
		return searchDoneMsg{seq: seq, res: runSearch(root, o, open, cancelled)}
	}
}

func (a *App) searchDone(m searchDoneMsg) {
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
	for i, h := range s.res.hits {
		h.skip = s.skipped[skipKey(h)]
		if keep != "" && skipKey(h) == keep {
			s.cursor = i
		}
	}
	if s.restore {
		at := s.restoreAt
		if at < len(s.res.hits) && skipKey(s.res.hits[at]) == s.restoreKey {
			at++
		}
		s.cursor = clamp(at, 0, max(0, len(s.res.hits)-1))
		s.restore = false
	}
	s.view.path = ""
}

// ---- keys ----

func (a *App) handleSearchKey(msg tea.KeyMsg) tea.Cmd {
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
	before := s.in[s.field].Value()
	var cmd tea.Cmd
	s.in[s.field], cmd = s.in[s.field].Update(msg)
	if s.in[s.field].Value() != before {
		return tea.Batch(cmd, a.scheduleSearch())
	}
	return cmd
}

func (a *App) searchMouse(msg tea.MouseMsg) tea.Cmd {
	s := a.search
	if msg.Action != tea.MouseActionPress {
		return nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		s.move(-3)
	case tea.MouseButtonWheelDown:
		s.move(3)
	case tea.MouseButtonLeft:
		r := s.rect
		if msg.X < r[0] || msg.X >= r[0]+r[2] || msg.Y < r[1] || msg.Y >= r[1]+r[3] {
			s.close()
		}
	}
	return nil
}

// searchOpenHit opens the selected line in the editor.
func (a *App) searchOpenHit() tea.Cmd {
	s := a.search
	h := s.cur()
	if h == nil {
		return nil
	}
	q, regex := s.in[fQuery].Value(), s.regex
	s.close()
	a.pushJump()
	if !a.openFile(h.path) {
		return nil
	}
	if t := a.curText(); t != nil {
		t.buf.GotoLine(h.row + 1)
		t.buf.Cur.Col = min(h.spans[0][0], len(t.buf.Lines[t.buf.Cur.Row]))
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
func (a *App) searchReplaceAll() tea.Cmd {
	s := a.search
	if s.busy {
		a.fail(fmt.Errorf("検索中です。結果が出てから実行してください"))
		return nil
	}
	if s.res.truncated {
		a.fail(fmt.Errorf("一致が %d 件を超えているため全置換できません。条件を絞ってください", maxSearchHits))
		return nil
	}
	if n, _ := s.counts(); n == 0 {
		a.fail(fmt.Errorf("置換する行がありません"))
		return nil
	}
	var hits []*searchHit
	for _, h := range s.res.hits {
		if !h.skip {
			hits = append(hits, h)
		}
	}
	s.close()
	a.replaceHits(hits)
	return nil
}

// searchReplaceOne replaces only the selected line and keeps the panel open,
// re-running the search so the cursor lands on the next hit.
func (a *App) searchReplaceOne() tea.Cmd {
	s := a.search
	if s.busy {
		a.fail(fmt.Errorf("検索中です。結果が出てから実行してください"))
		return nil
	}
	h := s.cur()
	if h == nil {
		a.fail(fmt.Errorf("置換する行がありません"))
		return nil
	}
	if h.skip {
		// picking a line to replace is explicit, so it overrides the exclusion
		h.skip = false
		delete(s.skipped, skipKey(h))
	}
	idx, key := s.cursor, skipKey(h)
	if !a.replaceHits([]*searchHit{h}) {
		return nil
	}
	s.restore, s.restoreAt, s.restoreKey = true, idx, key
	return a.searchNow()
}

// replaceHits applies the replacement to hits, opening each affected file as
// a buffer. It reports whether every file could be opened.
func (a *App) replaceHits(hits []*searchHit) bool {
	var paths []string
	byPath := map[string][]*searchHit{}
	for _, h := range hits {
		if _, ok := byPath[h.path]; !ok {
			paths = append(paths, h.path)
		}
		byPath[h.path] = append(byPath[h.path], h)
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
		edits, n, st := hitEdits(byPath[p], t.buf.Lines)
		stale += st
		if len(edits) > 0 {
			t.buf.ApplyEdits(edits)
			done += n
			changed++
		}
	}
	if len(failed) > 0 {
		a.fail(fmt.Errorf("開けなかったファイル: %s", strings.Join(failed, ", ")))
		return false
	}
	msg := fmt.Sprintf("%d 行 (%d ファイル) を置換しました。保存するまでディスクは変わりません（u で元に戻せます）", done, changed)
	if stale > 0 {
		a.msg, a.msgErr = fmt.Sprintf("%s。検索後に内容が変わった %d 行はスキップしました", msg, stale), true
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
	case s.res.err != nil:
		return bgfg(cPaletteBg, cError).Render(" " + firstLine(s.res.err.Error()))
	case s.in[fQuery].Value() == "":
		return stPaletteDim.Render(" 検索する文字列を入力してください")
	case s.busy:
		return stPaletteDim.Render(" 検索中…")
	}
	more := ""
	if s.res.truncated {
		more = " 以上"
	}
	out := fmt.Sprintf(" %d ファイル / %d 件%s", s.res.files, len(s.res.hits), more)
	if target, skipped := s.counts(); s.in[fRepl].Value() != "" || skipped > 0 {
		out += fmt.Sprintf(" ・ 置換対象 %d 件（%d 件を除外）", target, skipped)
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
	return stPalette.Render(" ") + t(s.caseSens, "[Aa] 大文字小文字") + sp + t(s.word, "[ab] 単語") + sp +
		t(s.regex, "[.*] 正規表現") + sp + t(s.keepCase, "[AB] 大文字小文字を保持")
}

// fieldBox draws input field i as a three-row box whose top edge carries the
// label. The focused field gets a bold border in the strong colour.
func (s *searchPanel) fieldBox(i, w int) [3]string {
	bd := bgfg(cPaletteBg, cBorder)
	if i == s.field {
		bd = bgfg(cPaletteBg, curTheme.Strong).Bold(true)
	}
	title := " " + searchLabels[i] + " "
	// View() is one cell wider than Width when the cursor sits at the end.
	s.in[i].Width = max(1, w-5)
	body := s.in[i].View()
	if s.in[i].Value() == "" {
		body = s.placeholderView(i)
	}
	return [3]string{
		bd.Render("┌─" + title + strings.Repeat("─", max(0, w-3-ansi.StringWidth(title))) + "┐"),
		bd.Render("│") + stPalette.Render(" ") + fitStyled(body, w-4, stPalette) + stPalette.Render(" ") + bd.Render("│"),
		bd.Render("└" + strings.Repeat("─", max(0, w-2)) + "┘"),
	}
}

// renderSearch draws the window and returns its screen position. The window
// fills the screen except for a one-cell margin on every side.
func (a *App) renderSearch() (x, y int, box []string) {
	s := a.search
	pw := min(a.w, max(30, a.w-2))
	ph := min(max(a.h-2, 14), a.h)
	inner := pw - 2
	lw := (inner - 1) / 2
	rw := inner - 1 - lw
	const fixedRows = 11 // title, 2x3 field rows, toggles, status, divider, bottom
	s.bodyH = max(1, ph-fixedRows)
	x, y = (a.w-pw)/2, min(1, max(0, a.h-ph))
	s.rect = [4]int{x, y, pw, ph}

	bs := func(t string) string { return stPaletteBox.Render(t) }
	line := func(content string) string { return bs("│") + content + bs("│") }

	title := " 全文検索 "
	box = append(box, bs("┌─"+title+strings.Repeat("─", max(0, pw-3-ansi.StringWidth(title)))+"┐"))

	// Two columns: search | replace, then include | exclude.
	avail := inner - 2
	bl := (avail - 1) / 2
	br := avail - 1 - bl
	sp := stPalette.Render(" ")
	for _, pair := range [][2]int{{fQuery, fRepl}, {fInclude, fExclude}} {
		l, r := s.fieldBox(pair[0], bl), s.fieldBox(pair[1], br)
		for k := range l {
			box = append(box, line(sp+l[k]+sp+r[k]+sp))
		}
	}
	box = append(box, line(fitStyled(s.togglesLine(), inner, stPalette)))

	left, help := s.statusLine(), stPaletteDim.Render("Tab 次の欄  Alt+C/W/R/P 切替  Ctrl+X 除外  Alt+1 この行を置換  Alt+Enter 全置換")
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

func (s *searchPanel) matchedRows(a *App, w, h int) []string {
	if s.cursor < s.scroll {
		s.scroll = s.cursor
	}
	if s.cursor >= s.scroll+h {
		s.scroll = s.cursor - h + 1
	}
	s.scroll = clamp(s.scroll, 0, max(0, len(s.res.hits)-h))
	var out []string
	for i := s.scroll; i < len(s.res.hits) && len(out) < h; i++ {
		out = append(out, a.matchedRow(s.res.hits[i], w, i == s.cursor))
	}
	if len(s.res.hits) == 0 && s.in[fQuery].Value() != "" && !s.busy && s.res.err == nil {
		out = append(out, stPaletteDim.Render(fit("  一致する行はありません", w)))
	}
	for len(out) < h {
		out = append(out, stPalette.Render(fit("", w)))
	}
	return out
}

func (a *App) matchedRow(h *searchHit, w int, selected bool) string {
	bg, fg, dim := cPaletteBg, cEditorFg, cLineNo
	if selected {
		bg, fg, dim = cListSel, curTheme.Strong, curTheme.SubSel
	}
	marker := "  "
	switch {
	case h.skip:
		marker = "x "
	case selected:
		marker = "> "
	}
	label := fmt.Sprintf("%s:%d", a.rel(h.path), h.row+1)
	if maxL := max(12, w*3/5); ansi.StringWidth(label) > maxL {
		label = "…" + ansi.TruncateLeft(label, ansi.StringWidth(label)-maxL+1, "")
	}
	prefix := marker + label + "  "
	rs := []rune(h.line)
	lead := 0
	for lead < len(rs) && (rs[lead] == ' ' || rs[lead] == '\t') {
		lead++
	}
	if len(h.spans) > 0 {
		lead = min(lead, h.spans[0][0])
	}
	avail := max(1, w-ansi.StringWidth(prefix))
	return bgfg(bg, dim).Render(prefix) + styledRunes(rs, nil, firstColumn(h.spans, lead, avail), avail, bg, fg, h.spans)
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
		v.lines, v.colors = b.Lines, newHighlighter(filepath.Base(path)).Colors(b)
	}
}

func (s *searchPanel) viewRows(a *App, w, h int) []string {
	blank := stPalette.Render(fit("", w))
	var out []string
	if hit := s.cur(); hit != nil && h > 0 {
		a.loadView(hit.path)
		v := s.view
		out = append(out, bgfg(cPaletteBg, curTheme.Heading).Bold(true).Render(fit(fmt.Sprintf(" %s:%d", a.rel(hit.path), hit.row+1), w)), blank)

		nd := max(3, digits(len(v.lines)))
		cw := max(1, w-nd-3)
		gutter := bgfg(cPaletteBg, cLineNo)
		xoff := firstColumn(hit.spans, 0, cw)
		diff := s.in[fRepl].Value() != "" && hit.new != hit.line
		budget := max(1, h-2)
		if diff {
			budget = max(1, budget-2)
		}
		start := max(0, hit.row-budget/2)
		end := min(len(v.lines), start+budget)
		start = max(0, end-budget)
		for r := start; r < end; r++ {
			var colors []string
			if r < len(v.colors) {
				colors = v.colors[r]
			}
			var spans [][2]int
			if r == hit.row {
				spans = hit.spans
			}
			out = append(out, gutter.Render(fmt.Sprintf("%*d │ ", nd, r+1))+styledRunes(v.lines[r], colors, xoff, cw, cPaletteBg, cEditorFg, spans))
			if r == hit.row && diff {
				out = append(out,
					gutter.Render(fmt.Sprintf("%*s - ", nd, ""))+styledRunes([]rune(hit.line), nil, xoff, cw, cDelBg, cEditorFg, nil),
					gutter.Render(fmt.Sprintf("%*s + ", nd, ""))+styledRunes([]rune(hit.new), nil, xoff, cw, cAddBg, cEditorFg, nil))
			}
		}
	}
	for len(out) < h {
		out = append(out, blank)
	}
	return out[:h]
}

// placeholderView draws the hint of an empty field with the cursor on its
// first character. textinput's own placeholder is not used: it pads with NULs
// when the text has wide characters.
func (s *searchPanel) placeholderView(i int) string {
	rs := []rune(searchPlaceholders[i])
	c := s.in[i].Cursor
	c.TextStyle = stPaletteDim
	c.SetChar(string(rs[0]))
	return c.View() + stPaletteDim.Render(string(rs[1:]))
}
