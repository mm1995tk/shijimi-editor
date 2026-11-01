package core

import (
	"time"

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

// WatchTimer schedules the next WatchTick.
func WatchTimer() Effect { return StartTimer{After: watchInterval, Fire: WatchTick{}} }

// handleWatch runs one WatchTick and schedules the next.
func (a *Core) handleWatch() []Effect {
	if a.single {
		a.checkDiskChanges()
		return []Effect{WatchTimer()}
	}
	reload := a.checkDiskChanges()
	sg := a.suggestTick()
	a.treeTicks++
	if reload || a.treeTicks >= treeRefreshTicks {
		a.treeTicks = 0
		a.Explorer.Refresh()
	}
	effs := append([]Effect{WatchTimer()}, sg...)
	if a.Gd.On {
		a.Gd.ticks++
		if reload || a.Gd.ticks%gitDiffRefreshTicks == 0 {
			effs = append(effs, a.ReloadChanges()...)
		}
	}
	return effs
}

// checkDiskChanges follows edits other processes (e.g. an AI agent) made to
// open files. A clean buffer is reloaded; a buffer with unsaved edits is left
// alone and the user decides: :reload takes the disk version, saving keeps
// theirs.
func (a *Core) checkDiskChanges() (reloaded bool) {
	if !a.Cfg.Editor.WatchFiles {
		return false
	}
	var reloadedTabs, conflicts []*Tab
	for _, t := range a.Tabs {
		if t.Kind != TabText {
			continue
		}
		if !t.Buf.DiskChanged() {
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
			a.Fail(err)
			continue
		}
		reloadedTabs = append(reloadedTabs, t)
	}
	switch {
	case len(conflicts) > 0:
		a.Msg, a.MsgErr = i18n.T("watch.changed1")+a.Rel(conflicts[0].Buf.Path)+i18n.T("watch.changed2"), true
	case len(reloadedTabs) > 0:
		a.Info(i18n.T("watch.reflected"), a.Rel(reloadedTabs[0].Buf.Path))
	}
	return len(reloadedTabs) > 0
}
