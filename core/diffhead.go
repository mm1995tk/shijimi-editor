package core

import (
	"github.com/mm1995tk/shijimi-editor/core/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/core/msg"
)

// diffhead.go is the right side of diff mode when it is HEAD (RightHead): a
// changed file is shown as HEAD has it, read-only, in place of the file.
//
// Each open tab of a changed file gets a HEAD view (Tab.head): a tab of its
// own, not in Core.Tabs, whose buffer holds the file at the commit HEAD and
// refuses every edit. Cur (and so CurText, the keys and the drawing) gives
// the HEAD view in place of the tab, so the diff, the cursor, the search and
// the review work on it as they do on the file; what manages the tabs (closing,
// the last file, the split) and what follows the files (saving, the language
// servers, watching the disk) keeps to the tabs themselves, which stay as they
// are. syncHeads makes and drops the views after every message.

// curTab is the tab of the focused pane, or nil when there is none: Cur
// without the HEAD view that diff mode may show in its place.
func (a *Core) curTab() *Tab {
	if a.Active < 0 || a.Active >= len(a.Tabs) {
		return nil
	}
	return a.Tabs[a.Active]
}

// shownTab is what is shown for t: its HEAD view while diff mode shows HEAD on
// the right and t's file is one of the changes, t itself otherwise.
func (a *Core) shownTab(t *Tab) *Tab {
	if t != nil && t.head != nil && a.headShown() && t.headRev == a.Gd.head {
		return t.head
	}
	return t
}

// headShown reports whether diff mode shows HEAD on the right, with the list
// of changes read for it.
func (a *Core) headShown() bool {
	g := &a.Gd
	return g.On && g.RightSide() == RightHead && !g.Loading && g.head != ""
}

// ReadOnlyShown reports whether t is shown read-only: the HEAD view of a file.
func (t *Tab) ReadOnlyShown() bool { return t != nil && t.Kind == TabText && t.Buf.ReadOnly }

// syncHeads gives every open tab of a changed file its HEAD view while diff
// mode shows HEAD on the right (made again when HEAD moved), and drops them
// otherwise. A new view starts where the tab is.
func (a *Core) syncHeads() {
	on := a.headShown()
	for _, t := range a.Tabs {
		if t.Kind != TabText {
			continue
		}
		f := a.Gd.ByAbs[t.Buf.Path]
		if !on || t.Buf.Path == "" || f == nil || f.Status == "binary" || f.Status == "deleted" {
			t.head, t.headRev = nil, ""
			continue
		}
		if t.head != nil && t.headRev == a.Gd.head {
			continue
		}
		text := ""
		if out, err := run(a.GitTopDir(), nil, "git", "show", a.Gd.head+":"+f.Path()); err == nil {
			text = string(out)
		}
		h := NewTab(buffer.NewReadOnly(t.Buf.Path, text))
		h.Buf.UseTabs = t.Buf.UseTabs
		h.Buf.Cur = h.Buf.ClampPos(buffer.Pos{Row: t.Buf.Cur.Row})
		h.Buf.ScrollRow = min(t.Buf.ScrollRow, max(0, len(h.Buf.Lines)-1))
		t.head, t.headRev = h, a.Gd.head
		a.Follow = true
	}
}

// heads is the HEAD views of the open tabs.
func (a *Core) heads() []*Tab {
	var out []*Tab
	for _, t := range a.Tabs {
		if t.head != nil {
			out = append(out, t.head)
		}
	}
	return out
}

// tellRefused says so when the shown tab refused an edit (it is read-only).
func (a *Core) tellRefused() {
	for _, h := range a.heads() {
		if h.Buf.Refused {
			h.Buf.Refused = false
			a.Fail(msg.Err("gd.readonly"))
		}
	}
}
