package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// The right panel: suggestions at the top, then the agent's permission
// requests and its log.

const (
	minAgentW = 24
	// The editor keeps this many columns beside the panel.
	minEditorBeside = 30
)

// agentWidth is the panel width: the user's choice, or a quarter of the screen.
func (a *App) agentWidth() int {
	if a.agentWidthPref() > 0 {
		return clamp(a.agentWidthPref(), minAgentW, max(minAgentW, a.screenW()/2))
	}
	return clamp(a.screenW()/4, 30, 50)
}

// agentShown reports whether the panel fits beside the editor.
func (a *App) agentShown(sideW int) bool {
	if !a.agentOpen() {
		return false
	}
	used := 0
	if a.sideShown() {
		used = sideW + 1
	}
	return a.screenW()-used-a.agentWidth()-1 >= minEditorBeside
}

// toggleAgentPanel moves to the panel, opening it, or leaves and closes it.
func (a *App) toggleAgentPanel() []Effect {
	if a.single {
		return nil
	}
	if a.focus == focusAgent {
		a.setAgent(false)
		a.follow = true
		if len(a.tabs) > 0 {
			return a.setFocus(focusEditor)
		}
		a.setSide(true)
		return a.setFocus(focusExplorer)
	}
	a.setAgent(true)
	a.follow = true
	return a.setFocus(focusAgent)
}

// leaveAgentPanel returns to the editor, keeping the panel open.
func (a *App) leaveAgentPanel() []Effect {
	if len(a.tabs) > 0 {
		return a.setFocus(focusEditor)
	}
	a.setSide(true)
	return a.setFocus(focusExplorer)
}

func (a *App) handleAgentKey(msg Key) []Effect {
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
func (a *App) agentCommand(arg string) []Effect {
	switch arg {
	case "":
		return a.toggleAgentPanel()
	case "cancel":
		a.cancelAgent()
	case "stop":
		if a.ag.client == nil {
			a.infoText(i18n.T("agent.notrunning"))
			return nil
		}
		a.stopAgent()
		a.ag.add(logEntry{kind: logInfo, text: i18n.T("agent.stoppedok")})
		a.infoText(i18n.T("agent.stoppedok"))
	default:
		a.fail(fmt.Errorf(i18n.T("agent.badarg"), arg))
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

// agentLay is how the panel is laid out in an area of w x h: what draws it
// and what fits the scroll to it (fitAgent) both read it.
type agentLay struct {
	list     []offer
	sel      int    // the selected suggestion, clamped to list
	rows     []int  // suggestRows of list
	head     int    // rows above the card: the title, the suggestions, the status
	hint     string // the line at the foot, only while the panel has the keys
	body     int    // rows between head and the hint
	cardRows int    // rows of the permission cards that fit in body
	logRows  int    // rows left for the log
	perms    []screen.AgentPerm
}

// agentLayout lays the panel out. It changes nothing.
func (a *App) agentLayout(w, h int, focused bool) agentLay {
	var lay agentLay
	lay.list = a.suggestions()
	lay.sel = clamp(a.sg.sel, 0, max(0, len(lay.list)-1))
	lay.rows = a.suggestRows(lay.list, max(2, h/2-1))
	lay.head = 1 + len(lay.rows)
	if len(lay.list) == 0 {
		lay.head++
	}
	if s := a.ag; s.run != nil || s.client != nil {
		lay.head += 2
	} else if len(s.log) > 0 {
		lay.head++
	}
	if focused {
		lay.hint = i18n.T("panel.hint.suggest")
		switch {
		case len(a.ag.perms) > 0:
			lay.hint = i18n.T("panel.hint.perm")
		case a.ag.run != nil:
			lay.hint = i18n.T("panel.hint.work")
		}
	}
	lay.body = h - lay.head - boolInt(lay.hint != "")
	cards := 0
	for _, p := range a.ag.perms {
		lines := wrapPlain(permTitle(p.req), w-2)
		lay.perms = append(lay.perms, screen.AgentPerm{Title: i18n.T("panel.permtitle"), Lines: lines, Keys: i18n.T("panel.permkeys")})
		cards += 2 + len(lines)
	}
	lay.cardRows = min(cards, max(0, lay.body))
	lay.logRows = lay.body - lay.cardRows
	return lay
}

// agentScreen is the panel as a screen.Agent, w columns wide and h rows
// high. The log is the part of it that fitAgent put in view. It changes
// nothing.
func (a *App) agentScreen(w, h int, focused bool) screen.Agent {
	lay := a.agentLayout(w, h, focused)
	ag := screen.Agent{
		Title: fmt.Sprintf(i18n.T("panel.suggestions"), len(lay.list)),
		Hint:  lay.hint,
		Perms: lay.perms,
	}
	if len(lay.list) == 0 {
		ag.Empty = i18n.T("panel.nosuggestions")
	}
	for _, r := range lay.rows {
		if r < 0 {
			continue
		}
		ag.Items = append(ag.Items, screen.AgentItem{Title: lay.list[r].title, Detail: lay.list[r].detail, Selected: r == lay.sel})
	}
	// The agent starts only when a suggestion runs, so its status line stays
	// hidden until then; a log left by an agent that stopped keeps the rule.
	if s := a.ag; s.run != nil || s.client != nil {
		status := i18n.T("panel.idle")
		if s.run != nil {
			status = i18n.T("panel.working")
		}
		name := s.name
		if name == "" {
			name = i18n.T("panel.agent")
		}
		ag.Rule, ag.Status = true, " "+name+": "+status
	} else if len(s.log) > 0 {
		ag.Rule = true
	}
	v := a.ag.view
	for i := v.start; i < v.start+v.n && i < len(v.rows); i++ {
		r := screen.AgentRow{Line: v.rows[i].line}
		r.SelFrom, r.SelTo = a.ag.sel.rowCols(i, v.rows[i].plain, w)
		ag.Log = append(ag.Log, r)
	}
	return ag
}

// drawAgent draws the panel as h lines of w cells.
func drawAgent(ag screen.Agent, w, h int, focused bool) []string {
	out := make([]string, 0, h)
	out = append(out, stPanelTitle.Render(fit(ag.Title, w)))
	if ag.Empty != "" {
		out = append(out, stPanelDim.Render(fit(ag.Empty, w)))
	}
	for _, s := range ag.Items {
		st := stPanel
		mark := "   "
		if s.Selected {
			mark = " ▸ "
			st = stSideSel
			if focused {
				st = stSideSelFocus
			}
		}
		out = append(out, st.Render(fit(mark+s.Title, w)))
		out = append(out, stPanelDim.Render(fit("     "+s.Detail, w)))
	}
	if ag.Rule {
		out = append(out, stPanelBord.Render(strings.Repeat("─", w)))
	}
	if ag.Status != "" {
		out = append(out, stPanelTitle.Render(fit(ag.Status, w)))
	}
	body := h - len(out) - boolInt(ag.Hint != "")
	var card []string
	for _, p := range ag.Perms {
		card = append(card, stPaletteHi.Render(fit(p.Title, w)))
		for _, l := range p.Lines {
			card = append(card, stPalette.Render(fit("  "+l, w)))
		}
		card = append(card, stPaletteDim.Render(fit(p.Keys, w)))
	}
	if len(card) > body {
		card = card[:max(0, body)]
	}
	out = append(out, card...)
	for _, r := range ag.Log {
		row := drawMdLine(r.Line, w)
		if r.SelFrom < r.SelTo {
			row = markRow(row, w, r.SelFrom, r.SelTo)
		}
		out = append(out, row)
	}
	for len(out) < h-boolInt(ag.Hint != "") {
		out = append(out, stPanel.Render(strings.Repeat(" ", w)))
	}
	if ag.Hint != "" {
		out = append(out, stPanelDim.Render(fit(" "+ag.Hint, w)))
	}
	return out[:h]
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// logSpanRow is a row of the log made of spans, padded to w columns.
func logSpanRow(w int, spans ...screen.Span) logRow {
	var sb strings.Builder
	for _, sp := range spans {
		sb.WriteString(sp.Text)
	}
	plain := sb.String()
	if pad := w - ansi.StringWidth(plain); pad > 0 {
		plain += strings.Repeat(" ", pad)
	}
	if spans == nil {
		spans = []screen.Span{}
	}
	return logRow{line: screen.Line{Spans: spans}, plain: plain}
}

// renderLog lays the log out at width w.
func (a *App) renderLog(w int) []logRow {
	var out []logRow
	// styled rows are padded to w in the text, so that their background reaches the edge
	addWrapped := func(text string, ww int, style, bg screen.Style) {
		for _, r := range wrapPlainRows(text, ww) {
			row := logSpanRow(w, screen.Span{Text: fit(" "+r.text, w), Style: style, Bg: bg})
			row.cont, row.space = r.cont, r.space
			out = append(out, row)
		}
	}
	for _, e := range a.ag.log {
		switch e.kind {
		case logUser:
			out = append(out, logSpanRow(w))
			addWrapped("› "+e.text, w-1, screen.StylePanelCmd, "")
		case logAgent:
			if strings.TrimSpace(e.text) == "" {
				continue
			}
			type cut struct {
				space  bool
				indent int
			}
			cuts := map[int]cut{} // row of the markdown -> the cut above it
			exact := map[int][2]int{}
			doc := renderMarkdown(e.text, max(1, w-1), mdOpts{
				wrapped: func(row int, space bool, indent int) { cuts[row] = cut{space, indent} },
				exact:   func(row, from, to int) { exact[row] = [2]int{from, to} },
			})
			for i, l := range doc.Lines {
				c, cont := cuts[i]
				if cont && len(out) > 0 {
					out[len(out)-1].space = c.space
				}
				r := logSpanRow(w, append([]screen.Span{plainSpan(" ")}, l.Spans...)...)
				r.cont, r.indent = cont, 1+c.indent
				if x, ok := exact[i]; ok {
					r.from, r.to = 1+x[0], 1+x[1]
				}
				out = append(out, r)
			}
		case logTool:
			mark := map[string]string{"pending": "○", "in_progress": "◐", "completed": "●", "failed": "✗"}[e.status]
			if mark == "" {
				mark = "○"
			}
			addWrapped(mark+" "+e.text, w-1, screen.StyleLineNo, "")
		case logPlan, logInfo:
			addWrapped(strings.TrimRight(e.text, "\n"), w-1, screen.StyleLineNo, "")
		case logError:
			addWrapped(e.text, w-1, screen.StyleStatusFg, screen.StyleStatusErrBg)
		}
	}
	return out
}

// wrapPlain wraps text without styles to w columns, keeping its line breaks.
func wrapPlain(text string, w int) []string {
	var out []string
	for _, r := range wrapPlainRows(text, w) {
		out = append(out, r.text)
	}
	return out
}

// wrapRow is a row of wrapped text. cont and space mean what they do in logRow;
// space is set on the row above the cut.
type wrapRow struct {
	text        string
	cont, space bool
}

// wrapPlainRows is wrapPlain that also says which rows continue the one above.
func wrapPlainRows(text string, w int) []wrapRow {
	w = max(1, w)
	var out []wrapRow
	for line := range strings.SplitSeq(text, "\n") {
		pos := 0
		for i, piece := range strings.Split(ansi.Wrap(line, w, ""), "\n") {
			if i > 0 {
				// the wrapper drops the spaces at a cut
				out[len(out)-1].space = pos < len(line) && line[pos] == ' ' || strings.HasSuffix(out[len(out)-1].text, " ")
				for pos < len(line) && line[pos] == ' ' {
					pos++
				}
			}
			if strings.HasPrefix(line[pos:], piece) {
				pos += len(piece)
			}
			out = append(out, wrapRow{text: piece, cont: i > 0})
		}
	}
	return out
}

// ---- mouse ----

// agentMouse handles the pointer over the panel. ok is false when (x, y)
// is not on it.
func (a *App) agentMouse(msg Mouse) ([]Effect, bool) {
	if msg.Region != RegionAgent {
		return nil, false
	}
	if msg.Action == MouseMotion && msg.Button == MouseNone {
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
	if msg.Action != MousePress {
		return nil, true
	}
	switch msg.Button {
	case MouseWheelUp:
		a.ag.scroll += 3
		return nil, true
	case MouseWheelDown:
		a.ag.scroll = max(0, a.ag.scroll-3)
		return nil, true
	case MouseLeft:
	default:
		return nil, true
	}
	if msg.Col == -1 { // the border
		a.dragBorder(resizeAgent)
		return nil, true
	}
	row := msg.Row - 1 // below the title
	list := a.suggestions()
	if rows := a.suggestRows(list, max(2, a.bodyH()/2-1)); row >= 0 && row < len(rows) {
		i := rows[row]
		if i < 0 {
			i = rows[row-1]
		}
		a.sg.sel = i
	}
	a.ag.sel = agentSel{}
	if a.inLog(msg) {
		p, _, _ := a.logPos(msg)
		a.ag.sel = agentSel{dragging: true, a: p, b: p}
	}
	return a.setFocus(focusAgent), true
}
