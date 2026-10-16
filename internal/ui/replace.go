package ui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mm1995tk/gh-editor/internal/buffer"
)

// replaceSep separates the search text from the replacement in
// `:replace [-x globs] [-i globs] query => replacement`.
const replaceSep = " => "

// replaceItem is one changed line. All occurrences on the line are replaced
// together; skip excludes the line from the replacement.
type replaceItem struct {
	row      int
	old, new string
	spans    [][2]int // rune offsets of each occurrence in old
	skip     bool
}

type replaceFile struct {
	path  string
	items []*replaceItem
	skip  bool
}

// replaceView is the tab that previews a project-wide replacement.
type replaceView struct {
	root        string
	query, repl string
	files       []*replaceFile
	cursor      int // index into entries()
	scroll      int
	pageH       int
}

type replaceMsg struct {
	query, repl string
	files       []*replaceFile
	err         error
}

// replaceEntry is a selectable row: a file header (item == nil) or a line.
type replaceEntry struct {
	file *replaceFile
	item *replaceItem
}

func (v *replaceView) tabTitle() string { return "置換: " + v.query }

func (v *replaceView) entries() []replaceEntry {
	var out []replaceEntry
	for _, f := range v.files {
		out = append(out, replaceEntry{file: f})
		for _, it := range f.items {
			out = append(out, replaceEntry{file: f, item: it})
		}
	}
	return out
}

// active counts the lines that will be changed and the files they are in.
func (v *replaceView) active() (lines, files int) {
	for _, f := range v.files {
		n := 0
		if !f.skip {
			for _, it := range f.items {
				if !it.skip {
					n++
				}
			}
		}
		lines += n
		if n > 0 {
			files++
		}
	}
	return
}

func (v *replaceView) toggle() {
	es := v.entries()
	if v.cursor >= len(es) {
		return
	}
	e := es[v.cursor]
	if e.item != nil {
		e.item.skip = !e.item.skip
	} else {
		e.file.skip = !e.file.skip
	}
}

func (v *replaceView) move(d int) {
	v.cursor = clamp(v.cursor+d, 0, max(0, len(v.entries())-1))
}

// splitReplaceArgs separates `query => replacement` (the first " => " wins).
func splitReplaceArgs(arg string) (query, repl string, ok bool) {
	q, r, ok := strings.Cut(arg, replaceSep)
	if !ok {
		// allow an empty replacement: `query =>`
		if t := strings.TrimSuffix(strings.TrimRight(arg, " "), " =>"); t != strings.TrimRight(arg, " ") {
			return t, "", true
		}
	}
	return q, r, ok
}

// literalRegexp matches q literally, case-insensitively unless q has an
// upper-case letter (the same smart-case rule as rg).
func literalRegexp(q string) *regexp.Regexp {
	pat := regexp.QuoteMeta(q)
	if !strings.ContainsFunc(q, unicode.IsUpper) {
		pat = "(?i)" + pat
	}
	return regexp.MustCompile(pat)
}

// planLine computes the replacement for one line; ok is false when q does
// not occur in it.
func planLine(re *regexp.Regexp, line, repl string) (it *replaceItem, ok bool) {
	locs := re.FindAllStringIndex(line, -1)
	if len(locs) == 0 {
		return nil, false
	}
	it = &replaceItem{old: line}
	var sb strings.Builder
	last := 0
	for _, l := range locs {
		sb.WriteString(line[last:l[0]])
		sb.WriteString(repl)
		last = l[1]
		it.spans = append(it.spans, [2]int{utf8.RuneCountInString(line[:l[0]]), utf8.RuneCountInString(line[:l[1]])})
	}
	sb.WriteString(line[last:])
	it.new = sb.String()
	return it, true
}

func (a *App) replace(arg string) tea.Cmd {
	body, repl, ok := splitReplaceArgs(arg)
	q, include, exclude := parseGrepArgs(body, a.cfg.Search)
	if !ok || q == "" {
		a.fail(fmt.Errorf("使い方: :replace [-x 除外] [-i 対象] 検索文字列%s置換文字列", replaceSep))
		return a.openCmdline(":replace " + strings.TrimSpace(arg))
	}
	root := a.root
	// open buffers may differ from disk; read them from the buffer instead
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
	a.info("検索しています…")
	return func() tea.Msg {
		re := literalRegexp(q)
		var paths []string
		seen := map[string]bool{}
		for _, h := range searchHits(root, q, include, exclude, true) {
			if !seen[h.path] {
				seen[h.path] = true
				paths = append(paths, h.path)
			}
		}
		// unsaved buffers may contain matches that are not on disk yet
		for p := range open {
			if !seen[p] && inRoot(root, p) {
				paths = append(paths, p)
				seen[p] = true
			}
		}
		var files []*replaceFile
		for _, p := range paths {
			lines, ok := open[p]
			if !ok {
				b, err := buffer.Open(p)
				if err != nil {
					continue // binary or unreadable
				}
				lines = make([]string, len(b.Lines))
				for i, l := range b.Lines {
					lines[i] = string(l)
				}
			}
			f := &replaceFile{path: p}
			for row, l := range lines {
				if it, ok := planLine(re, l, repl); ok {
					it.row = row
					f.items = append(f.items, it)
				}
			}
			if len(f.items) > 0 {
				files = append(files, f)
			}
		}
		return replaceMsg{query: q, repl: repl, files: files}
	}
}

func inRoot(root, p string) bool {
	r, err := filepath.Rel(root, p)
	return err == nil && !strings.HasPrefix(r, "..")
}

func (a *App) showReplace(m replaceMsg) tea.Cmd {
	if m.err != nil {
		a.fail(m.err)
		return nil
	}
	if len(m.files) == 0 {
		a.fail(fmt.Errorf("見つかりません: %s", m.query))
		return nil
	}
	a.addTab(&tab{kind: tabReplace, rep: &replaceView{root: a.root, query: m.query, repl: m.repl, files: m.files}})
	a.focus = focusEditor
	a.info("")
	return nil
}

// apply replaces the checked lines. Every affected file is opened as a
// buffer and edited in memory as one undo step; nothing is written to disk
// until the user saves, so Ctrl+Z / u reverts the whole replacement.
func (a *App) applyReplace(v *replaceView) tea.Cmd {
	lines, nfiles := v.active()
	if lines == 0 {
		a.fail(fmt.Errorf("置換する行がありません"))
		return nil
	}
	a.closeTab(true)
	var failed []string
	for _, f := range v.files {
		if f.skip {
			continue
		}
		var edits []buffer.Edit
		for _, it := range f.items {
			if it.skip {
				continue
			}
			for _, sp := range it.spans {
				edits = append(edits, buffer.Edit{Start: buffer.Pos{Row: it.row, Col: sp[0]}, End: buffer.Pos{Row: it.row, Col: sp[1]}, Text: v.repl})
			}
		}
		if len(edits) == 0 {
			continue
		}
		if !a.openFile(f.path) {
			failed = append(failed, a.rel(f.path))
			continue
		}
		if t := a.curText(); t != nil {
			t.buf.ApplyEdits(edits)
		}
	}
	if len(failed) > 0 {
		a.fail(fmt.Errorf("開けなかったファイル: %s", strings.Join(failed, ", ")))
		return nil
	}
	a.info("%d 行 (%d ファイル) を置換しました。保存するまでディスクは変わりません（u で元に戻せます）", lines, nfiles)
	return nil
}

func (a *App) handleReplaceKey(t *tab, msg tea.KeyMsg) tea.Cmd {
	v := t.rep
	key := msg.String()
	if a.pending == " " {
		a.pending = ""
		return a.spaceMode(key)
	}
	half := max(1, v.pageH/2)
	switch key {
	case "q", "esc":
		a.closeTab(true)
	case ":":
		return a.openCmdline(":")
	case " ":
		a.pending = " "
	case "j", "down":
		v.move(1)
	case "k", "up":
		v.move(-1)
	case "ctrl+d", "pgdown":
		v.move(half)
	case "ctrl+u", "pgup":
		v.move(-half)
	case "g", "home":
		v.cursor = 0
	case "G", "end":
		v.cursor = max(0, len(v.entries())-1)
	case "x", "tab":
		v.toggle()
	case "enter", "A":
		return a.applyReplace(v)
	}
	return nil
}

func (v *replaceView) render(w, h int, focused bool) []string {
	lines, nfiles := v.active()
	head := fmt.Sprintf(" 置換: %q → %q   %d 行 / %d ファイル", v.query, v.repl, lines, nfiles)
	out := []string{bgfg(cTabBarBg, "#ffffff").Bold(true).Render(fit(head, w))}
	body := h - 2
	v.pageH = body

	type row struct {
		text, bg, fg string
		bold         bool
		entry        int
	}
	var rows []row
	e := 0
	for _, f := range v.files {
		mark := "[x]"
		if f.skip {
			mark = "[ ]"
		}
		rows = append(rows, row{text: fmt.Sprintf(" %s %s (%d)", mark, relTo(v.root, f.path), len(f.items)), bg: cEditorBg, fg: cEditorFg, bold: true, entry: e})
		e++
		for _, it := range f.items {
			mark := "[x]"
			if it.skip || f.skip {
				mark = "[ ]"
			}
			rows = append(rows,
				row{text: fmt.Sprintf("   %s %5d - %s", mark, it.row+1, it.old), bg: cDelBg, fg: cEditorFg, entry: e},
				row{text: fmt.Sprintf("           + %s", it.new), bg: cAddBg, fg: cEditorFg, entry: -1})
			e++
		}
	}
	// keep the cursor entry in view
	first, last := -1, -1
	for i, r := range rows {
		if r.entry == v.cursor {
			if first < 0 {
				first = i
			}
			last = i
			if r.entry >= 0 && i+1 < len(rows) && rows[i+1].entry == -1 {
				last = i + 1
			}
		}
	}
	if first >= 0 {
		if first < v.scroll {
			v.scroll = first
		}
		if last >= v.scroll+body {
			v.scroll = last - body + 1
		}
	}
	v.scroll = clamp(v.scroll, 0, max(0, len(rows)-body))
	for i := v.scroll; i < len(rows) && i < v.scroll+body; i++ {
		r := rows[i]
		st := bgfg(r.bg, r.fg).Bold(r.bold)
		if focused && r.entry == v.cursor {
			st = bgfg(cListSel, "#ffffff").Bold(r.bold)
		}
		out = append(out, st.Render(fit(r.text, w)))
	}
	for len(out) < h-1 {
		out = append(out, stEmpty.Render(fit("", w)))
	}
	return append(out, stPanelDim.Render(fit(" j/k:移動  x:除外/復帰（ファイル行ならファイルごと）  Enter:置換を実行  q:閉じる", w)))
}
