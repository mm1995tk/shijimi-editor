package core

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

// AgentWidth is the panel width: the user's choice, or a quarter of the screen.
func (a *Core) AgentWidth() int {
	if a.AgentWidthPref() > 0 {
		return Clamp(a.AgentWidthPref(), minAgentW, max(minAgentW, a.ScreenW()/2))
	}
	return Clamp(a.ScreenW()/4, 30, 50)
}

// AgentShown reports whether the panel fits beside the editor.
func (a *Core) AgentShown(sideW int) bool {
	if !a.AgentOpen() {
		return false
	}
	used := 0
	if a.SideShown() {
		used = sideW + 1
	}
	return a.ScreenW()-used-a.AgentWidth()-1 >= minEditorBeside
}

// toggleAgentPanel moves to the panel, opening it, or leaves and closes it.
func (a *Core) toggleAgentPanel() []Effect {
	if a.single {
		return nil
	}
	if a.Focus == FocusAgent {
		a.SetAgent(false)
		a.Follow = true
		if len(a.Tabs) > 0 {
			return a.SetFocus(FocusEditor)
		}
		a.SetSide(true)
		return a.SetFocus(FocusExplorer)
	}
	a.SetAgent(true)
	a.Follow = true
	return a.SetFocus(FocusAgent)
}

// leaveAgentPanel returns to the editor, keeping the panel open.
func (a *Core) leaveAgentPanel() []Effect {
	if len(a.Tabs) > 0 {
		return a.SetFocus(FocusEditor)
	}
	a.SetSide(true)
	return a.SetFocus(FocusExplorer)
}

func (a *Core) handleAgentKey(msg Key) []Effect {
	n := len(a.Suggestions())
	switch msg.String() {
	case "j", "down":
		a.Sg.Sel = Clamp(a.Sg.Sel+1, 0, max(0, n-1))
	case "k", "up":
		a.Sg.Sel = Clamp(a.Sg.Sel-1, 0, max(0, n-1))
	case "enter":
		return a.runSuggestion()
	case "x":
		a.dismissSuggestion()
	case "y":
		a.AnswerPermission(true)
	case "n":
		a.AnswerPermission(false)
	case "c":
		a.cancelAgent()
	case "ctrl+u", "pgup":
		a.Ag.Scroll += 10
	case "ctrl+d", "pgdown":
		a.Ag.Scroll = max(0, a.Ag.Scroll-10)
	case "G", "end":
		a.Ag.Scroll = 0
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
func (a *Core) agentCommand(arg string) []Effect {
	switch arg {
	case "":
		return a.toggleAgentPanel()
	case "cancel":
		a.cancelAgent()
	case "stop":
		if a.Ag.Client == nil {
			a.InfoText(i18n.T("agent.notrunning"))
			return nil
		}
		a.stopAgent()
		a.Ag.add(LogEntry{Kind: LogInfo, Text: i18n.T("agent.stoppedok")})
		a.InfoText(i18n.T("agent.stoppedok"))
	default:
		a.Fail(fmt.Errorf(i18n.T("agent.badarg"), arg))
	}
	return nil
}

// ---- drawing ----

// suggestRows is where the suggestion rows of the panel start, and the
// suggestion each row shows (-1 for a detail line).
func (a *Core) suggestRows(list []offer, limit int) []int {
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
func (a *Core) agentLayout(w, h int, focused bool) agentLay {
	var lay agentLay
	lay.list = a.Suggestions()
	lay.sel = Clamp(a.Sg.Sel, 0, max(0, len(lay.list)-1))
	lay.rows = a.suggestRows(lay.list, max(2, h/2-1))
	lay.head = 1 + len(lay.rows)
	if len(lay.list) == 0 {
		lay.head++
	}
	if s := a.Ag; s.Run != nil || s.Client != nil {
		lay.head += 2
	} else if len(s.Log) > 0 {
		lay.head++
	}
	if focused {
		lay.hint = i18n.T("panel.hint.suggest")
		switch {
		case len(a.Ag.Perms) > 0:
			lay.hint = i18n.T("panel.hint.perm")
		case a.Ag.Run != nil:
			lay.hint = i18n.T("panel.hint.work")
		}
	}
	lay.body = h - lay.head - BoolInt(lay.hint != "")
	cards := 0
	for _, p := range a.Ag.Perms {
		lines := wrapPlain(permTitle(p.req), w-2)
		lay.perms = append(lay.perms, screen.AgentPerm{Title: i18n.T("panel.permtitle"), Lines: lines, Keys: i18n.T("panel.permkeys")})
		cards += 2 + len(lines)
	}
	lay.cardRows = min(cards, max(0, lay.body))
	lay.logRows = lay.body - lay.cardRows
	return lay
}

// AgentScreen is the panel as a screen.Agent, w columns wide and h rows
// high. The log is the part of it that fitAgent put in view. It changes
// nothing.
func (a *Core) AgentScreen(w, h int, focused bool) screen.Agent {
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
		ag.Items = append(ag.Items, screen.AgentItem{Title: lay.list[r].title, Detail: lay.list[r].Detail, Selected: r == lay.sel})
	}
	// The agent starts only when a suggestion runs, so its status line stays
	// hidden until then; a log left by an agent that stopped keeps the rule.
	if s := a.Ag; s.Run != nil || s.Client != nil {
		status := i18n.T("panel.idle")
		if s.Run != nil {
			status = i18n.T("panel.working")
		}
		name := s.name
		if name == "" {
			name = i18n.T("panel.agent")
		}
		ag.Rule, ag.Status = true, " "+name+": "+status
	} else if len(s.Log) > 0 {
		ag.Rule = true
	}
	v := a.Ag.View
	for i := v.Start; i < v.Start+v.n && i < len(v.Rows); i++ {
		r := screen.AgentRow{Line: v.Rows[i].line}
		r.SelFrom, r.SelTo = a.Ag.Sel.rowCols(i, v.Rows[i].Plain, w)
		ag.Log = append(ag.Log, r)
	}
	return ag
}

// BoolInt is 1 for true and 0 for false.
func BoolInt(b bool) int {
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
	return logRow{line: screen.Line{Spans: spans}, Plain: plain}
}

// renderLog lays the log out at width w.
func (a *Core) renderLog(w int) []logRow {
	var out []logRow
	// styled rows are padded to w in the text, so that their background reaches the edge
	addWrapped := func(text string, ww int, style, bg screen.Style) {
		for _, r := range wrapPlainRows(text, ww) {
			row := logSpanRow(w, screen.Span{Text: Fit(" "+r.text, w), Style: style, Bg: bg})
			row.cont, row.space = r.cont, r.space
			out = append(out, row)
		}
	}
	for _, e := range a.Ag.Log {
		switch e.Kind {
		case logUser:
			out = append(out, logSpanRow(w))
			addWrapped("› "+e.Text, w-1, screen.StylePanelCmd, "")
		case LogAgent:
			if strings.TrimSpace(e.Text) == "" {
				continue
			}
			type cut struct {
				space  bool
				indent int
			}
			cuts := map[int]cut{} // row of the markdown -> the cut above it
			exact := map[int][2]int{}
			doc := RenderMarkdown(e.Text, max(1, w-1), MdOpts{
				Wrapped: func(row int, space bool, indent int) { cuts[row] = cut{space, indent} },
				Exact:   func(row, from, to int) { exact[row] = [2]int{from, to} },
			})
			for i, l := range doc.Lines {
				c, cont := cuts[i]
				if cont && len(out) > 0 {
					out[len(out)-1].space = c.space
				}
				r := logSpanRow(w, append([]screen.Span{PlainSpan(" ")}, l.Spans...)...)
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
			addWrapped(mark+" "+e.Text, w-1, screen.StyleLineNo, "")
		case logPlan, LogInfo:
			addWrapped(strings.TrimRight(e.Text, "\n"), w-1, screen.StyleLineNo, "")
		case logError:
			addWrapped(e.Text, w-1, screen.StyleStatusFg, screen.StyleStatusErrBg)
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

// AgentMouse handles the pointer over the panel. ok is false when (x, y)
// is not on it.
func (a *Core) AgentMouse(msg Mouse) ([]Effect, bool) {
	if msg.Region != RegionAgent {
		return nil, false
	}
	if msg.Action == MouseMotion && msg.Button == MouseNone {
		// the pointer left the editor: popups it opened close
		if c := a.Card; c != nil && c.ByPointer {
			a.Card = nil
		}
		if a.HoverAt != nil {
			a.HoverText, a.HoverAt = "", nil
		}
		a.HoverSeq++
		a.HoverWord = WordSpan{}
		a.HoverCell = ScreenPt{-1, -1}
		return nil, true
	}
	if msg.Action != MousePress {
		return nil, true
	}
	switch msg.Button {
	case MouseWheelUp:
		a.Ag.Scroll += 3
		return nil, true
	case MouseWheelDown:
		a.Ag.Scroll = max(0, a.Ag.Scroll-3)
		return nil, true
	case MouseLeft:
	default:
		return nil, true
	}
	if msg.Col == -1 { // the border
		a.DragBorder(ResizeAgent)
		return nil, true
	}
	row := msg.Row - 1 // below the title
	list := a.Suggestions()
	if rows := a.suggestRows(list, max(2, a.BodyH()/2-1)); row >= 0 && row < len(rows) {
		i := rows[row]
		if i < 0 {
			i = rows[row-1]
		}
		a.Sg.Sel = i
	}
	a.Ag.Sel = AgentSel{}
	if a.inLog(msg) {
		p, _, _ := a.logPos(msg)
		a.Ag.Sel = AgentSel{Dragging: true, A: p, B: p}
	}
	return a.SetFocus(FocusAgent), true
}

// FitAgentPanel fits the panel's state to the area it is drawn in, w wide and
// h tall: the selected suggestion, the scroll and the selection of the log,
// and where the log sits (agentView, which the mouse reads). It is the part
// of drawing the panel that changes state, so it runs before drawing (fit).
func (a *Core) FitAgentPanel(w, h int) {
	lay := a.agentLayout(w, h, a.FocusIn(FocusAgent))
	a.Sg.Sel = lay.sel
	log := a.renderLog(w)
	a.Ag.Scroll = Clamp(a.Ag.Scroll, 0, max(0, len(log)-lay.logRows))
	end := len(log) - a.Ag.Scroll
	start := max(0, end-lay.logRows)
	if a.Ag.View.W != w {
		a.Ag.Sel = AgentSel{}
	}
	a.Ag.View = agentView{Rows: log, Y0: lay.head + lay.cardRows, Start: start, n: end - start, W: w}
	if a.Ag.Sel.A.Row >= len(log) || a.Ag.Sel.B.Row >= len(log) {
		a.Ag.Sel = AgentSel{}
	}
}
