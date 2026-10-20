package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
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
		if err := t.buf.Reload(); err != nil {
			a.fail(err)
			continue
		}
		reloadedTabs = append(reloadedTabs, t)
	}
	switch {
	case len(conflicts) > 0:
		a.msg, a.msgErr = "ディスク上で変更されました（未保存の編集あり）: "+a.rel(conflicts[0].buf.Path)+" — :reload でディスクの内容を読み込みます。保存すると上書きします", true
	case len(reloadedTabs) > 0:
		a.info("外部の変更を反映しました: %s（Ctrl+G で差分モード）", a.rel(reloadedTabs[0].buf.Path))
	}
	return len(reloadedTabs) > 0
}
