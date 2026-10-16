package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/gh-editor/internal/buffer"
	"github.com/mm1995tk/gh-editor/internal/config"
	"github.com/mm1995tk/gh-editor/internal/lsp"
)

const lspTimeout = 10 * time.Second

// lspManager starts language servers on demand and keeps diagnostics.
type lspManager struct {
	cfg     *config.Config
	root    string
	clients map[string]*lsp.Client // key: server name + "\x00" + root
	failed  map[string]bool
	events  chan lsp.Event
	diags   map[string]map[string][]lsp.Diagnostic // path -> server -> diags
}

func newLSPManager(cfg *config.Config, root string) *lspManager {
	return &lspManager{
		cfg: cfg, root: root,
		clients: map[string]*lsp.Client{},
		failed:  map[string]bool{},
		events:  make(chan lsp.Event, 256),
		diags:   map[string]map[string][]lsp.Diagnostic{},
	}
}

type lspEventMsg lsp.Event

func (m *lspManager) wait() tea.Cmd {
	return func() tea.Msg { return lspEventMsg(<-m.events) }
}

// clientsFor returns running clients for a tab, starting them if needed.
// Errors (e.g. server binary missing) are returned once per server.
func (m *lspManager) clientsFor(t *tab) ([]*lsp.Client, []error) {
	if t.kind != tabText || t.lang == nil || t.buf.Path == "" {
		return nil, nil
	}
	var out []*lsp.Client
	var errs []error
	root := t.lang.FindRoot(t.buf.Path, m.root)
	for _, name := range t.lang.Servers {
		key := name + "\x00" + root
		if c, ok := m.clients[key]; ok {
			out = append(out, c)
			continue
		}
		if m.failed[key] {
			continue
		}
		s := m.cfg.Servers[name]
		if s == nil {
			m.failed[key] = true
			errs = append(errs, fmt.Errorf("言語サーバー %q の設定がありません", name))
			continue
		}
		c, err := lsp.Start(s, root, m.events)
		if err != nil {
			m.failed[key] = true
			errs = append(errs, err)
			continue
		}
		m.clients[key] = c
		out = append(out, c)
	}
	return out, errs
}

// client returns the first client for t that supports capability.
func (m *lspManager) client(t *tab, capability string) *lsp.Client {
	cs, _ := m.clientsFor(t)
	for _, c := range cs {
		if c.Supports(capability) {
			return c
		}
	}
	return nil
}

func (m *lspManager) diagnostics(path string) []lsp.Diagnostic {
	var out []lsp.Diagnostic
	for _, ds := range m.diags[path] {
		out = append(out, ds...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Range.Start, out[j].Range.Start
		return a.Line < b.Line || (a.Line == b.Line && a.Character < b.Character)
	})
	return out
}

func (m *lspManager) stopAll() {
	for _, c := range m.clients {
		c.Stop()
	}
}

func (m *lspManager) restart() {
	m.stopAll()
	m.clients = map[string]*lsp.Client{}
	m.failed = map[string]bool{}
	m.diags = map[string]map[string][]lsp.Diagnostic{}
}

// ---- app integration ----

func lspPos(b *buffer.Buffer, p buffer.Pos) lsp.Position {
	return lsp.Position{Line: p.Row, Character: lsp.ToUTF16(b.Lines[p.Row], p.Col)}
}

func bufPos(b *buffer.Buffer, p lsp.Position) buffer.Pos {
	row := clamp(p.Line, 0, len(b.Lines)-1)
	return buffer.Pos{Row: row, Col: lsp.FromUTF16(b.Lines[row], p.Character)}
}

func toEdits(b *buffer.Buffer, tes []lsp.TextEdit) []buffer.Edit {
	out := make([]buffer.Edit, len(tes))
	for i, te := range tes {
		out[i] = buffer.Edit{Start: bufPos(b, te.Range.Start), End: bufPos(b, te.Range.End), Text: te.NewText}
	}
	return out
}

// attachLSP configures a newly opened text tab for its language and opens
// it on the language servers.
func (a *App) attachLSP(t *tab) {
	if t.kind != tabText || t.buf.Path == "" {
		return
	}
	t.lang = a.cfg.LanguageFor(t.buf.Path)
	if t.lang == nil {
		return
	}
	if t.lang.IndentUnit != "" {
		t.buf.SetIndentUnit(t.lang.IndentUnit)
	}
	cs, errs := a.lsp.clientsFor(t)
	for _, err := range errs {
		a.fail(err)
	}
	for _, c := range cs {
		c.DidOpen(t.buf.Path, t.lang.ID(), t.buf.Text(), t.buf.Version+1)
	}
	t.lspVersion = t.buf.Version
}

func (a *App) detachLSP(t *tab) {
	cs, _ := a.lsp.clientsFor(t)
	for _, c := range cs {
		c.DidClose(t.buf.Path)
	}
}

// syncLSP sends didChange for buffers edited since the last sync.
func (a *App) syncLSP() {
	for _, t := range a.tabs {
		if t.kind != tabText || t.lang == nil || t.buf.Version == t.lspVersion {
			continue
		}
		cs, _ := a.lsp.clientsFor(t)
		text := t.buf.Text()
		for _, c := range cs {
			c.DidChange(t.buf.Path, text, t.buf.Version+1)
		}
		t.lspVersion = t.buf.Version
	}
}

type hoverMsg struct{ text string }
type locationsMsg struct {
	title string
	locs  []lsp.Location
}
type completionMsg struct {
	tab     *tab
	version int
	start   buffer.Pos
	items   []lsp.CompletionItem
}
type workspaceEditMsg struct{ edit lsp.WorkspaceEdit }
type formatMsg struct {
	tab   *tab
	edits []lsp.TextEdit
	save  bool
}
type lspErrMsg struct{ err error }

func (a *App) lspRequest(capability string, f func(ctx context.Context, c *lsp.Client, t *tab) tea.Msg) tea.Cmd {
	t := a.cur()
	if t == nil || t.kind != tabText {
		return nil
	}
	if t.lang == nil {
		a.fail(fmt.Errorf("この言語の言語サーバーは設定されていません"))
		return nil
	}
	c := a.lsp.client(t, capability)
	if c == nil {
		a.fail(fmt.Errorf("言語サーバーが %s に対応していないか、起動中です", capability))
		return nil
	}
	a.syncLSP()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), lspTimeout)
		defer cancel()
		return f(ctx, c, t)
	}
}

func (a *App) hover() tea.Cmd {
	return a.lspRequest("hoverProvider", func(ctx context.Context, c *lsp.Client, t *tab) tea.Msg {
		text, err := c.Hover(ctx, t.buf.Path, lspPos(t.buf, t.buf.Cur))
		if err != nil {
			return lspErrMsg{err}
		}
		return hoverMsg{text}
	})
}

func (a *App) gotoDefinition() tea.Cmd {
	return a.lspRequest("definitionProvider", func(ctx context.Context, c *lsp.Client, t *tab) tea.Msg {
		pos := lspPos(t.buf, t.buf.Cur)
		locs, err := c.Definition(ctx, t.buf.Path, pos)
		if err != nil {
			return lspErrMsg{err}
		}
		// Already on the definition: list the usages instead, like VS Code.
		if onLocation(locs, t.buf.Path, pos) {
			refs, err := c.References(ctx, t.buf.Path, pos)
			if err != nil {
				return lspErrMsg{err}
			}
			return locationsMsg{"参照", withoutLocation(refs, t.buf.Path, pos)}
		}
		return locationsMsg{"定義", locs}
	})
}

// withoutLocation returns locs minus those containing pos in the file at path.
func withoutLocation(locs []lsp.Location, path string, pos lsp.Position) []lsp.Location {
	var out []lsp.Location
	for _, l := range locs {
		if !onLocation([]lsp.Location{l}, path, pos) {
			out = append(out, l)
		}
	}
	return out
}

// onLocation reports whether pos in the file at path lies within one of locs.
func onLocation(locs []lsp.Location, path string, pos lsp.Position) bool {
	atOrAfter := func(a, b lsp.Position) bool {
		return a.Line > b.Line || (a.Line == b.Line && a.Character >= b.Character)
	}
	for _, l := range locs {
		if lsp.Path(l.URI) == path && atOrAfter(pos, l.Range.Start) && atOrAfter(l.Range.End, pos) {
			return true
		}
	}
	return false
}

func (a *App) references() tea.Cmd {
	return a.lspRequest("referencesProvider", func(ctx context.Context, c *lsp.Client, t *tab) tea.Msg {
		locs, err := c.References(ctx, t.buf.Path, lspPos(t.buf, t.buf.Cur))
		if err != nil {
			return lspErrMsg{err}
		}
		return locationsMsg{"参照", locs}
	})
}

func (a *App) completion() tea.Cmd {
	t := a.cur()
	if t == nil || t.kind != tabText {
		return nil
	}
	start := wordStart(t.buf)
	version := t.buf.Version
	pos := t.buf.Cur
	return a.lspRequest("completionProvider", func(ctx context.Context, c *lsp.Client, t *tab) tea.Msg {
		items, err := c.Completion(ctx, t.buf.Path, lspPos(t.buf, pos))
		if err != nil {
			return lspErrMsg{err}
		}
		return completionMsg{tab: t, version: version, start: start, items: items}
	})
}

func (a *App) rename() tea.Cmd {
	t := a.cur()
	if t == nil || t.kind != tabText {
		return nil
	}
	pos := t.buf.Cur
	s := wordStart(t.buf)
	e := s
	for line := t.buf.Lines[s.Row]; e.Col < len(line) && isIdent(line[e.Col]); e.Col++ {
	}
	old := t.buf.TextRange(s, e)
	return a.pal.ask("新しい名前", old, func(v string) tea.Cmd {
		v = strings.TrimSpace(v)
		if v == "" || v == old {
			return nil
		}
		return a.lspRequest("renameProvider", func(ctx context.Context, c *lsp.Client, t *tab) tea.Msg {
			we, err := c.Rename(ctx, t.buf.Path, lspPos(t.buf, pos), v)
			if err != nil {
				return lspErrMsg{err}
			}
			return workspaceEditMsg{we}
		})
	})
}

func (a *App) format(save bool) tea.Cmd {
	t := a.cur()
	if t == nil || t.kind != tabText {
		return nil
	}
	unit := t.buf.IndentUnit()
	return a.lspRequest("documentFormattingProvider", func(ctx context.Context, c *lsp.Client, t *tab) tea.Msg {
		edits, err := c.Format(ctx, t.buf.Path, max(1, len(unit)), unit != "\t")
		if err != nil {
			return lspErrMsg{err}
		}
		return formatMsg{tab: t, edits: edits, save: save}
	})
}

func (a *App) applyWorkspaceEdit(we lsp.WorkspaceEdit) {
	files, n := 0, 0
	for uri, edits := range we {
		path := lsp.Path(uri)
		if !a.openFile(path) {
			continue
		}
		t := a.cur()
		t.buf.ApplyEdits(toEdits(t.buf, edits))
		files++
		n += len(edits)
	}
	a.info("%d 個のファイルで %d 箇所を変更しました（未保存）", files, n)
}

func (a *App) showLocations(m locationsMsg) tea.Cmd {
	if len(m.locs) == 0 {
		a.fail(fmt.Errorf("%sが見つかりません", m.title))
		return nil
	}
	jump := func(l lsp.Location) tea.Cmd {
		if !a.openFile(lsp.Path(l.URI)) {
			return nil
		}
		t := a.cur()
		t.buf.Anchor = nil
		p := bufPos(t.buf, l.Range.Start)
		t.buf.SetCursor(p.Row, buffer.VisualCol(t.buf.Lines[p.Row], p.Col), false)
		a.follow = true
		return a.setFocus(focusEditor)
	}
	if len(m.locs) == 1 {
		return jump(m.locs[0])
	}
	items := make([]paletteItem, len(m.locs))
	for i, l := range m.locs {
		p := lsp.Path(l.URI)
		items[i] = paletteItem{label: fmt.Sprintf("%s:%d", a.rel(p), l.Range.Start.Line+1), detail: strings.TrimSpace(lineAt(a, p, l.Range.Start.Line))}
	}
	return a.pal.pick(m.title, items, func(i int) tea.Cmd { return jump(m.locs[i]) })
}

// lineAt returns line n of a file, from an open buffer when possible.
func lineAt(a *App, path string, n int) string {
	for _, t := range a.tabs {
		if t.kind == tabText && t.buf.Path == path && n < len(t.buf.Lines) {
			return string(t.buf.Lines[n])
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	if n < len(lines) {
		return lines[n]
	}
	return ""
}

func (a *App) diagnosticPicker() tea.Cmd {
	type entry struct {
		path string
		d    lsp.Diagnostic
	}
	var all []entry
	paths := make([]string, 0, len(a.lsp.diags))
	for p := range a.lsp.diags {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		for _, d := range a.lsp.diagnostics(p) {
			all = append(all, entry{p, d})
		}
	}
	if len(all) == 0 {
		a.info("診断はありません")
		return nil
	}
	items := make([]paletteItem, len(all))
	sevName := map[int]string{1: "エラー", 2: "警告", 3: "情報", 4: "ヒント"}
	for i, e := range all {
		items[i] = paletteItem{label: fmt.Sprintf("%s:%d %s", a.rel(e.path), e.d.Range.Start.Line+1, firstLine(e.d.Message)), detail: sevName[sevRank(e.d.Severity)]}
	}
	return a.pal.pick("診断", items, func(i int) tea.Cmd {
		return a.showLocations(locationsMsg{"診断", []lsp.Location{{URI: lsp.URI(all[i].path), Range: all[i].d.Range}}})
	})
}

// gotoDiagnostic jumps to the next (dir=1) or previous (dir=-1) diagnostic.
func (a *App) gotoDiagnostic(dir int) {
	t := a.cur()
	if t == nil || t.kind != tabText {
		return
	}
	ds := a.lsp.diagnostics(t.buf.Path)
	if len(ds) == 0 {
		a.info("このファイルに診断はありません")
		return
	}
	cur := t.buf.Cur
	var target *lsp.Diagnostic
	if dir > 0 {
		for i := range ds {
			p := bufPos(t.buf, ds[i].Range.Start)
			if cur.Less(p) {
				target = &ds[i]
				break
			}
		}
		if target == nil {
			target = &ds[0]
		}
	} else {
		for i := len(ds) - 1; i >= 0; i-- {
			p := bufPos(t.buf, ds[i].Range.Start)
			if p.Less(cur) {
				target = &ds[i]
				break
			}
		}
		if target == nil {
			target = &ds[len(ds)-1]
		}
	}
	p := bufPos(t.buf, target.Range.Start)
	t.buf.Anchor = nil
	t.buf.SetCursor(p.Row, buffer.VisualCol(t.buf.Lines[p.Row], p.Col), false)
	a.follow = true
	a.info("%s", firstLine(target.Message))
}

// ---- completion popup ----

type completionPopup struct {
	tab   *tab
	start buffer.Pos
	all   []lsp.CompletionItem
	items []lsp.CompletionItem
	sel   int
}

func (c *completionPopup) filter() {
	b := c.tab.buf
	prefix := ""
	if c.start.Row == b.Cur.Row && c.start.Col <= b.Cur.Col {
		prefix = strings.ToLower(string(b.Lines[b.Cur.Row][c.start.Col:b.Cur.Col]))
	}
	c.items = c.items[:0]
	for _, it := range c.all {
		key := it.FilterText
		if key == "" {
			key = it.Label
		}
		if strings.HasPrefix(strings.ToLower(key), prefix) || (prefix != "" && strings.Contains(strings.ToLower(key), prefix)) {
			c.items = append(c.items, it)
		}
	}
	isPrefix := func(it lsp.CompletionItem) bool {
		key := it.FilterText
		if key == "" {
			key = it.Label
		}
		return strings.HasPrefix(strings.ToLower(key), prefix)
	}
	sort.SliceStable(c.items, func(i, j int) bool {
		if pi, pj := isPrefix(c.items[i]), isPrefix(c.items[j]); pi != pj {
			return pi
		}
		ki, kj := c.items[i].SortText, c.items[j].SortText
		if ki == "" {
			ki = c.items[i].Label
		}
		if kj == "" {
			kj = c.items[j].Label
		}
		return ki < kj
	})
	c.sel = clamp(c.sel, 0, max(0, len(c.items)-1))
}

func (a *App) acceptCompletion() {
	c := a.comp
	a.comp = nil
	if c == nil || len(c.items) == 0 {
		return
	}
	it := c.items[c.sel]
	b := c.tab.buf
	start := c.start
	text := it.InsertText
	if text == "" {
		text = it.Label
	}
	if it.TextEdit != nil {
		start = bufPos(b, it.TextEdit.Range.Start)
		text = it.TextEdit.NewText
	}
	if it.InsertTextFormat == 2 {
		text = stripSnippet(text)
	}
	if b.Cur.Less(start) {
		start = b.Cur
	}
	b.SelectRange(start, b.Cur)
	b.InsertBlock(text)
	a.follow = true
}

// stripSnippet removes LSP snippet placeholders: ${1:foo} -> foo, $0 -> "".
func stripSnippet(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			sb.WriteByte(s[i+1])
			i++
			continue
		}
		if s[i] == '$' && i+1 < len(s) {
			if s[i+1] == '{' {
				j := i + 2
				for j < len(s) && s[j] >= '0' && s[j] <= '9' {
					j++
				}
				if j < len(s) && s[j] == ':' {
					j++
				}
				depth := 1
				k := j
				for k < len(s) && depth > 0 {
					if s[k] == '{' {
						depth++
					} else if s[k] == '}' {
						depth--
					}
					k++
				}
				sb.WriteString(stripSnippet(s[j : k-1]))
				i = k - 1
				continue
			}
			if s[i+1] >= '0' && s[i+1] <= '9' {
				i++
				continue
			}
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
}

func wordStart(b *buffer.Buffer) buffer.Pos {
	p := b.Cur
	line := b.Lines[p.Row]
	for p.Col > 0 && isIdent(line[p.Col-1]) {
		p.Col--
	}
	return p
}

func isIdent(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

var completionKinds = map[int]string{
	1: "text", 2: "method", 3: "func", 4: "ctor", 5: "field", 6: "var", 7: "class", 8: "iface",
	9: "module", 10: "prop", 12: "value", 13: "enum", 14: "keyword", 15: "snippet", 21: "const", 22: "struct", 25: "type",
}

func (c *completionPopup) render(maxH int) []string {
	n := min(len(c.items), maxH, 10)
	start := 0
	if c.sel >= n {
		start = c.sel - n + 1
	}
	w := 20
	for _, it := range c.items[start : start+n] {
		w = max(w, ansi.StringWidth(it.Label)+10)
	}
	w = min(w, 60)
	out := make([]string, 0, n)
	for i := start; i < start+n; i++ {
		it := c.items[i]
		st, dim := stPalette, stPaletteDim
		if i == c.sel {
			st, dim = stPaletteSel, bgfg(cListSel, curTheme.SubSel)
		}
		kind := completionKinds[it.Kind]
		label := fit(" "+it.Label, w-9)
		out = append(out, st.Render(label)+dim.Render(fit(" "+kind, 9)))
	}
	return out
}

func (a *App) lspStatus() string {
	t := a.cur()
	if t == nil || t.kind != tabText || t.lang == nil {
		return ""
	}
	var names []string
	for _, n := range t.lang.Servers {
		root := t.lang.FindRoot(t.buf.Path, a.root)
		if c, ok := a.lsp.clients[n+"\x00"+root]; ok {
			select {
			case <-c.Ready():
				names = append(names, n)
			default:
				names = append(names, n+"…")
			}
		}
	}
	return strings.Join(names, ",")
}

func relTo(root, p string) string {
	if r, err := filepath.Rel(root, p); err == nil && !strings.HasPrefix(r, "..") {
		return r
	}
	return p
}
