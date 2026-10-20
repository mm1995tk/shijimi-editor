package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// The right panel: suggestions at the top, then the agent's permission
// requests and its log.

const (
	minAgentW = 24
	// The editor keeps this many columns beside the panel: more while the
	// panel is open by default, fewer once the user has opened it.
	autoEditorBeside   = 80
	pinnedEditorBeside = 30
)

// agentWidth is the panel width: the user's choice, or a quarter of the screen.
func (a *App) agentWidth() int {
	if a.agentWPref > 0 {
		return clamp(a.agentWPref, minAgentW, max(minAgentW, a.w/2))
	}
	return clamp(a.w/4, 30, 50)
}

// agentShown reports whether the panel fits beside the editor.
func (a *App) agentShown(sideW int) bool {
	if !a.showAgent {
		return false
	}
	used := 0
	if a.showSide {
		used = sideW + 1
	}
	need := autoEditorBeside
	if a.agentPinned || a.focus == focusAgent {
		need = pinnedEditorBeside
	}
	return a.w-used-a.agentWidth()-1 >= need
}

// toggleAgentPanel moves to the panel, opening it, or leaves and closes it.
func (a *App) toggleAgentPanel() tea.Cmd {
	if a.focus == focusAgent {
		a.showAgent = false
		a.agentPinned = false
		a.follow = true
		if len(a.tabs) > 0 {
			return a.setFocus(focusEditor)
		}
		a.showSide = true
		return a.setFocus(focusExplorer)
	}
	a.agentPinned = true
	a.showAgent = true
	a.follow = true
	return a.setFocus(focusAgent)
}

// leaveAgentPanel returns to the editor, keeping the panel open.
func (a *App) leaveAgentPanel() tea.Cmd {
	if len(a.tabs) > 0 {
		return a.setFocus(focusEditor)
	}
	a.showSide = true
	return a.setFocus(focusExplorer)
}

func (a *App) handleAgentKey(msg tea.KeyPressMsg) tea.Cmd {
	n := len(a.suggestions())
	switch msg.String() {
	case "j", "down":
		a.sg.sel = clamp(a.sg.sel+1, 0, max(0, n-1))
	case "k", "up":
		a.sg.sel = clamp(a.sg.sel-1, 0, max(0, n-1))
	case "enter":
		return a.runSuggestion()
	case "x":
		a.dismissSuggestion()
	case "y":
		a.answerPermission(true)
	case "n":
		a.answerPermission(false)
	case "c":
		a.cancelAgent()
	case "ctrl+u", "pgup":
		a.ag.scroll += 10
	case "ctrl+d", "pgdown":
		a.ag.scroll = max(0, a.ag.scroll-10)
	case "G", "end":
		a.ag.scroll = 0
	case ":":
		return a.openCmdline(":")
	case "esc", "tab":
		return a.leaveAgentPanel()
	case "alt+i":
		return a.toggleAgentPanel()
	}
	return nil
}

// agentCommand runs ":agent [cancel|stop]".
func (a *App) agentCommand(arg string) tea.Cmd {
	switch arg {
	case "":
		return a.toggleAgentPanel()
	case "cancel":
		a.cancelAgent()
	case "stop":
		if a.ag.client == nil {
			a.info("エージェントは動いていません")
			return nil
		}
		a.stopAgent()
		a.ag.add(logEntry{kind: logInfo, text: "エージェントを止めました"})
		a.info("エージェントを止めました")
	default:
		a.fail(fmt.Errorf("不明な引数: %s（:agent / :agent cancel / :agent stop）", arg))
	}
	return nil
}

// ---- drawing ----

// suggestRows is where the suggestion rows of the panel start, and the
// suggestion each row shows (-1 for a detail line).
func (a *App) suggestRows(list []offer, limit int) []int {
	var rows []int
	for i := range list {
		if len(rows)+2 > limit {
			break
		}
		rows = append(rows, i, -1)
	}
	return rows
}

// renderAgent draws the panel, h rows of w columns.
func (a *App) renderAgent(w, h int, focused bool) []string {
	out := make([]string, 0, h)
	list := a.suggestions()
	a.sg.sel = clamp(a.sg.sel, 0, max(0, len(list)-1))
	out = append(out, stPanelTitle.Render(fit(fmt.Sprintf(" 提案 %d", len(list)), w)))
	if len(list) == 0 {
		out = append(out, stPanelDim.Render(fit("  今の状態に合う提案はありません", w)))
	}
	maxRows := max(2, h/2-1)
	for _, r := range a.suggestRows(list, maxRows) {
		if r < 0 {
			continue
		}
		s := list[r]
		st := stPanel
		mark := "   "
		if r == a.sg.sel {
			mark = " ▸ "
			st = stSideSel
			if focused {
				st = stSideSelFocus
			}
		}
		out = append(out, st.Render(fit(mark+s.title, w)))
		out = append(out, stPanelDim.Render(fit("     "+s.detail, w)))
	}

	status := "未接続"
	switch s := a.ag; {
	case s.run != nil:
		status = "● 作業中（c で中止）"
	case s.client != nil:
		status = "待機中"
	}
	name := a.ag.name
	if name == "" {
		name = "エージェント"
	}
	out = append(out, stPanelBord.Render(strings.Repeat("─", w)))
	out = append(out, stPanelTitle.Render(fit(" "+name+": "+status, w)))

	var hint string
	if focused {
		hint = "Enter 実行  x 消す  j/k 選ぶ  Esc 戻る"
		switch {
		case len(a.ag.perms) > 0:
			hint = "y 許可  n 拒否  c 中止"
		case a.ag.run != nil:
			hint = "c 中止  Ctrl+U/D スクロール  Esc 戻る"
		}
	}
	body := h - len(out)
	if hint != "" {
		body--
	}
	var card []string
	for _, p := range a.ag.perms {
		card = append(card, stPaletteHi.Render(fit(" 許可を求めています", w)))
		for _, l := range wrapPlain(permTitle(p.req), w-2) {
			card = append(card, stPalette.Render(fit("  "+l, w)))
		}
		card = append(card, stPaletteDim.Render(fit("  y 許可 / n 拒否", w)))
	}
	if len(card) > body {
		card = card[:body]
	}
	out = append(out, card...)
	body -= len(card)

	log := a.renderLog(w)
	a.ag.scroll = clamp(a.ag.scroll, 0, max(0, len(log)-body))
	end := len(log) - a.ag.scroll
	start := max(0, end-body)
	out = append(out, log[start:end]...)
	for len(out) < h-boolInt(hint != "") {
		out = append(out, stPanel.Render(strings.Repeat(" ", w)))
	}
	if hint != "" {
		out = append(out, stPanelDim.Render(fit(" "+hint, w)))
	}
	return out[:h]
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// renderLog lays the log out at width w.
func (a *App) renderLog(w int) []string {
	var out []string
	for _, e := range a.ag.log {
		switch e.kind {
		case logUser:
			out = append(out, stPanel.Render(strings.Repeat(" ", w)))
			for _, l := range wrapPlain("› "+e.text, w-1) {
				out = append(out, stPanelCmd.Render(fit(" "+l, w)))
			}
		case logAgent:
			if strings.TrimSpace(e.text) == "" {
				continue
			}
			for _, l := range renderMarkdown(e.text, max(1, w-1), mdOpts{}) {
				out = append(out, stPanel.Render(" ")+l)
			}
		case logTool:
			mark := map[string]string{"pending": "○", "in_progress": "◐", "completed": "●", "failed": "✗"}[e.status]
			if mark == "" {
				mark = "○"
			}
			for _, l := range wrapPlain(mark+" "+e.text, w-1) {
				out = append(out, stPanelDim.Render(fit(" "+l, w)))
			}
		case logPlan, logInfo:
			for _, l := range wrapPlain(strings.TrimRight(e.text, "\n"), w-1) {
				out = append(out, stPanelDim.Render(fit(" "+l, w)))
			}
		case logError:
			for _, l := range wrapPlain(e.text, w-1) {
				out = append(out, stStatusError.Render(fit(" "+l, w)))
			}
		}
	}
	return out
}

// wrapPlain wraps text without styles to w columns, keeping its line breaks.
func wrapPlain(text string, w int) []string {
	w = max(1, w)
	var out []string
	for _, line := range strings.Split(text, "\n") {
		out = append(out, strings.Split(ansi.Wrap(line, w, ""), "\n")...)
	}
	return out
}

// ---- mouse ----

// agentMouse handles the pointer over the panel. ok is false when (x, y)
// is not on it.
func (a *App) agentMouse(l layout, msg mouseEvent) (tea.Cmd, bool) {
	if l.agentW == 0 || msg.X < l.agentX-1 || msg.Y < l.top || msg.Y >= l.top+l.bodyH {
		return nil, false
	}
	if msg.Action == mouseMotion && msg.Button == tea.MouseNone {
		// the pointer left the editor: popups it opened close
		if c := a.card; c != nil && c.byPointer {
			a.card = nil
		}
		if a.hoverAt != nil {
			a.hoverText, a.hoverAt = "", nil
		}
		a.hoverSeq++
		a.hoverWord = wordSpan{}
		a.hoverCell = screenPt{-1, -1}
		return nil, true
	}
	if msg.Action != mousePress {
		return nil, true
	}
	switch msg.Button {
	case tea.MouseWheelUp:
		a.ag.scroll += 3
		return nil, true
	case tea.MouseWheelDown:
		a.ag.scroll = max(0, a.ag.scroll-3)
		return nil, true
	case tea.MouseLeft:
	default:
		return nil, true
	}
	if msg.X == l.agentX-1 {
		a.resizing = resizeAgent
		return nil, true
	}
	row := msg.Y - l.top - 1 // below the title
	list := a.suggestions()
	if rows := a.suggestRows(list, max(2, l.bodyH/2-1)); row >= 0 && row < len(rows) {
		i := rows[row]
		if i < 0 {
			i = rows[row-1]
		}
		a.sg.sel = i
	}
	return a.setFocus(focusAgent), true
}
