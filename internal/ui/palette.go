package ui

import (
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
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
	input textinput.Model
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
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 4096
	return &palette{input: ti}
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
	p.input.Placeholder = "ファイル名で検索  ( > コマンド   : 行番号 )"
	p.input.SetValue(initial)
	p.input.CursorEnd()
	p.sel = 0
	return p.input.Focus()
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
	return p.input.Focus()
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
		label := "行番号を入力してください"
		if err == nil {
			label = strconv.Itoa(n) + " 行目へ移動"
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

func (p *palette) render(w int) []string {
	var out []string
	label := " > "
	if p.prompt != "" {
		label = " " + p.prompt + ": "
	} else if p.picking {
		label = " " + p.pickTitle + " > "
	}
	p.input.SetWidth(max(1, w-ansi.StringWidth(label)-2))
	out = append(out, stPaletteBox.Render(fit("", w)))
	out = append(out, stPaletteBox.Render(label)+fitStyled(p.input.View(), w-ansi.StringWidth(label), stPalette))
	start := 0
	if p.sel >= maxPaletteItems {
		start = p.sel - maxPaletteItems + 1
	}
	for i := start; i < len(p.items) && i < start+maxPaletteItems; i++ {
		out = append(out, renderItem(p.items[i], w, i == p.sel))
	}
	if p.prompt == "" && len(p.items) == 0 {
		out = append(out, stPaletteDim.Render(fit("  一致する項目はありません", w)))
	}
	out = append(out, stPalette.Render(fit("", w)))
	return out
}

func renderItem(it paletteItem, w int, selected bool) string {
	base, hi := stPalette, stPaletteHi
	if selected {
		base, hi = stPaletteSel, stPaletteHiS
	}
	matched := map[int]bool{}
	for _, m := range it.matched {
		matched[m] = true
	}
	var sb strings.Builder
	sb.WriteString(base.Render("  "))
	used := 2
	for i, r := range it.label {
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
	if it.detail != "" && it.detail != "." {
		d := "  " + it.detail
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
	p.input.Placeholder = "絞り込み"
	p.input.SetValue("")
	p.sel = 0
	return p.input.Focus()
}

// pickLive is pick for a list the language server filters: the list is not
// fuzzy-filtered here. The caller asks the server after each edit of the
// input (queryLater) and puts the answer in with setPickItems.
func (p *palette) pickLive(title string, onPick func(int) tea.Cmd) tea.Cmd {
	cmd := p.pick(title, nil, onPick)
	p.live = true
	p.liveSeq++
	p.input.Placeholder = "名前を入力して検索"
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
