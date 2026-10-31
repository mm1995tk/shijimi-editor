package ui

import (
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

const (
	maxPaletteItems = 12
	maxIndexedFiles = 50000
)

var skipDirs = map[string]bool{".git": true, "node_modules": true, ".direnv": true, "target": true, "dist": true, ".next": true}

type paletteItem struct {
	label   string
	detail  string
	matched []int // byte offsets into label
	index   int
}

// palette is the overlay used for Quick Open (Ctrl+P), the command palette
// (">" prefix), go-to-line (":" prefix) and free-text prompts.
type palette struct {
	open  bool
	input lineInput
	sel   int
	items []paletteItem
	files []string

	// prompt mode
	prompt   string
	onSubmit func(value string) tea.Cmd

	// pick mode
	picking   bool
	pickTitle string
	pickAll   []paletteItem
	onPick    func(index int) tea.Cmd

	// live is set when the list comes from a server query (see pickLive);
	// liveSeq numbers the edits so that late answers can be told apart.
	live    bool
	liveSeq int
}

func newPalette() *palette {
	return &palette{input: newLineInput(4096)}
}

func indexFiles(root string) []string {
	var files []string
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != root && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if rel, err := filepath.Rel(root, p); err == nil {
			files = append(files, rel)
		}
		if len(files) >= maxIndexedFiles {
			return filepath.SkipAll
		}
		return nil
	})
	return files
}

// show opens the palette in quick-open/command mode with initial text.
func (p *palette) show(root, initial string) tea.Cmd {
	p.open = true
	p.picking = false
	p.live = false
	p.prompt = ""
	p.onSubmit = nil
	p.files = indexFiles(root)
	p.input.Placeholder = i18n.T("palette.placeholder")
	p.input.SetValue(initial)
	p.input.CursorEnd()
	p.sel = 0
	p.input.Focus()
	return nil
}

// ask opens the palette as a single-line prompt.
func (p *palette) ask(prompt, initial string, onSubmit func(string) tea.Cmd) tea.Cmd {
	p.open = true
	p.picking = false
	p.live = false
	p.prompt = prompt
	p.onSubmit = onSubmit
	p.items = nil
	p.input.Placeholder = ""
	p.input.SetValue(initial)
	p.input.CursorEnd()
	p.sel = 0
	p.input.Focus()
	return nil
}

func (p *palette) close() {
	p.open = false
	p.input.Blur()
}

func (p *palette) filter(cmds []command) {
	v := p.input.Value()
	p.items = p.items[:0]
	switch {
	case p.picking && p.live:
		p.items = append(p.items, p.pickAll...)
	case p.picking:
		q := strings.TrimSpace(v)
		if q == "" {
			p.items = append(p.items, p.pickAll...)
			break
		}
		labels := make([]string, len(p.pickAll))
		for i, it := range p.pickAll {
			labels[i] = it.label
		}
		for _, m := range fuzzy.Find(q, labels) {
			it := p.pickAll[m.Index]
			it.matched = m.MatchedIndexes
			p.items = append(p.items, it)
		}
	case p.prompt != "":
		return
	case strings.HasPrefix(v, ">"):
		q := strings.TrimSpace(v[1:])
		names := make([]string, len(cmds))
		for i, c := range cmds {
			names[i] = c.name
		}
		if q == "" {
			for i, c := range cmds {
				p.items = append(p.items, paletteItem{label: c.name, detail: c.key, index: i})
			}
		} else {
			for _, m := range fuzzy.Find(q, names) {
				p.items = append(p.items, paletteItem{label: m.Str, detail: cmds[m.Index].key, matched: m.MatchedIndexes, index: m.Index})
			}
		}
	case strings.HasPrefix(v, ":"):
		n, err := strconv.Atoi(strings.TrimSpace(v[1:]))
		label := i18n.T("palette.needline")
		if err == nil {
			label = strconv.Itoa(n) + i18n.T("palette.gotoline")
		}
		p.items = append(p.items, paletteItem{label: label, index: -1})
	default:
		q := strings.TrimSpace(v)
		if q == "" {
			for i, f := range p.files {
				if i >= 200 {
					break
				}
				p.items = append(p.items, paletteItem{label: filepath.Base(f), detail: filepath.Dir(f), index: i})
			}
		} else {
			for i, m := range fuzzy.Find(q, p.files) {
				if i >= 200 {
					break
				}
				p.items = append(p.items, paletteItem{label: m.Str, matched: m.MatchedIndexes, index: m.Index})
			}
		}
	}
	p.sel = clamp(p.sel, 0, max(0, len(p.items)-1))
}

// head is the palette's heading and the mark after it: a prompt asks with
// ":", the rest search or pick with ">".
func (p *palette) head() (title, prefix string) {
	switch {
	case p.prompt != "":
		return p.prompt, ":"
	case p.picking:
		return p.pickTitle, ">"
	}
	return "", ">"
}

// paletteLabel is what precedes the input on the palette's first row.
func paletteLabel(title, prefix string) string {
	switch {
	case prefix == ":":
		return " " + title + ": "
	case title != "":
		return " " + title + " > "
	}
	return " > "
}

// label is what precedes the input on the palette's first row.
func (p *palette) label() string { return paletteLabel(p.head()) }

// overlay is the palette as a screen.Overlay: the input field and the rows in
// view (maxPaletteItems at most, scrolled so that the selected one shows) with
// Active the selected one among them. It has no place in a pane, so it has no
// At (the TUI centres it). It changes nothing. Drawing it as cells is
// drawPalette's work.
func (p *palette) overlay() screen.Overlay {
	title, prefix := p.head()
	ov := screen.Overlay{
		Kind:  screen.OverlayPalette,
		Title: title,
		Field: &screen.Field{
			Prefix:      prefix,
			Text:        p.input.Value(),
			Cursor:      p.input.Position(),
			Placeholder: p.input.Placeholder,
		},
	}
	start := 0
	if p.sel >= maxPaletteItems {
		start = p.sel - maxPaletteItems + 1
	}
	ov.Active = p.sel - start
	for i := start; i < len(p.items) && i < start+maxPaletteItems; i++ {
		it := p.items[i]
		item := screen.Item{Label: it.label, Detail: it.detail, Matched: runeIndexes(it.label, it.matched)}
		ov.Items = append(ov.Items, item)
	}
	if p.prompt == "" && len(p.items) == 0 {
		ov.Hint = i18n.T("palette.nomatch")
	}
	return ov
}

// drawPalette draws a palette Overlay as a box w cells wide. in is the TUI's
// state of the input (what is in view of a long text); the Overlay holds the
// text and the caret.
func drawPalette(ov screen.Overlay, in *lineInput, w int) []string {
	var out []string
	label := paletteLabel(ov.Title, ov.Field.Prefix)
	out = append(out, stPaletteBox.Render(fit("", w)))
	out = append(out, stPaletteBox.Render(label)+fitStyled(in.view(inputStyleOn(cPaletteBg)), w-ansi.StringWidth(label), stPalette))
	for i, it := range ov.Items {
		out = append(out, renderItem(it, w, i == ov.Active))
	}
	if ov.Hint != "" {
		out = append(out, stPaletteDim.Render(fit(ov.Hint, w)))
	}
	out = append(out, stPalette.Render(fit("", w)))
	return out
}

func renderItem(it screen.Item, w int, selected bool) string {
	base, hi := stPalette, stPaletteHi
	if selected {
		base, hi = stPaletteSel, stPaletteHiS
	}
	matched := map[int]bool{}
	for _, m := range it.Matched {
		matched[m] = true
	}
	var sb strings.Builder
	sb.WriteString(base.Render("  "))
	used := 2
	for i, r := range []rune(it.Label) {
		rw := ansi.StringWidth(string(r))
		if used+rw > w-1 {
			break
		}
		if matched[i] {
			sb.WriteString(hi.Render(string(r)))
		} else {
			sb.WriteString(base.Render(string(r)))
		}
		used += rw
	}
	if it.Detail != "" && it.Detail != "." {
		d := "  " + it.Detail
		dw := ansi.StringWidth(d)
		if used+dw < w {
			st := stPaletteDim
			if selected {
				st = bgfg(cListSel, curTheme.SubSel)
			}
			sb.WriteString(st.Render(d))
			used += dw
		}
	}
	if used < w {
		sb.WriteString(base.Render(strings.Repeat(" ", w-used)))
	}
	return sb.String()
}

// pick opens the palette as a fuzzy list; onPick receives the index into items.
func (p *palette) pick(title string, items []paletteItem, onPick func(int) tea.Cmd) tea.Cmd {
	p.open = true
	p.picking = true
	p.prompt = ""
	p.pickTitle = title
	p.live = false
	for i := range items {
		items[i].index = i
	}
	p.pickAll = items
	p.onPick = onPick
	p.input.Placeholder = i18n.T("palette.filter")
	p.input.SetValue("")
	p.sel = 0
	p.input.Focus()
	return nil
}

// pickLive is pick for a list the language server filters: the list is not
// fuzzy-filtered here. The caller asks the server after each edit of the
// input (queryLater) and puts the answer in with setPickItems.
func (p *palette) pickLive(title string, onPick func(int) tea.Cmd) tea.Cmd {
	cmd := p.pick(title, nil, onPick)
	p.live = true
	p.liveSeq++
	p.input.Placeholder = i18n.T("palette.typename")
	return cmd
}

// setPickItems replaces the list of a pick palette and selects the first row.
func (p *palette) setPickItems(items []paletteItem) {
	for i := range items {
		items[i].index = i
	}
	p.pickAll = items
	p.sel = 0
}

// runeIndexes turns byte offsets into s (small to large, each at the start of
// a rune) into the positions of those runes.
func runeIndexes(s string, byteOffsets []int) []int {
	var out []int
	n, j := 0, 0
	for b := range s {
		if j >= len(byteOffsets) {
			break
		}
		if b == byteOffsets[j] {
			out = append(out, n)
			j++
		}
		n++
	}
	return out
}

// paletteWidth is the width of the palette window.
func (a *App) paletteWidth() int { return clamp(a.w*3/5, 30, 100) }
