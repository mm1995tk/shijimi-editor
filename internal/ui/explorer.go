package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type node struct {
	name     string
	path     string
	isDir    bool
	expanded bool
	loaded   bool
	depth    int
	parent   *node
	children []*node
}

// explorer is the file tree shown in the sidebar.
type explorer struct {
	root   *node
	flat   []*node
	sel    int
	scroll int
	git    map[string]string
	// ignored holds paths matched by .gitignore; the tree shows them dimmed.
	// A directory entry stands for everything under it.
	ignored map[string]bool
	icons   bool // Nerd Font icons; off falls back to ▸/▾
}

func newExplorer(root string) *explorer {
	e := &explorer{root: &node{name: filepath.Base(root), path: root, isDir: true, expanded: true, depth: -1}}
	e.load(e.root)
	e.flatten()
	e.loadGitStatus()
	return e
}

var hiddenNames = map[string]bool{".git": true}

func (e *explorer) load(n *node) {
	entries, err := os.ReadDir(n.path)
	n.loaded = true
	if err != nil {
		return
	}
	old := map[string]*node{}
	for _, c := range n.children {
		old[c.name] = c
	}
	n.children = n.children[:0]
	for _, ent := range entries {
		if hiddenNames[ent.Name()] {
			continue
		}
		isDir := ent.IsDir()
		if ent.Type()&os.ModeSymlink != 0 {
			if st, err := os.Stat(filepath.Join(n.path, ent.Name())); err == nil {
				isDir = st.IsDir()
			}
		}
		c := old[ent.Name()]
		if c == nil || c.isDir != isDir {
			c = &node{name: ent.Name(), path: filepath.Join(n.path, ent.Name()), isDir: isDir}
		}
		c.depth = n.depth + 1
		c.parent = n
		n.children = append(n.children, c)
	}
	sort.SliceStable(n.children, func(i, j int) bool { return nodeLess(n.children[i], n.children[j]) })
}

// nodeLess orders: dot dirs, dirs, dot files, files.
func nodeLess(a, b *node) bool {
	if a.isDir != b.isDir {
		return a.isDir
	}
	ad, bd := strings.HasPrefix(a.name, "."), strings.HasPrefix(b.name, ".")
	if ad != bd {
		return ad
	}
	return strings.ToLower(a.name) < strings.ToLower(b.name)
}

func (e *explorer) flatten() {
	e.flat = e.flat[:0]
	var walk func(n *node)
	walk = func(n *node) {
		for _, c := range n.children {
			e.flat = append(e.flat, c)
			if c.isDir && c.expanded {
				walk(c)
			}
		}
	}
	walk(e.root)
	e.sel = clamp(e.sel, 0, max(0, len(e.flat)-1))
}

// refresh reloads every expanded directory from disk.
func (e *explorer) refresh() {
	var selPath string
	if n := e.selected(); n != nil {
		selPath = n.path
	}
	var walk func(n *node)
	walk = func(n *node) {
		if n.isDir && n.expanded {
			e.load(n)
			for _, c := range n.children {
				walk(c)
			}
		}
	}
	walk(e.root)
	e.flatten()
	for i, n := range e.flat { // keep the cursor on the same entry when rows shift
		if n.path == selPath {
			e.sel = i
		}
	}
	e.sel = clamp(e.sel, 0, max(0, len(e.flat)-1))
	e.loadGitStatus()
}

func (e *explorer) selected() *node {
	if e.sel < 0 || e.sel >= len(e.flat) {
		return nil
	}
	return e.flat[e.sel]
}

func (e *explorer) toggle(n *node) {
	if !n.isDir {
		return
	}
	n.expanded = !n.expanded
	if n.expanded && !n.loaded {
		e.load(n)
	}
	e.flatten()
}

// collapseOrParent collapses the selected dir, or moves to its parent.
func (e *explorer) collapseOrParent() {
	n := e.selected()
	if n == nil {
		return
	}
	if n.isDir && n.expanded {
		e.toggle(n)
		return
	}
	if n.parent != nil && n.parent != e.root {
		for i, f := range e.flat {
			if f == n.parent {
				e.sel = i
				return
			}
		}
	}
}

// reveal expands parents of path and selects it.
func (e *explorer) reveal(path string) {
	rel, err := filepath.Rel(e.root.path, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return
	}
	cur := e.root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if !cur.loaded {
			e.load(cur)
		}
		var next *node
		for _, c := range cur.children {
			if c.name == part {
				next = c
				break
			}
		}
		if next == nil {
			break
		}
		if next.isDir {
			next.expanded = true
			if !next.loaded {
				e.load(next)
			}
		}
		cur = next
	}
	e.flatten()
	for i, f := range e.flat {
		if f.path == path {
			e.sel = i
		}
	}
}

// targetDir is where new files are created: the selected dir, or the
// selected file's parent.
func (e *explorer) targetDir() string {
	n := e.selected()
	if n == nil {
		return e.root.path
	}
	if n.isDir {
		return n.path
	}
	return filepath.Dir(n.path)
}

func (e *explorer) ensureVisible(h int) {
	if e.sel < e.scroll {
		e.scroll = e.sel
	}
	if e.sel >= e.scroll+h {
		e.scroll = e.sel - h + 1
	}
	e.scroll = clamp(e.scroll, 0, max(0, len(e.flat)-h))
}

func (e *explorer) render(w, h int, focused bool, activePath string) []string {
	lines := make([]string, 0, h)
	title := " EXPLORER: " + strings.ToUpper(e.root.name)
	lines = append(lines, stSideTitle.Render(fit(title, w)))
	body := h - 1
	e.ensureVisible(body)
	for i := 0; i < body; i++ {
		idx := e.scroll + i
		if idx >= len(e.flat) {
			lines = append(lines, stSide.Render(fit("", w)))
			continue
		}
		n := e.flat[idx]
		chevron := "  "
		if n.isDir {
			chevron = "▸ "
			if n.expanded {
				chevron = "▾ "
			}
			if e.icons {
				chevron = " " // codicon chevron-right
				if n.expanded {
					chevron = " " // codicon chevron-down
				}
			}
		}
		icon := ""
		iconColor := ""
		if e.icons {
			ic := iconFor(n)
			icon, iconColor = ic.glyph+" ", ic.color
		}
		// Each nesting level draws a faint guide under its parent's chevron.
		indent := " " + strings.Repeat("│ ", n.depth)
		st := stSide
		if n.isDir {
			st = stSideDir
		}
		if n.path == activePath {
			st = stSideActive
		}
		if idx == e.sel {
			if focused {
				st = stSideSelFocus
			} else {
				st = stSideSel
			}
		}
		letter := e.git[n.path]
		textSt, tail, avail := st, "", w
		if e.isIgnored(n) {
			textSt = st.Foreground(lipgloss.Color(curTheme.Dim))
		} else if letter != "" {
			col := gitColor(letter)
			if letter == "•" {
				col = cDirty
			}
			textSt = st.Foreground(lipgloss.Color(col))
			tail = textSt.Render(" " + letter)
			avail = w - 2
		}
		iconSt := textSt
		if iconColor != "" && !e.isIgnored(n) {
			iconSt = st.Foreground(lipgloss.Color(iconColor))
		}
		// Segments are styled separately so each keeps the row background.
		guideSt := st.UnsetBold().Foreground(lipgloss.Color(curTheme.Dim))
		head := guideSt.Render(indent) + st.Render(chevron) + iconSt.Render(icon)
		lead := indent + chevron + icon
		name := ansi.Truncate(n.name, max(0, avail-ansi.StringWidth(lead)), "…")
		used := ansi.StringWidth(lead) + ansi.StringWidth(name)
		line := head + textSt.Render(name) + textSt.Render(strings.Repeat(" ", max(0, avail-used)))
		lines = append(lines, ansi.Truncate(line, avail, "")+tail)
	}
	return lines
}

// isIgnored reports whether n or one of its ancestors is matched by .gitignore.
func (e *explorer) isIgnored(n *node) bool {
	for ; n != nil; n = n.parent {
		if e.ignored[n.path] {
			return true
		}
	}
	return false
}

// loadGitStatus records git status letters for files under the root,
// shown as VS Code-style decorations in the tree, and the paths that
// .gitignore matches.
func (e *explorer) loadGitStatus() {
	e.git = map[string]string{}
	e.ignored = map[string]bool{}
	top, err := run(e.root.path, nil, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return
	}
	topDir := strings.TrimSpace(string(top))
	// --ignored=matching lists an ignored directory as one entry instead of
	// every file under it. Git before 2.18 lacks it; fall back to no ignores.
	out, err := run(e.root.path, nil, "git", "status", "--porcelain", "--untracked-files=all", "--ignored=matching", "-z")
	if err != nil {
		out, err = run(e.root.path, nil, "git", "status", "--porcelain", "--untracked-files=all", "-z")
		if err != nil {
			return
		}
	}
	parts := strings.Split(string(out), "\x00")
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		if len(p) < 4 {
			continue
		}
		xy, path := p[:2], p[3:]
		if xy[0] == 'R' || xy[0] == 'C' {
			i++ // the original path follows
		}
		if xy == "!!" {
			e.ignored[filepath.Join(topDir, strings.TrimSuffix(path, "/"))] = true
			continue
		}
		letter := "M"
		switch {
		case xy == "??":
			letter = "U"
		case xy[0] == 'A':
			letter = "A"
		case xy[0] == 'D' || xy[1] == 'D':
			letter = "D"
		case xy[0] == 'R':
			letter = "R"
		}
		abs := filepath.Join(topDir, path)
		e.git[abs] = letter
		for d := filepath.Dir(abs); len(d) >= len(topDir) && d != filepath.Dir(d); d = filepath.Dir(d) {
			if _, ok := e.git[d]; !ok {
				e.git[d] = "•"
			}
		}
	}
}

func gitColor(letter string) string {
	switch letter {
	case "U", "A":
		return curTheme.GitAdded
	case "D":
		return curTheme.GitDeleted
	case "R":
		return curTheme.GitAdded
	}
	return cDirty
}
