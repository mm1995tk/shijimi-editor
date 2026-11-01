package tui

import (
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestSpaceMenuPopup(t *testing.T) {
	a := newTestApp(t, "hello\n")
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if strings.Contains(a.render(), "ファイルを開く") {
		t.Fatal("menu shown before Space")
	}
	keys(a, "space")
	if !strings.Contains(a.render(), "ファイルを開く") {
		t.Fatal("menu not shown after Space")
	}
	keys(a, "esc")
	if strings.Contains(a.render(), "ファイルを開く") {
		t.Fatal("menu still shown after Esc")
	}
	keys(a, "space", "z")
	if strings.Contains(a.render(), "ファイルを開く") {
		t.Fatal("menu still shown after unassigned key")
	}
}

func TestSpaceMenuFiltersItems(t *testing.T) {
	a := newTestApp(t, "hello\n")

	a.Focus = core.FocusExplorer
	keys(a, "space")
	v := a.render()
	if !strings.Contains(v, "ファイルを開く") || !strings.Contains(v, "名前の変更") {
		t.Fatal("explorer: want file and buffer commands")
	}
	keys(a, "esc")
	a.Focus = core.FocusEditor

	a.CurText().Preview = core.PreviewFull
	keys(a, "space")
	v = a.render()
	if !strings.Contains(v, "ファイルを開く") || strings.Contains(v, "名前の変更") || strings.Contains(v, "貼り付け") {
		t.Fatal("full preview: want only commands usable there")
	}
	keys(a, "esc")
	a.CurText().Preview = core.PreviewOff

	a.CloseTab(true)
	keys(a, "space")
	v = a.render()
	if !strings.Contains(v, "ファイルを開く") || strings.Contains(v, "名前の変更") || strings.Contains(v, "コピー") {
		t.Fatal("no text tab: want text-only commands hidden")
	}
}
