package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
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
}

func newExplorer(root string) *explorer {
	e := &explorer{root: &node{name: filepath.Base(root), path: root, isDir: true, expanded: true, depth: -1}}
	e.load(e.root)
	e.flatten()
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
	sort.SliceStable(n.children, func(i, j int) bool {
		a, b := n.children[i], n.children[j]
		if a.isDir != b.isDir {
			return a.isDir
		}
		return strings.ToLower(a.name) < strings.ToLower(b.name)
	})
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
		icon := "  "
		if n.isDir {
			icon = "▸ "
			if n.expanded {
				icon = "▾ "
			}
		}
		label := strings.Repeat("  ", n.depth) + " " + icon + n.name
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
		lines = append(lines, st.Render(fit(label, w)))
	}
	return lines
}
