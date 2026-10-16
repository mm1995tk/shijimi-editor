package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

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
func (a *App) checkDiskChanges() {
	if !a.cfg.Editor.WatchFiles {
		return
	}
	var reloaded, conflicts []*tab
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
		reloaded = append(reloaded, t)
	}
	switch {
	case len(conflicts) > 0:
		a.msg, a.msgErr = "ディスク上で変更されました（未保存の編集あり）: "+a.rel(conflicts[0].buf.Path)+" — :reload でディスクの内容を読み込みます。保存すると上書きします", true
	case len(reloaded) > 0:
		a.info("外部の変更を反映しました: %s（Ctrl+G で差分をレビュー）", a.rel(reloaded[0].buf.Path))
	}
}
