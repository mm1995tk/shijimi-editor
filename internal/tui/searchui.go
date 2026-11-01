package tui

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

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
func (a *Core) openSearch(field int) []Effect {
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
func (a *Core) openSearchArgs(arg string, replace bool) []Effect {
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
func (a *Core) openBufferLines() map[string][]string {
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
func (a *Core) searchNow() []Effect {
	return a.searchEffects(a.search.seq.Add(1))
}

// scheduleSearch starts a search once the inputs have been idle.
func (a *Core) scheduleSearch() []Effect {
	s := a.search
	seq := s.seq.Add(1)
	s.busy = true
	return []Effect{StartTimer{After: searchDebounce, Fire: SearchTick{seq}}}
}

// searchTick starts the search a debounce timer was waiting for, unless more
// was typed since or the window was closed.
func (a *Core) searchTick(m SearchTick) []Effect {
	if !a.search.open || m.seq != a.search.seq.Load() {
		return nil
	}
	return a.searchEffects(m.seq)
}

func (a *Core) searchEffects(seq int64) []Effect {
	s := a.search
	s.busy = true
	o, root, open := s.opts(), a.root, a.openBufferLines()
	cancelled := func() bool { return s.seq.Load() != seq }
	return []Effect{RunSearch{Run: func() Event {
		return SearchDone{seq: seq, res: core.RunSearch(root, o, open, cancelled)}
	}}}
}

func (a *Core) searchDone(m SearchDone) {
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

func (a *Core) handleSearchKey(msg Key) []Effect {
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
func (a *Core) searchInput(msg Input) []Effect {
	s := a.search
	before := s.in[s.field].Value()
	a.lineInputUpdate(&s.in[s.field], msg)
	if s.in[s.field].Value() != before {
		return a.scheduleSearch()
	}
	return nil
}

// searchOpenHit opens the selected line in the editor.
func (a *Core) searchOpenHit() []Effect {
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
func (a *Core) searchReplaceAll() []Effect {
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
func (a *Core) searchReplaceOne() []Effect {
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
func (a *Core) replaceHits(hits []*core.SearchHit) bool {
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

// firstColumn picks the first visible column so that the first match is in
// view within w cells.
func firstColumn(spans [][2]int, from, w int) int {
	if len(spans) == 0 || spans[0][1]-from < w {
		return from
	}
	return max(from, spans[0][0]-w/3)
}

func digits(n int) int { return len(fmt.Sprint(n)) }

// searchCaret is the screen cell where text typed into the focused field
// goes. It follows the layout of renderSearch and fieldBox.
func (a *Core) searchCaret() (x, y int, ok bool) {
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

// searchGeom is where the parts of the search window sit.
type searchGeom struct {
	x, y, pw, ph int // the window on screen
	inner        int // cells between its borders
	lw, rw       int // the matched lines and the preview
	bl, br       int // the left and the right input field
	bodyH        int // rows of the matched lines and the preview
}

func (a *Core) searchGeom() searchGeom {
	var g searchGeom
	g.pw = min(a.screenW(), max(30, a.screenW()-2))
	g.ph = min(max(a.screenH()-2, 14), a.screenH())
	g.inner = g.pw - 2
	g.lw = (g.inner - 1) / 2
	g.rw = g.inner - 1 - g.lw
	avail := g.inner - 2
	g.bl = (avail - 1) / 2
	g.br = avail - 1 - g.bl
	const fixedRows = 11 // title, 2x3 field rows, toggles, status, divider, bottom
	g.bodyH = max(1, g.ph-fixedRows)
	g.x, g.y = (a.screenW()-g.pw)/2, min(1, max(0, a.screenH()-g.ph))
	return g
}

// fit brings the window up to date for the frame about to be drawn: where it
// is, the widths of the inputs, the scroll that follows the cursor and the
// file of the preview. renderSearch only reads them.
func (s *searchPanel) fit(a *Core) {
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

// loadView reads the file for the View area (from the open buffer if there is
// one) and its syntax colors.
func (a *Core) loadView(path string) {
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

// fieldW is the width of input field i: the left column holds the search and
// include fields, the right one the replace and exclude fields.
func (g searchGeom) fieldW(i int) int {
	if i == fRepl || i == fExclude {
		return g.br
	}
	return g.bl
}
