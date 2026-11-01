package core

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// Node is a file or directory in the file tree.
type Node struct {
	Name     string
	Path     string
	isDir    bool
	expanded bool
	loaded   bool
	depth    int
	parent   *Node
	children []*Node
}

// Explorer is the file tree shown in the sidebar.
type Explorer struct {
	root   *Node
	Flat   []*Node
	Sel    int
	Scroll int
	Git    map[string]string
	// ignored holds paths matched by .gitignore; the tree shows them dimmed.
	// A directory entry stands for everything under it.
	ignored map[string]bool
	Icons   bool // Nerd Font icons; off falls back to ▸/▾
}

// NewExplorer is the file tree of root, with root expanded.
func NewExplorer(root string) *Explorer {
	e := &Explorer{root: &Node{Name: filepath.Base(root), Path: root, isDir: true, expanded: true, depth: -1}}
	e.load(e.root)
	e.flatten()
	e.loadGitStatus()
	return e
}

var hiddenNames = map[string]bool{".git": true}

func (e *Explorer) load(n *Node) {
	entries, err := os.ReadDir(n.Path)
	n.loaded = true
	if err != nil {
		return
	}
	old := map[string]*Node{}
	for _, c := range n.children {
		old[c.Name] = c
	}
	n.children = n.children[:0]
	for _, ent := range entries {
		if hiddenNames[ent.Name()] {
			continue
		}
		isDir := ent.IsDir()
		if ent.Type()&os.ModeSymlink != 0 {
			if st, err := os.Stat(filepath.Join(n.Path, ent.Name())); err == nil {
				isDir = st.IsDir()
			}
		}
		c := old[ent.Name()]
		if c == nil || c.isDir != isDir {
			c = &Node{Name: ent.Name(), Path: filepath.Join(n.Path, ent.Name()), isDir: isDir}
		}
		c.depth = n.depth + 1
		c.parent = n
		n.children = append(n.children, c)
	}
	sort.SliceStable(n.children, func(i, j int) bool { return nodeLess(n.children[i], n.children[j]) })
}

// nodeLess orders: dot dirs, dirs, dot files, files.
func nodeLess(a, b *Node) bool {
	if a.isDir != b.isDir {
		return a.isDir
	}
	ad, bd := strings.HasPrefix(a.Name, "."), strings.HasPrefix(b.Name, ".")
	if ad != bd {
		return ad
	}
	return strings.ToLower(a.Name) < strings.ToLower(b.Name)
}

func (e *Explorer) flatten() {
	e.Flat = e.Flat[:0]
	var walk func(n *Node)
	walk = func(n *Node) {
		for _, c := range n.children {
			e.Flat = append(e.Flat, c)
			if c.isDir && c.expanded {
				walk(c)
			}
		}
	}
	walk(e.root)
	e.Sel = Clamp(e.Sel, 0, max(0, len(e.Flat)-1))
}

// Refresh reloads every expanded directory from disk.
func (e *Explorer) Refresh() {
	var selPath string
	if n := e.selected(); n != nil {
		selPath = n.Path
	}
	var walk func(n *Node)
	walk = func(n *Node) {
		if n.isDir && n.expanded {
			e.load(n)
			for _, c := range n.children {
				walk(c)
			}
		}
	}
	walk(e.root)
	e.flatten()
	for i, n := range e.Flat { // keep the cursor on the same entry when rows shift
		if n.Path == selPath {
			e.Sel = i
		}
	}
	e.Sel = Clamp(e.Sel, 0, max(0, len(e.Flat)-1))
	e.loadGitStatus()
}

func (e *Explorer) selected() *Node {
	if e.Sel < 0 || e.Sel >= len(e.Flat) {
		return nil
	}
	return e.Flat[e.Sel]
}

// Toggle expands or collapses the directory n.
func (e *Explorer) Toggle(n *Node) {
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
func (e *Explorer) collapseOrParent() {
	n := e.selected()
	if n == nil {
		return
	}
	if n.isDir && n.expanded {
		e.Toggle(n)
		return
	}
	if n.parent != nil && n.parent != e.root {
		for i, f := range e.Flat {
			if f == n.parent {
				e.Sel = i
				return
			}
		}
	}
}

// Reveal expands parents of path and selects it.
func (e *Explorer) Reveal(path string) {
	rel, err := filepath.Rel(e.root.Path, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return
	}
	cur := e.root
	for part := range strings.SplitSeq(rel, string(filepath.Separator)) {
		if !cur.loaded {
			e.load(cur)
		}
		var next *Node
		for _, c := range cur.children {
			if c.Name == part {
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
	for i, f := range e.Flat {
		if f.Path == path {
			e.Sel = i
		}
	}
}

// targetDir is where new files are created: the selected dir, or the
// selected file's parent.
func (e *Explorer) targetDir() string {
	n := e.selected()
	if n == nil {
		return e.root.Path
	}
	if n.isDir {
		return n.Path
	}
	return filepath.Dir(n.Path)
}

// EnsureVisible scrolls the tree so the selected row is within h rows.
func (e *Explorer) EnsureVisible(h int) {
	if e.Sel < e.Scroll {
		e.Scroll = e.Sel
	}
	if e.Sel >= e.Scroll+h {
		e.Scroll = e.Sel - h + 1
	}
	e.Scroll = Clamp(e.Scroll, 0, max(0, len(e.Flat)-h))
}

// Screen is the tree as a screen.Explorer: the rows in view, h rows high with
// the title, scrolled as fit left it. activePath is the file open in the editor.
// It changes nothing.
func (e *Explorer) Screen(h int, activePath string) screen.Explorer {
	ex := screen.Explorer{
		Title: " EXPLORER: " + strings.ToUpper(e.root.Name),
		Icons: e.Icons,
		Top:   e.Scroll,
		Total: len(e.Flat),
	}
	for i := range max(0, h-1) {
		idx := e.Scroll + i
		if idx >= len(e.Flat) {
			break
		}
		n := e.Flat[idx]
		r := screen.ExplorerRow{
			Depth: n.depth, Name: n.Name, Dir: n.isDir, Expanded: n.expanded,
			Open: n.Path == activePath, Selected: idx == e.Sel,
			Git: e.Git[n.Path], Ignored: e.IsIgnored(n),
		}
		if e.Icons {
			ic := IconFor(n)
			r.Icon, r.IconColor = ic.Glyph+" ", ic.Color
		}
		ex.Rows = append(ex.Rows, r)
	}
	return ex
}

// IsIgnored reports whether n or one of its ancestors is matched by .gitignore.
func (e *Explorer) IsIgnored(n *Node) bool {
	for ; n != nil; n = n.parent {
		if e.ignored[n.Path] {
			return true
		}
	}
	return false
}

// loadGitStatus records git status letters for files under the root,
// shown as VS Code-style decorations in the tree, and the paths that
// .gitignore matches.
func (e *Explorer) loadGitStatus() {
	e.Git = map[string]string{}
	e.ignored = map[string]bool{}
	top, err := run(e.root.Path, nil, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return
	}
	topDir := strings.TrimSpace(string(top))
	// --ignored=matching lists an ignored directory as one entry instead of
	// every file under it. Git before 2.18 lacks it; fall back to no ignores.
	out, err := run(e.root.Path, nil, "git", "status", "--porcelain", "--untracked-files=all", "--ignored=matching", "-z")
	if err != nil {
		out, err = run(e.root.Path, nil, "git", "status", "--porcelain", "--untracked-files=all", "-z")
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
		e.Git[abs] = letter
		for d := filepath.Dir(abs); len(d) >= len(topDir) && d != filepath.Dir(d); d = filepath.Dir(d) {
			if _, ok := e.Git[d]; !ok {
				e.Git[d] = "•"
			}
		}
	}
}
