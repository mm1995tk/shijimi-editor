package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// gitDiffRefreshTicks is how many watch ticks pass between refreshes of the
// CHANGES list in diff mode.
const gitDiffRefreshTicks = 3

// treeRefreshTicks is how many watch ticks pass between reloads of the file
// tree (directory contents and git marks), so changes made outside the editor
// show up.
const treeRefreshTicks = 3

// watchInterval is how often open files are compared with the disk.
const watchInterval = time.Second

type watchMsg struct{}

func watchTick() tea.Cmd {
	return tea.Tick(watchInterval, func(time.Time) tea.Msg { return watchMsg{} })
}

// checkDiskChanges follows edits other processes (e.g. an AI agent) made to
// open files. A clean buffer is reloaded; a buffer with unsaved edits is left
// alone and the user decides: :reload takes the disk version, saving keeps
// theirs.
func (a *App) checkDiskChanges() (reloaded bool) {
	if !a.cfg.Editor.WatchFiles {
		return false
	}
	var reloadedTabs, conflicts []*tab
	for _, t := range a.tabs {
		if t.kind != tabText {
			continue
		}
		if !t.buf.DiskChanged() {
			t.conflict = false
			continue
		}
		if t.dirty() {
			if !t.conflict {
				t.conflict = true
				conflicts = append(conflicts, t)
			}
			continue
		}
		if err := t.reload(); err != nil {
			a.fail(err)
			continue
		}
		reloadedTabs = append(reloadedTabs, t)
	}
	switch {
	case len(conflicts) > 0:
		a.msg, a.msgErr = i18n.T("watch.changed1")+a.rel(conflicts[0].buf.Path)+i18n.T("watch.changed2"), true
	case len(reloadedTabs) > 0:
		a.info(i18n.T("watch.reflected"), a.rel(reloadedTabs[0].buf.Path))
	}
	return len(reloadedTabs) > 0
}
