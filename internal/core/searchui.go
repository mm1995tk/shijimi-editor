package core

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/config"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// searchDebounce is how long the inputs must be idle before a search starts.
const searchDebounce = 150 * time.Millisecond

// Condition fields, in Tab order.
const (
	FQuery = iota
	FRepl
	FInclude
	FExclude
	NumFields
)

func SearchLabels() [NumFields]string {
	return [NumFields]string{i18n.T("app.find"), i18n.T("search.replace"), i18n.T("search.include"), i18n.T("search.exclude")}
}

func SearchPlaceholders() [NumFields]string {
	return [NumFields]string{i18n.T("app.find"), i18n.T("search.replace"), i18n.T("search.includeph"), i18n.T("search.excludeph")}
}

// searchView caches the file shown in the View area.
type searchView struct {
	path   string
	Lines  [][]rune
	Colors [][]string
}

// searchPanel is the floating project search / replace window: conditions on
// top, matched lines at the lower left, a preview of the selected one at the
// lower right.
type SearchPanel struct {
	Open  bool
	In    [NumFields]InputField
	Field int

	CaseSens, Word, Regex, KeepCase bool

	seq     atomic.Int64 // id of the latest requested search
	Busy    bool
	Res     SearchResult
	Skipped map[string]bool // lines left out of the replacement, by path and row

	Cursor, Scroll int
	restore        bool // after the next search, put the cursor at restoreAt
	restoreAt      int
	restoreKey     string // the replaced line; if it still matches, move past it
	BodyH          int
	rect           [4]int // x, y, w, h on screen
	View           searchView
}

func newSearchPanel(def config.Search) *SearchPanel {
	s := &SearchPanel{Skipped: map[string]bool{}}
	for i := range s.In {
		s.In[i] = NewInputField(4096)
	}
	s.In[FInclude].SetValue(strings.Join(def.Include, ","))
	s.In[FExclude].SetValue(strings.Join(def.Exclude, ","))
	return s
}

func (s *SearchPanel) opts() SearchOpts {
	return SearchOpts{
		Query:    s.In[FQuery].Value(),
		Repl:     s.In[FRepl].Value(),
		Include:  splitGlobs(s.In[FInclude].Value()),
		Exclude:  splitGlobs(s.In[FExclude].Value()),
		CaseSens: s.CaseSens,
		Word:     s.Word,
		Regex:    s.Regex,
		KeepCase: s.KeepCase,
	}
}

func (s *SearchPanel) FocusField(f int) []Effect {
	s.Field = (f + NumFields) % NumFields
	for i := range s.In {
		if i == s.Field {
			s.In[i].Focus()
		} else {
			s.In[i].Blur()
		}
	}
	return nil
}

func (s *SearchPanel) close() {
	s.Open = false
	for i := range s.In {
		s.In[i].Blur()
	}
}

func (s *SearchPanel) Cur() *SearchHit {
	if s.Cursor < 0 || s.Cursor >= len(s.Res.Hits) {
		return nil
	}
	return s.Res.Hits[s.Cursor]
}

func (s *SearchPanel) move(d int) {
	s.Cursor = Clamp(s.Cursor+d, 0, max(0, len(s.Res.Hits)-1))
}

func skipKey(h *SearchHit) string { return fmt.Sprintf("%s\x00%d", h.Path, h.Row) }

func (s *SearchPanel) toggleSkip() {
	if h := s.Cur(); h != nil {
		h.Skip = !h.Skip
		if h.Skip {
			s.Skipped[skipKey(h)] = true
		} else {
			delete(s.Skipped, skipKey(h))
		}
	}
}

// counts returns the number of lines that will be replaced and the number
// left out.
func (s *SearchPanel) Counts() (target, skipped int) {
	for _, h := range s.Res.Hits {
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
func (a *Core) OpenSearch(field int) []Effect {
	if a.single {
		return nil
	}
	s := a.Search
	s.Open = true
	cmds := [][]Effect{s.FocusField(field)}
	if s.In[FQuery].Value() != "" {
		cmds = append(cmds, a.searchNow())
	}
	return Batch(cmds...)
}

// openSearchArgs opens the window for `:grep [-x globs] [-i globs] query` or
// (replace) `:replace [-x globs] [-i globs] query => replacement`.
func (a *Core) OpenSearchArgs(arg string, replace bool) []Effect {
	field := FQuery
	if replace {
		field = FRepl
	}
	if strings.TrimSpace(arg) == "" {
		return a.OpenSearch(field)
	}
	s := a.Search
	body, repl := arg, ""
	if replace {
		body, repl, _ = SplitReplaceArgs(arg)
	}
	q, include, exclude := parseGrepArgs(body, a.Cfg.Search)
	s.In[FQuery].SetValue(q)
	s.In[FRepl].SetValue(repl)
	s.In[FInclude].SetValue(strings.Join(include, ","))
	s.In[FExclude].SetValue(strings.Join(exclude, ","))
	s.Regex = false // the command line always searches for the text itself
	s.Skipped = map[string]bool{}
	return a.OpenSearch(field)
}

// ---- searching ----

// openBufferLines returns the lines of the open text buffers; they may differ
// from the files on disk.
func (a *Core) openBufferLines() map[string][]string {
	open := map[string][]string{}
	for _, t := range a.Tabs {
		if t.Kind == TabText && t.Buf.Path != "" {
			lines := make([]string, len(t.Buf.Lines))
			for i, l := range t.Buf.Lines {
				lines[i] = string(l)
			}
			open[t.Buf.Path] = lines
		}
	}
	return open
}

// searchNow starts a search immediately.
func (a *Core) searchNow() []Effect {
	return a.searchEffects(a.Search.seq.Add(1))
}

// scheduleSearch starts a search once the inputs have been idle.
func (a *Core) scheduleSearch() []Effect {
	s := a.Search
	seq := s.seq.Add(1)
	s.Busy = true
	return []Effect{StartTimer{After: searchDebounce, Fire: SearchTick{seq}}}
}

// searchTick starts the search a debounce timer was waiting for, unless more
// was typed since or the window was closed.
func (a *Core) searchTick(m SearchTick) []Effect {
	if !a.Search.Open || m.seq != a.Search.seq.Load() {
		return nil
	}
	return a.searchEffects(m.seq)
}

func (a *Core) searchEffects(seq int64) []Effect {
	s := a.Search
	s.Busy = true
	o, root, open := s.opts(), a.Root, a.openBufferLines()
	cancelled := func() bool { return s.seq.Load() != seq }
	return []Effect{RunSearch{Run: func() Event {
		return SearchDone{seq: seq, res: SearchProject(root, o, open, cancelled)}
	}}}
}

func (a *Core) searchDone(m SearchDone) {
	s := a.Search
	if m.seq != s.seq.Load() {
		return
	}
	var keep string
	if h := s.Cur(); h != nil {
		keep = skipKey(h)
	}
	s.Busy = false
	s.Res = m.res
	s.Cursor = 0
	for i, h := range s.Res.Hits {
		h.Skip = s.Skipped[skipKey(h)]
		if keep != "" && skipKey(h) == keep {
			s.Cursor = i
		}
	}
	if s.restore {
		at := s.restoreAt
		if at < len(s.Res.Hits) && skipKey(s.Res.Hits[at]) == s.restoreKey {
			at++
		}
		s.Cursor = Clamp(at, 0, max(0, len(s.Res.Hits)-1))
		s.restore = false
	}
	s.View.path = ""
}

// ---- keys ----

func (a *Core) HandleSearchKey(msg Key) []Effect {
	s := a.Search
	switch key := msg.String(); key {
	case "esc", "ctrl+c":
		s.close()
		return nil
	case "tab":
		return s.FocusField(s.Field + 1)
	case "shift+tab":
		return s.FocusField(s.Field - 1)
	case "up", "ctrl+p":
		s.move(-1)
		return nil
	case "down", "ctrl+n":
		s.move(1)
		return nil
	case "pgup":
		s.move(-max(1, s.BodyH-1))
		return nil
	case "pgdown":
		s.move(max(1, s.BodyH-1))
		return nil
	case "ctrl+x", "alt+x":
		s.toggleSkip()
		return nil
	case "alt+c":
		s.CaseSens = !s.CaseSens
		return a.searchNow()
	case "alt+w":
		s.Word = !s.Word
		return a.searchNow()
	case "alt+r":
		s.Regex = !s.Regex
		return a.searchNow()
	case "alt+p":
		s.KeepCase = !s.KeepCase
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
	s := a.Search
	before := s.In[s.Field].Value()
	a.lineInputUpdate(&s.In[s.Field], msg)
	if s.In[s.Field].Value() != before {
		return a.scheduleSearch()
	}
	return nil
}

// searchOpenHit opens the selected line in the editor.
func (a *Core) searchOpenHit() []Effect {
	s := a.Search
	h := s.Cur()
	if h == nil {
		return nil
	}
	q, regex := s.In[FQuery].Value(), s.Regex
	s.close()
	a.pushJump()
	if !a.OpenFile(h.Path) {
		return nil
	}
	if t := a.CurText(); t != nil {
		t.Buf.GotoLine(h.Row + 1)
		t.Buf.Cur.Col = min(h.Spans[0][0], len(t.Buf.Lines[t.Buf.Cur.Row]))
		if !regex {
			a.lastSearch = q
		}
	}
	a.Follow = true
	return a.SetFocus(FocusEditor)
}

// searchReplaceAll replaces the checked lines. Every affected file is opened
// as a buffer and edited in memory as one undo step; nothing is written to
// disk until the user saves, so Ctrl+Z / u reverts the whole replacement.
func (a *Core) searchReplaceAll() []Effect {
	s := a.Search
	if s.Busy {
		a.Fail(errors.New(i18n.T("search.busy")))
		return nil
	}
	if s.Res.Truncated {
		a.Fail(fmt.Errorf(i18n.T("search.toomany"), MaxSearchHits))
		return nil
	}
	if n, _ := s.Counts(); n == 0 {
		a.Fail(errors.New(i18n.T("search.nothingtoreplace")))
		return nil
	}
	var hits []*SearchHit
	for _, h := range s.Res.Hits {
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
	s := a.Search
	if s.Busy {
		a.Fail(errors.New(i18n.T("search.busy")))
		return nil
	}
	h := s.Cur()
	if h == nil {
		a.Fail(errors.New(i18n.T("search.nothingtoreplace")))
		return nil
	}
	if h.Skip {
		// picking a line to replace is explicit, so it overrides the exclusion
		h.Skip = false
		delete(s.Skipped, skipKey(h))
	}
	idx, key := s.Cursor, skipKey(h)
	if !a.replaceHits([]*SearchHit{h}) {
		return nil
	}
	s.restore, s.restoreAt, s.restoreKey = true, idx, key
	return a.searchNow()
}

// replaceHits applies the replacement to hits, opening each affected file as
// a buffer. It reports whether every file could be opened.
func (a *Core) replaceHits(hits []*SearchHit) bool {
	var paths []string
	byPath := map[string][]*SearchHit{}
	for _, h := range hits {
		if _, ok := byPath[h.Path]; !ok {
			paths = append(paths, h.Path)
		}
		byPath[h.Path] = append(byPath[h.Path], h)
	}
	var failed []string
	done, stale, changed := 0, 0, 0
	for _, p := range paths {
		if !a.OpenFile(p) {
			failed = append(failed, a.Rel(p))
			continue
		}
		t := a.CurText()
		if t == nil {
			continue
		}
		edits, n, st := HitEdits(byPath[p], t.Buf.Lines)
		stale += st
		if len(edits) > 0 {
			t.Buf.ApplyEdits(edits)
			done += n
			changed++
		}
	}
	if len(failed) > 0 {
		a.Fail(fmt.Errorf(i18n.T("search.openfail"), strings.Join(failed, ", ")))
		return false
	}
	msg := fmt.Sprintf(i18n.T("search.replaced"), done, changed)
	if stale > 0 {
		a.Msg, a.MsgErr = fmt.Sprintf(i18n.T("search.skipped"), msg, stale), true
		return true
	}
	a.Info("%s", msg)
	return true
}

// ---- rendering ----

// firstColumn picks the first visible column so that the first match is in
// view within w cells.
func FirstColumn(spans [][2]int, from, w int) int {
	if len(spans) == 0 || spans[0][1]-from < w {
		return from
	}
	return max(from, spans[0][0]-w/3)
}

func Digits(n int) int { return len(fmt.Sprint(n)) }

// searchCaret is the screen cell where text typed into the focused field
// goes. It follows the layout of renderSearch and fieldBox.
func (a *Core) SearchCaret() (x, y int, ok bool) {
	s := a.Search
	if s.rect[2] == 0 {
		return 0, 0, false
	}
	// window border, gap, field border, padding
	x, y = s.rect[0]+4, s.rect[1]+2
	if s.Field == FRepl || s.Field == FExclude {
		x += (s.rect[2]-5)/2 + 1
	}
	if s.Field == FInclude || s.Field == FExclude {
		y += 3
	}
	if s.In[s.Field].Value() != "" {
		x += s.In[s.Field].Caret()
	}
	return x, y, true
}

// searchGeom is where the parts of the search window sit.
type SearchGeom struct {
	X, Y, Pw, ph int // the window on screen
	Inner        int // cells between its borders
	Lw, Rw       int // the matched lines and the preview
	Bl, Br       int // the left and the right input field
	bodyH        int // rows of the matched lines and the preview
}

func (a *Core) SearchGeom() SearchGeom {
	var g SearchGeom
	g.Pw = min(a.ScreenW(), max(30, a.ScreenW()-2))
	g.ph = min(max(a.ScreenH()-2, 14), a.ScreenH())
	g.Inner = g.Pw - 2
	g.Lw = (g.Inner - 1) / 2
	g.Rw = g.Inner - 1 - g.Lw
	avail := g.Inner - 2
	g.Bl = (avail - 1) / 2
	g.Br = avail - 1 - g.Bl
	const fixedRows = 11 // title, 2x3 field rows, toggles, status, divider, bottom
	g.bodyH = max(1, g.ph-fixedRows)
	g.X, g.Y = (a.ScreenW()-g.Pw)/2, min(1, max(0, a.ScreenH()-g.ph))
	return g
}

// fit brings the window up to date for the frame about to be drawn: where it
// is, the widths of the inputs, the scroll that follows the cursor and the
// file of the preview. renderSearch only reads them.
func (s *SearchPanel) fit(a *Core) {
	g := a.SearchGeom()
	s.BodyH = g.bodyH
	s.rect = [4]int{g.X, g.Y, g.Pw, g.ph}
	// View() is one cell wider than Width when the cursor sits at the end.
	for i := range s.In {
		s.In[i].SetWidth(max(1, g.fieldW(i)-5))
	}
	if s.Cursor < s.Scroll {
		s.Scroll = s.Cursor
	}
	if s.Cursor >= s.Scroll+g.bodyH {
		s.Scroll = s.Cursor - g.bodyH + 1
	}
	s.Scroll = Clamp(s.Scroll, 0, max(0, len(s.Res.Hits)-g.bodyH))
	if hit := s.Cur(); hit != nil {
		a.loadView(hit.Path)
	}
}

// loadView reads the file for the View area (from the open buffer if there is
// one) and its syntax colors.
func (a *Core) loadView(path string) {
	v := &a.Search.View
	if v.path == path {
		return
	}
	*v = searchView{path: path}
	for _, t := range a.Tabs {
		if t.Kind == TabText && t.Buf.Path == path {
			v.Lines, v.Colors = t.Buf.Lines, t.Hl.Colors(t.Buf)
			return
		}
	}
	if b, err := buffer.Open(path); err == nil {
		v.Lines, v.Colors = b.Lines, highlightNow(path, b)
	}
}

// fieldW is the width of input field i: the left column holds the search and
// include fields, the right one the replace and exclude fields.
func (g SearchGeom) fieldW(i int) int {
	if i == FRepl || i == FExclude {
		return g.Br
	}
	return g.Bl
}
