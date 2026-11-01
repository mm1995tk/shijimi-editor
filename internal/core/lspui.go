package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/config"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/lsp"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

const LspTimeout = 10 * time.Second

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

// clientsFor returns running clients for a tab, starting them if needed.
// Errors (e.g. server binary missing) are returned once per server.
func (m *lspManager) clientsFor(t *Tab) ([]*lsp.Client, []error) {
	if t.Kind != TabText || t.lang == nil || t.Buf.Path == "" {
		return nil, nil
	}
	var out []*lsp.Client
	var errs []error
	root := t.lang.FindRoot(t.Buf.Path, m.root)
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
			errs = append(errs, fmt.Errorf(i18n.T("lspui.noconfig"), name))
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
func (m *lspManager) client(t *Tab, capability string) *lsp.Client {
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
	row := Clamp(p.Line, 0, len(b.Lines)-1)
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
func (a *Core) attachLSP(t *Tab) {
	if a.single || t.Kind != TabText || t.Buf.Path == "" {
		return
	}
	t.lang = a.cfg.LanguageFor(t.Buf.Path)
	if t.lang == nil {
		return
	}
	if t.lang.IndentUnit != "" {
		t.Buf.SetIndentUnit(t.lang.IndentUnit)
	}
	cs, errs := a.lsp.clientsFor(t)
	for _, err := range errs {
		a.Fail(err)
	}
	for _, c := range cs {
		c.DidOpen(t.Buf.Path, t.lang.ID(), t.Buf.Text(), t.Buf.Version+1)
	}
	t.lspVersion = t.Buf.Version
}

func (a *Core) detachLSP(t *Tab) {
	cs, _ := a.lsp.clientsFor(t)
	for _, c := range cs {
		c.DidClose(t.Buf.Path)
	}
}

// syncLSP sends didChange for buffers edited since the last sync.
func (a *Core) SyncLSP() {
	for _, t := range a.tabs {
		if t.Kind != TabText || t.lang == nil || t.Buf.Version == t.lspVersion {
			continue
		}
		cs, _ := a.lsp.clientsFor(t)
		text := t.Buf.Text()
		for _, c := range cs {
			c.DidChange(t.Buf.Path, text, t.Buf.Version+1)
		}
		t.lspVersion = t.Buf.Version
	}
}

type hoverMsg struct {
	text  string
	mouse *mouseHover // set when the pointer, not the cursor, asked for it
}
type locationsMsg struct {
	title string
	locs  []lsp.Location
}
type completionMsg struct {
	tab     *Tab
	version int
	start   buffer.Pos
	items   []lsp.CompletionItem
}
type workspaceEditMsg struct{ edit lsp.WorkspaceEdit }
type formatMsg struct {
	tab   *Tab
	edits []lsp.TextEdit
	save  bool
}
type lspErrMsg struct{ err error }

// lspReq builds the []Effect that carries out a RunLSP.
func (a *Core) lspReq(run func(ctx context.Context) Event) []Effect {
	return []Effect{RunLSP{Run: run}}
}

func (a *Core) lspRequest(capability string, f func(ctx context.Context, c *lsp.Client, t *Tab) Event) []Effect {
	t := a.Cur()
	if t == nil || t.Kind != TabText {
		return nil
	}
	if t.lang == nil {
		a.Fail(errors.New(i18n.T("lspui.noserver")))
		return nil
	}
	c := a.lsp.client(t, capability)
	if c == nil {
		a.Fail(fmt.Errorf(i18n.T("lspui.unsupported"), capability))
		return nil
	}
	a.SyncLSP()
	return a.lspReq(func(ctx context.Context) Event { return f(ctx, c, t) })
}

func (a *Core) hover() []Effect {
	return a.lspRequest("hoverProvider", func(ctx context.Context, c *lsp.Client, t *Tab) Event {
		text, err := c.Hover(ctx, t.Buf.Path, lspPos(t.Buf, t.Buf.Cur))
		if err != nil {
			return lspErrMsg{err}
		}
		return hoverMsg{text: text}
	})
}

func (a *Core) gotoDefinition() []Effect {
	return a.lspRequest("definitionProvider", func(ctx context.Context, c *lsp.Client, t *Tab) Event {
		pos := lspPos(t.Buf, t.Buf.Cur)
		locs, err := c.Definition(ctx, t.Buf.Path, pos)
		if err != nil {
			return lspErrMsg{err}
		}
		// Already on the definition: list the usages instead, like VS Code.
		if onLocation(locs, t.Buf.Path, pos) {
			refs, err := c.References(ctx, t.Buf.Path, pos)
			if err != nil {
				return lspErrMsg{err}
			}
			return locationsMsg{i18n.T("lspui.references"), withoutLocation(refs, t.Buf.Path, pos)}
		}
		return locationsMsg{i18n.T("lspui.definition"), locs}
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

func (a *Core) references() []Effect {
	return a.lspRequest("referencesProvider", func(ctx context.Context, c *lsp.Client, t *Tab) Event {
		locs, err := c.References(ctx, t.Buf.Path, lspPos(t.Buf, t.Buf.Cur))
		if err != nil {
			return lspErrMsg{err}
		}
		return locationsMsg{i18n.T("lspui.references"), locs}
	})
}

func (a *Core) completion() []Effect {
	t := a.Cur()
	if t == nil || t.Kind != TabText {
		return nil
	}
	start := wordStart(t.Buf)
	version := t.Buf.Version
	pos := t.Buf.Cur
	return a.lspRequest("completionProvider", func(ctx context.Context, c *lsp.Client, t *Tab) Event {
		items, err := c.Completion(ctx, t.Buf.Path, lspPos(t.Buf, pos))
		if err != nil {
			return lspErrMsg{err}
		}
		return completionMsg{tab: t, version: version, start: start, items: items}
	})
}

func (a *Core) rename() []Effect {
	t := a.Cur()
	if t == nil || t.Kind != TabText {
		return nil
	}
	pos := t.Buf.Cur
	s := wordStart(t.Buf)
	e := s
	for line := t.Buf.Lines[s.Row]; e.Col < len(line) && isIdent(line[e.Col]); e.Col++ {
	}
	old := t.Buf.TextRange(s, e)
	return a.pal.ask(i18n.T("app.newname"), old, func(v string) []Effect {
		v = strings.TrimSpace(v)
		if v == "" || v == old {
			return nil
		}
		return a.lspRequest("renameProvider", func(ctx context.Context, c *lsp.Client, t *Tab) Event {
			we, err := c.Rename(ctx, t.Buf.Path, lspPos(t.Buf, pos), v)
			if err != nil {
				return lspErrMsg{err}
			}
			return workspaceEditMsg{we}
		})
	})
}

func (a *Core) format(save bool) []Effect {
	t := a.Cur()
	if t == nil || t.Kind != TabText {
		return nil
	}
	if t.lang != nil && t.lang.Formatter != nil {
		return a.runFormatter(t, t.lang.Formatter, save)
	}
	unit := t.Buf.IndentUnit()
	return a.lspRequest("documentFormattingProvider", func(ctx context.Context, c *lsp.Client, t *Tab) Event {
		edits, err := c.Format(ctx, t.Buf.Path, max(1, len(unit)), unit != "\t")
		if err != nil {
			return lspErrMsg{err}
		}
		return formatMsg{tab: t, edits: edits, save: save}
	})
}

type codeActionsMsg struct {
	client  *lsp.Client
	actions []lsp.CodeAction
}
type codeActionResolvedMsg struct {
	client *lsp.Client
	action lsp.CodeAction
}

// codeAction lists the quick fixes and refactorings for the selection, or
// for the cursor position when nothing is selected.
func (a *Core) codeAction() []Effect {
	t := a.Cur()
	if t == nil || t.Kind != TabText {
		return nil
	}
	rng, diags := codeActionRange(t.Buf, a.lsp.diagnostics(t.Buf.Path))
	return a.lspRequest("codeActionProvider", func(ctx context.Context, c *lsp.Client, t *Tab) Event {
		actions, err := c.CodeActions(ctx, t.Buf.Path, rng, diags)
		if err != nil {
			return lspErrMsg{err}
		}
		return codeActionsMsg{c, actions}
	})
}

// codeActionRange returns the range to ask code actions for and the
// diagnostics overlapping it. With no selection and the cursor outside every
// diagnostic, a diagnostic on the cursor's line is used, so its fix is
// offered without first moving onto the underlined span.
func codeActionRange(b *buffer.Buffer, all []lsp.Diagnostic) (lsp.Range, []lsp.Diagnostic) {
	s, e, sel := b.Selection()
	if !sel {
		s, e = b.Cur, b.Cur
	}
	rng := lsp.Range{Start: lspPos(b, s), End: lspPos(b, e)}
	le := func(p, q lsp.Position) bool {
		return p.Line < q.Line || p.Line == q.Line && p.Character <= q.Character
	}
	var diags []lsp.Diagnostic
	for _, d := range all {
		if le(d.Range.Start, rng.End) && le(rng.Start, d.Range.End) {
			diags = append(diags, d)
		}
	}
	if len(diags) > 0 || sel {
		return rng, diags
	}
	for _, d := range all {
		if d.Range.Start.Line <= s.Row && s.Row <= d.Range.End.Line {
			diags = append(diags, d)
		}
	}
	if len(diags) > 0 {
		rng = diags[0].Range
	}
	return rng, diags
}

func (a *Core) showCodeActions(m codeActionsMsg) []Effect {
	if len(m.actions) == 0 {
		a.InfoText(i18n.T("lspui.noactions"))
		return nil
	}
	actions := slices.Clone(m.actions)
	// Preferred fixes first, then the other quick fixes, then refactorings.
	rank := func(act lsp.CodeAction) int {
		switch {
		case act.IsPreferred:
			return 0
		case act.Kind == "quickfix" || strings.HasPrefix(act.Kind, "quickfix."):
			return 1
		}
		return 2
	}
	sort.SliceStable(actions, func(i, j int) bool { return rank(actions[i]) < rank(actions[j]) })
	items := make([]paletteItem, len(actions))
	for i, act := range actions {
		items[i] = paletteItem{label: act.Title, detail: act.Kind}
	}
	return a.pal.pick(i18n.T("lspui.codeactions"), items, func(i int) []Effect {
		act := actions[i]
		return a.lspReq(func(ctx context.Context) Event {
			r, err := m.client.ResolveCodeAction(ctx, act)
			if err != nil {
				return lspErrMsg{err}
			}
			return codeActionResolvedMsg{m.client, r}
		})
	})
}

// runCodeAction applies the action's edit, then runs its command; the
// command's own edits come back as workspace/applyEdit events.
func (a *Core) runCodeAction(m codeActionResolvedMsg) []Effect {
	if len(m.action.Edit) > 0 {
		a.applyWorkspaceEdit(m.action.Edit)
	}
	cmd := m.action.Command
	if cmd == nil {
		if len(m.action.Edit) == 0 {
			a.InfoText(i18n.T("lspui.noedit"))
		}
		return nil
	}
	a.SyncLSP()
	return a.lspReq(func(ctx context.Context) Event {
		if err := m.client.ExecuteCommand(ctx, *cmd); err != nil {
			return lspErrMsg{err}
		}
		return nil
	})
}

func (a *Core) applyWorkspaceEdit(we lsp.WorkspaceEdit) {
	files, n := 0, 0
	for uri, edits := range we {
		path := lsp.Path(uri)
		if !a.openFile(path) {
			continue
		}
		t := a.Cur()
		t.Buf.ApplyEdits(toEdits(t.Buf, edits))
		files++
		n += len(edits)
	}
	a.Info(i18n.T("lspui.applied"), files, n)
}

func (a *Core) showLocations(m locationsMsg) []Effect {
	if len(m.locs) == 0 {
		a.Fail(fmt.Errorf(i18n.T("lspui.notfound"), m.title))
		return nil
	}
	jump := a.jumpTo
	if len(m.locs) == 1 {
		return jump(m.locs[0])
	}
	items := make([]paletteItem, len(m.locs))
	for i, l := range m.locs {
		p := lsp.Path(l.URI)
		items[i] = paletteItem{label: fmt.Sprintf("%s:%d", a.Rel(p), l.Range.Start.Line+1), detail: strings.TrimSpace(lineAt(a, p, l.Range.Start.Line))}
	}
	return a.pal.pick(m.title, items, func(i int) []Effect { return jump(m.locs[i]) })
}

// lineAt returns line n of a file, from an open buffer when possible.
func lineAt(a *Core, path string, n int) string {
	for _, t := range a.tabs {
		if t.Kind == TabText && t.Buf.Path == path && n < len(t.Buf.Lines) {
			return string(t.Buf.Lines[n])
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

func (a *Core) diagnosticPicker() []Effect {
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
		a.InfoText(i18n.T("lspui.nodiag"))
		return nil
	}
	items := make([]paletteItem, len(all))
	sevName := map[int]string{1: i18n.T("diag.error"), 2: i18n.T("diag.warning"), 3: i18n.T("diag.info"), 4: i18n.T("diag.hint")}
	for i, e := range all {
		items[i] = paletteItem{label: fmt.Sprintf("%s:%d %s", a.Rel(e.path), e.d.Range.Start.Line+1, FirstLine(e.d.Message)), detail: sevName[sevRank(e.d.Severity)]}
	}
	return a.pal.pick(i18n.T("ctx.diagnostics"), items, func(i int) []Effect {
		return a.showLocations(locationsMsg{i18n.T("ctx.diagnostics"), []lsp.Location{{URI: lsp.URI(all[i].path), Range: all[i].d.Range}}})
	})
}

// gotoDiagnostic jumps to the next (dir=1) or previous (dir=-1) diagnostic.
func (a *Core) gotoDiagnostic(dir int) {
	t := a.Cur()
	if t == nil || t.Kind != TabText {
		return
	}
	ds := a.lsp.diagnostics(t.Buf.Path)
	if len(ds) == 0 {
		a.InfoText(i18n.T("lspui.nodiagfile"))
		return
	}
	cur := t.Buf.Cur
	var target *lsp.Diagnostic
	if dir > 0 {
		for i := range ds {
			p := bufPos(t.Buf, ds[i].Range.Start)
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
			p := bufPos(t.Buf, ds[i].Range.Start)
			if p.Less(cur) {
				target = &ds[i]
				break
			}
		}
		if target == nil {
			target = &ds[len(ds)-1]
		}
	}
	p := bufPos(t.Buf, target.Range.Start)
	t.Buf.Anchor = nil
	t.Buf.SetCursor(p.Row, buffer.VisualCol(t.Buf.Lines[p.Row], p.Col), false)
	a.follow = true
	a.Info("%s", FirstLine(target.Message))
}

// ---- completion popup ----

type completionPopup struct {
	tab   *Tab
	start buffer.Pos
	all   []lsp.CompletionItem
	items []lsp.CompletionItem
	sel   int
}

func (c *completionPopup) filter() {
	b := c.tab.Buf
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
	c.sel = Clamp(c.sel, 0, max(0, len(c.items)-1))
}

func (a *Core) acceptCompletion() {
	c := a.comp
	a.comp = nil
	if c == nil || len(c.items) == 0 {
		return
	}
	it := c.items[c.sel]
	b := c.tab.Buf
	start := c.start
	text := it.InsertText
	if text == "" {
		text = it.Label
	}
	if it.TextEdit != nil {
		start = bufPos(b, it.TextEdit.Range.Start)
		text = it.TextEdit.NewText
	}
	if b.Cur.Less(start) {
		start = b.Cur
	}
	a.follow = true
	if it.InsertTextFormat == 2 {
		a.startSnippet(c.tab, start, text)
		return
	}
	if r := a.recIns; r != nil && !a.replaying && start.Row == b.Cur.Row {
		r.steps = append(r.steps, insertStep{completion: true, text: text, back: b.Cur.Col - start.Col})
	}
	b.SelectRange(start, b.Cur)
	b.InsertBlock(text)
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

// overlay is the completion popup as a screen.Overlay: the candidates in view
// (maxH of them at most, ten at most, scrolled so that the selected one shows)
// with Active the selected one among them, anchored at the cursor. It changes
// nothing. Drawing it as cells is drawCompletion's work.
func (c *completionPopup) overlay(maxH int) screen.Overlay {
	n := min(len(c.items), maxH, 10)
	start := 0
	if c.sel >= n {
		start = c.sel - n + 1
	}
	at := ToScreenPos(c.tab.Buf.Cur)
	ov := screen.Overlay{
		Kind:   screen.OverlayCompletion,
		Pane:   c.tab.Path(),
		At:     &at,
		Active: c.sel - start,
	}
	for _, it := range c.items[start : start+n] {
		ov.Items = append(ov.Items, screen.Item{Label: it.Label, Detail: it.Detail, Kind: completionKinds[it.Kind]})
	}
	return ov
}

func (a *Core) lspStatus() string {
	t := a.Cur()
	if t == nil || t.Kind != TabText || t.lang == nil {
		return ""
	}
	var names []string
	for _, n := range t.lang.Servers {
		root := t.lang.FindRoot(t.Buf.Path, a.root)
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

// jumpTo opens the file of l and puts the cursor at the start of its range.
func (a *Core) jumpTo(l lsp.Location) []Effect {
	a.pushJump()
	if !a.openFile(lsp.Path(l.URI)) {
		return nil
	}
	t := a.Cur()
	t.Buf.Anchor = nil
	p := bufPos(t.Buf, l.Range.Start)
	t.Buf.SetCursor(p.Row, buffer.VisualCol(t.Buf.Lines[p.Row], p.Col), false)
	a.follow = true
	return a.SetFocus(FocusEditor)
}

// handleLSPReply takes the Events a language server's reply becomes. ok is
// false for an Event that is not one of them.
func (a *Core) handleLSPReply(ev Event) (effs []Effect, ok bool) {
	switch msg := ev.(type) {
	case hoverMsg:
		a.showHover(msg)
	case locationsMsg:
		return a.showLocations(msg), true
	case completionMsg:
		t := a.CurText()
		if t == nil || t != msg.tab || a.mode != ModeInsert && a.helix || len(msg.items) == 0 {
			return nil, true
		}
		a.comp = &completionPopup{tab: t, start: msg.start, all: msg.items}
		a.comp.filter()
		if len(a.comp.items) == 0 {
			a.comp = nil
		}
	case signatureMsg:
		a.showSignature(msg)
	case workspaceEditMsg:
		a.applyWorkspaceEdit(msg.edit)
	case symbolsMsg:
		return a.showSymbols(msg), true
	case symbolQueryMsg:
		return a.runSymbolQuery(msg), true
	case workspaceSymbolsMsg:
		a.showWorkspaceSymbols(msg)
	case codeActionsMsg:
		return a.showCodeActions(msg), true
	case codeActionResolvedMsg:
		return a.runCodeAction(msg), true
	case formatMsg:
		if len(msg.edits) > 0 {
			msg.tab.Buf.ApplyEdits(toEdits(msg.tab.Buf, msg.edits))
		}
		if msg.save {
			return a.saveTab(msg.tab), true
		}
		a.InfoText(i18n.T("app.formatted"))
	case lspErrMsg:
		a.Fail(msg.err)
	case semanticMsg:
		msg.h.sem.apply(msg)
	default:
		return nil, false
	}
	return nil, true
}

func (a *Core) showHover(msg hoverMsg) {
	if t := a.CurText(); a.FullPreview(t) {
		return // the popup would be anchored to the hidden buffer
	}
	if m := msg.mouse; m != nil {
		if m.seq != a.hoverSeq || m.tab != a.Cur() || strings.TrimSpace(msg.text) == "" {
			return
		}
		a.hoverText, a.hoverScroll = msg.text, 0
		a.hoverAt, a.hoverWord = &m.pos, m.word
		return
	}
	if strings.TrimSpace(msg.text) == "" {
		a.InfoText(i18n.T("app.nohover"))
	} else {
		a.hoverText = msg.text
		a.hoverScroll = 0
		a.hoverAt = nil
	}
}
