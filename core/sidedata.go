package core

import (
	"github.com/mm1995tk/shijimi-editor/core/msg"
	"github.com/mm1995tk/shijimi-editor/core/screen"
)

// sidedata.go is the data the TUI draws its three panels from (the tree, the
// list of changes, the agent panel; docs/design/gui-protocol.md). They are not
// part of the Screen: the TUI makes its own rows of them. The GUI draws the
// tree and the changes from the tree API and git instead. Reading changes
// nothing.

// ChangesList is the list of changed files of the diff mode in the part that
// is in view (as FitChanges left it).
type ChangesList struct {
	Files   int  // number of changed files
	PR      bool // the base is a pull request, with its review
	Viewed  int  // PR: files marked as viewed
	Base    string
	Top     int // Rows[0] in the whole list
	Total   int // rows of the whole list; 0: it is loading, failed, or has no changes
	Loading bool
	Err     msg.Msg
	Rows    []ChangeRow
}

// ChangeRow is a directory or a file of the list.
type ChangeRow struct {
	Depth    int
	Name     string
	Dir      bool
	Closed   bool // Dir: its files are hidden
	Selected bool

	Status  string // a file: the Status of its diff.File ("added", "deleted", "renamed", "binary", ...)
	Added   int
	Deleted int
	Viewed  bool // PR: the file is marked as viewed
	Stale   bool // PR: the file changed after the review
	Threads int  // PR: review threads in the file
}

// ChangesOf is the list of changes, h rows high with the title.
func (a *Core) ChangesOf(h int) ChangesList {
	g := &a.Gd
	pr := a.PrShown()
	vis := g.vis()
	l := ChangesList{Files: len(g.Files), PR: pr, Top: g.scroll, Total: len(vis), Loading: g.Loading, Err: g.err}
	if g.Base == BasePR {
		l.Base = "PR"
	} else {
		l.Base = "DIFF"
	}
	if pr {
		l.Viewed = g.Rv.viewedCount(g.Files)
	}
	for i := range max(0, h-1) {
		idx := g.scroll + i
		if idx >= len(vis) {
			break
		}
		row := g.Rows[vis[idx]]
		r := ChangeRow{Depth: row.depth, Name: row.name}
		if row.file < 0 {
			r.Dir, r.Closed = true, g.closed[row.path]
			r.Selected = g.selDir == row.path
			l.Rows = append(l.Rows, r)
			continue
		}
		f := g.Files[row.file]
		r.Selected = row.file == g.Sel && g.selDir == ""
		r.Status = f.Status
		r.Added, r.Deleted = f.Added, f.Deleted
		if pr {
			r.Viewed = g.Rv.Viewed[f.Path()]
			r.Stale = a.IsStale(f.Path())
			r.Threads = len(g.Rv.threadsIn(f.Path()))
		}
		l.Rows = append(l.Rows, r)
	}
	return l
}

// AgentPanel is what the agent panel shows, w columns wide and h rows high.
type AgentPanel struct {
	Count   int          // suggestions in all
	Items   []AgentOffer // the suggestions in view
	Started bool         // the agent runs or its client is up
	Working bool         // the agent is working on a request
	Name    string       // the agent's name; empty before it tells
	HasLog  bool         // the log has entries (kept after the agent stopped)
	Perms   [][]string   // the permission requests, each wrapped to the panel
	Log     []AgentLogRow
	Hint    string // the line at the foot, only while the panel has the keys
}

// AgentOffer is a suggestion.
type AgentOffer struct {
	Title, Detail string
	Selected      bool
}

// AgentLogRow is a row of the log in view with the part of it that is selected
// (columns [SelFrom, SelTo); none when SelFrom >= SelTo).
type AgentLogRow struct {
	Line           screen.Line
	SelFrom, SelTo int
}

// AgentPanelOf is the panel's content as FitAgentPanel left it.
func (a *Core) AgentPanelOf(w, h int, focused bool) AgentPanel {
	lay := a.agentLayout(w, h, focused)
	s := a.Ag
	p := AgentPanel{
		Count:   len(lay.list),
		Started: s.Run != nil || s.Client != nil,
		Working: s.Run != nil,
		Name:    s.name,
		HasLog:  len(s.Log) > 0,
		Hint:    lay.hint,
	}
	for _, r := range lay.rows {
		if r < 0 {
			continue
		}
		p.Items = append(p.Items, AgentOffer{Title: a.text(lay.list[r].title), Detail: a.text(lay.list[r].Detail), Selected: r == lay.sel})
	}
	p.Perms = lay.perms
	v := s.View
	for i := v.Start; i < v.Start+v.n && i < len(v.Rows); i++ {
		r := AgentLogRow{Line: v.Rows[i].line}
		r.SelFrom, r.SelTo = s.Sel.rowCols(i, v.Rows[i].Plain, w)
		p.Log = append(p.Log, r)
	}
	return p
}
