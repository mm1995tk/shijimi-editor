package core

import (
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sahilm/fuzzy"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

const (
	MaxPaletteItems = 12
	maxIndexedFiles = 50000
)

// PaletteItem is a row of the palette.
type PaletteItem struct {
	Label   string
	Detail  string
	matched []int // byte offsets into label
	index   int
}

// Palette is the overlay used for Quick Open (Ctrl+P), the command palette
// (">" prefix), go-to-line (":" prefix) and free-text prompts.
type Palette struct {
	Open  bool
	Input InputField
	Sel   int
	items []PaletteItem
	files []string

	// prompt mode
	prompt   string
	onSubmit func(value string) []Effect

	// pick mode
	picking   bool
	pickTitle string
	pickAll   []PaletteItem
	OnPick    func(index int) []Effect

	// live is set when the list comes from a server query (see pickLive);
	// liveSeq numbers the edits so that late answers can be told apart.
	live    bool
	liveSeq int
}

func newPalette() *Palette {
	return &Palette{Input: NewInputField(4096)}
}

func indexFiles(root string) []string {
	var files []string
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != root && SkipDirs[d.Name()] {
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

// Show opens the palette in quick-open/command mode with initial text.
func (p *Palette) Show(root, initial string) []Effect {
	p.Open = true
	p.picking = false
	p.live = false
	p.prompt = ""
	p.onSubmit = nil
	p.files = indexFiles(root)
	p.Input.Placeholder = i18n.T("palette.placeholder")
	p.Input.SetValue(initial)
	p.Input.CursorEnd()
	p.Sel = 0
	p.Input.Focus()
	return nil
}

// Ask opens the palette as a single-line prompt.
func (p *Palette) Ask(prompt, initial string, onSubmit func(string) []Effect) []Effect {
	p.Open = true
	p.picking = false
	p.live = false
	p.prompt = prompt
	p.onSubmit = onSubmit
	p.items = nil
	p.Input.Placeholder = ""
	p.Input.SetValue(initial)
	p.Input.CursorEnd()
	p.Sel = 0
	p.Input.Focus()
	return nil
}

// Close closes the palette.
func (p *Palette) Close() {
	p.Open = false
	p.Input.Blur()
}

func (p *Palette) filter(cmds []Command) {
	v := p.Input.Value()
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
			labels[i] = it.Label
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
			names[i] = c.Name
		}
		if q == "" {
			for i, c := range cmds {
				p.items = append(p.items, PaletteItem{Label: c.Name, Detail: c.Key, index: i})
			}
		} else {
			for _, m := range fuzzy.Find(q, names) {
				p.items = append(p.items, PaletteItem{Label: m.Str, Detail: cmds[m.Index].Key, matched: m.MatchedIndexes, index: m.Index})
			}
		}
	case strings.HasPrefix(v, ":"):
		n, err := strconv.Atoi(strings.TrimSpace(v[1:]))
		label := i18n.T("palette.needline")
		if err == nil {
			label = strconv.Itoa(n) + i18n.T("palette.gotoline")
		}
		p.items = append(p.items, PaletteItem{Label: label, index: -1})
	default:
		q := strings.TrimSpace(v)
		if q == "" {
			for i, f := range p.files {
				if i >= 200 {
					break
				}
				p.items = append(p.items, PaletteItem{Label: filepath.Base(f), Detail: filepath.Dir(f), index: i})
			}
		} else {
			for i, m := range fuzzy.Find(q, p.files) {
				if i >= 200 {
					break
				}
				p.items = append(p.items, PaletteItem{Label: m.Str, matched: m.MatchedIndexes, index: m.Index})
			}
		}
	}
	p.Sel = Clamp(p.Sel, 0, max(0, len(p.items)-1))
}

// head is the palette's heading and the mark after it: a prompt asks with
// ":", the rest search or pick with ">".
func (p *Palette) head() (title, prefix string) {
	switch {
	case p.prompt != "":
		return p.prompt, ":"
	case p.picking:
		return p.pickTitle, ">"
	}
	return "", ">"
}

// PaletteLabel is what precedes the input on the palette's first row.
func PaletteLabel(title, prefix string) string {
	switch {
	case prefix == ":":
		return " " + title + ": "
	case title != "":
		return " " + title + " > "
	}
	return " > "
}

// label is what precedes the input on the palette's first row.
func (p *Palette) label() string { return PaletteLabel(p.head()) }

// Overlay is the palette as a screen.Overlay: the input field and the rows in
// view (MaxPaletteItems at most, scrolled so that the selected one shows) with
// Active the selected one among them. It has no place in a pane, so it has no
// At (the TUI centres it). It changes nothing. Drawing it as cells is
// drawPalette's work.
func (p *Palette) Overlay() screen.Overlay {
	title, prefix := p.head()
	ov := screen.Overlay{
		Kind:  screen.OverlayPalette,
		Title: title,
		Field: &screen.Field{
			Prefix:      prefix,
			Text:        p.Input.Value(),
			Cursor:      p.Input.Position(),
			Placeholder: p.Input.Placeholder,
		},
	}
	start := 0
	if p.Sel >= MaxPaletteItems {
		start = p.Sel - MaxPaletteItems + 1
	}
	ov.Active = p.Sel - start
	for i := start; i < len(p.items) && i < start+MaxPaletteItems; i++ {
		it := p.items[i]
		item := screen.Item{Label: it.Label, Detail: it.Detail, Matched: runeIndexes(it.Label, it.matched)}
		ov.Items = append(ov.Items, item)
	}
	if p.prompt == "" && len(p.items) == 0 {
		ov.Hint = i18n.T("palette.nomatch")
	}
	return ov
}

// Pick opens the palette as a fuzzy list; onPick receives the index into items.
func (p *Palette) Pick(title string, items []PaletteItem, onPick func(int) []Effect) []Effect {
	p.Open = true
	p.picking = true
	p.prompt = ""
	p.pickTitle = title
	p.live = false
	for i := range items {
		items[i].index = i
	}
	p.pickAll = items
	p.OnPick = onPick
	p.Input.Placeholder = i18n.T("palette.filter")
	p.Input.SetValue("")
	p.Sel = 0
	p.Input.Focus()
	return nil
}

// pickLive is pick for a list the language server filters: the list is not
// fuzzy-filtered here. The caller asks the server after each edit of the
// input (queryLater) and puts the answer in with setPickItems.
func (p *Palette) pickLive(title string, onPick func(int) []Effect) []Effect {
	cmd := p.Pick(title, nil, onPick)
	p.live = true
	p.liveSeq++
	p.Input.Placeholder = i18n.T("palette.typename")
	return cmd
}

// setPickItems replaces the list of a pick palette and selects the first row.
func (p *Palette) setPickItems(items []PaletteItem) {
	for i := range items {
		items[i].index = i
	}
	p.pickAll = items
	p.Sel = 0
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

// PaletteWidth is the width of the palette window.
func (a *Core) PaletteWidth() int { return Clamp(a.ScreenW()*3/5, 30, 100) }
