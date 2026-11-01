package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/internal/core"
)

// agentSelApp opens the panel with a log and returns the layout.
func agentSelApp(t *testing.T, log ...core.LogEntry) (*App, layout) {
	t.Helper()
	a := newTestApp(t, "x\n")
	a.HandleKey(core.Key{Code: 'i', Mod: core.ModAlt})
	a.Ag.Log = log
	a.render()
	return a, a.layout()
}

func agentRowY(a *App, l layout, i int) int { return l.top + a.Ag.View.Y0 + (i - a.Ag.View.Start) }

func TestAgentDragSelectCopies(t *testing.T) {
	a, l := agentSelApp(t, core.LogEntry{Kind: core.LogInfo, Text: "hello world\n日本語です"})
	y := agentRowY(a, l, 0)
	// the log starts with one column of indent
	a.HandleMouse(a.locate(l.agentX+7, y, core.Mouse{Action: core.MousePress, Button: core.MouseLeft}))
	a.HandleMouse(a.locate(l.agentX+4, y+1, core.Mouse{Action: core.MouseMotion, Button: core.MouseLeft}))
	a.HandleMouse(a.locate(l.agentX+4, y+1, core.Mouse{Action: core.MouseRelease, Button: core.MouseLeft}))
	if want := "world\n日本"; a.Clipboard != want {
		t.Fatalf("clipboard = %q, want %q", a.Clipboard, want)
	}
	if !strings.Contains(a.Msg, "8 文字") || !a.Ag.Sel.On {
		t.Fatalf("msg = %q sel = %+v", a.Msg, a.Ag.Sel)
	}
	drawn := strings.Join(a.renderAgent(l.agentW, l.bodyH, true), "\n")
	a.Ag.Sel.On = false
	plain := strings.Join(a.renderAgent(l.agentW, l.bodyH, true), "\n")
	a.Ag.Sel.On = true
	if drawn == plain || ansi.Strip(drawn) != ansi.Strip(plain) {
		t.Fatal("selection should change only the style of the text")
	}
	a.Update(core.Key{Code: 'a', Text: "a"})
	if a.Ag.Sel.On {
		t.Fatal("a key press should clear the selection")
	}
}

func TestAgentSelectionJoinsWrappedRows(t *testing.T) {
	long := strings.Repeat("word ", 20) + "https://example.com/" + strings.Repeat("x", 80)
	a, l := agentSelApp(t,
		core.LogEntry{Kind: core.LogAgent, Text: long},
		core.LogEntry{Kind: core.LogInfo, Text: long})
	n := len(a.Ag.View.Rows)
	if n < 6 {
		t.Fatalf("only %d rows", n)
	}
	a.Ag.Scroll = 0
	a.render()
	top := a.Ag.View.Start
	w := a.Ag.View.W
	// from the first row to the last, all of it
	a.Ag.Sel = core.AgentSel{On: true, A: core.AgentPos{top, 0}, B: core.AgentPos{top + n - 1, w - 1}}
	got := a.Ag.Sel.Text(a.Ag.View.Rows, w)
	if want := long + "\n" + long; got != want {
		t.Fatalf("copied\n%q\nwant\n%q", got, want)
	}
	_ = l
}

func TestAgentSelectionOutsidePanel(t *testing.T) {
	a, l := agentSelApp(t, core.LogEntry{Kind: core.LogInfo, Text: "abc\ndef"})
	y := agentRowY(a, l, 0)
	a.HandleMouse(a.locate(l.agentX+1, y, core.Mouse{Action: core.MousePress, Button: core.MouseLeft}))
	// dragged far left of the panel and below it
	a.HandleMouse(a.locate(0, y+1, core.Mouse{Action: core.MouseMotion, Button: core.MouseLeft}))
	a.HandleMouse(a.locate(0, a.h-1, core.Mouse{Action: core.MouseRelease, Button: core.MouseLeft}))
	if a.Ag.Sel.Dragging || a.Clipboard == "" || strings.Contains(a.Clipboard, "x") {
		t.Fatalf("sel = %+v clipboard = %q", a.Ag.Sel, a.Clipboard)
	}
}

func TestAgentClickDoesNotCopy(t *testing.T) {
	a, l := agentSelApp(t, core.LogEntry{Kind: core.LogInfo, Text: "hello"})
	a.Focus = core.FocusEditor
	a.Clipboard = "keep"
	y := agentRowY(a, l, 0)
	a.HandleMouse(a.locate(l.agentX+2, y, core.Mouse{Action: core.MousePress, Button: core.MouseLeft}))
	a.HandleMouse(a.locate(l.agentX+2, y, core.Mouse{Action: core.MouseRelease, Button: core.MouseLeft}))
	if a.Clipboard != "keep" || a.Ag.Sel.On || a.Focus != core.FocusAgent {
		t.Fatalf("clipboard = %q sel = %+v focus = %v", a.Clipboard, a.Ag.Sel, a.Focus)
	}
}

func TestAgentSelectionStaysOnTextWhenScrolled(t *testing.T) {
	var log []core.LogEntry
	for i := range 60 {
		log = append(log, core.LogEntry{Kind: core.LogInfo, Text: strings.Repeat("a", i%9+1)})
	}
	a, l := agentSelApp(t, log...)
	y := agentRowY(a, l, a.Ag.View.Start+2)
	a.HandleMouse(a.locate(l.agentX+1, y, core.Mouse{Action: core.MousePress, Button: core.MouseLeft}))
	a.HandleMouse(a.locate(l.agentX+3, y, core.Mouse{Action: core.MouseRelease, Button: core.MouseLeft}))
	before := ansi.Strip(a.Ag.View.Rows[a.Ag.Sel.A.Row].Plain)
	a.HandleMouse(a.locate(l.agentX+3, y, core.Mouse{Action: core.MousePress, Button: core.MouseWheelUp}))
	a.render()
	if !a.Ag.Sel.On || ansi.Strip(a.Ag.View.Rows[a.Ag.Sel.A.Row].Plain) != before {
		t.Fatalf("selection moved: %+v", a.Ag.Sel)
	}
}

func TestMarkRowKeepsColors(t *testing.T) {
	row := stStatusError.Render("abcd")
	got := core.MarkRow(row, 4, 1, 3)
	if ansi.Strip(got) != "abcd" || !strings.Contains(got, "\x1b[7m") || strings.Count(got, "\x1b[27m") != 1 {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(ansi.Cut(got, 1, 3), "\x1b[7m") {
		t.Fatalf("selection lost: %q", got)
	}
}

// renderAgent draws the panel from the state fit left, the way View does.
func (a *App) renderAgent(w, h int, focused bool) []string {
	return drawAgent(a.AgentScreen(w, h, focused), w, h, focused)
}

func TestAgentSelectionDropsQuoteBars(t *testing.T) {
	long := "quoted text that is long enough to wrap around the panel more than once so we see the bar"
	a, _ := agentSelApp(t, core.LogEntry{Kind: core.LogAgent, Text: "> " + long})
	rows, w := a.Ag.View.Rows, a.Ag.View.W
	if len(rows) < 3 {
		t.Fatalf("only %d rows", len(rows))
	}
	s := core.AgentSel{On: true, A: core.AgentPos{0, 0}, B: core.AgentPos{len(rows) - 1, w - 1}}
	if got, want := s.Text(rows, w), "▎ "+long; got != want {
		t.Fatalf("copied %q, want %q", got, want)
	}
}

func TestAgentSelectionKeepsSpacesOfWrappedCode(t *testing.T) {
	code := "x := 1" + strings.Repeat(" ", 37) + "// note\twith\ta tab and 日本語 that goes on and on past the edge"
	a, _ := agentSelApp(t, core.LogEntry{Kind: core.LogAgent, Text: "```go\n" + code + "\n```"})
	rows, w := a.Ag.View.Rows, a.Ag.View.W
	if len(rows) < 5 {
		t.Fatalf("code not wrapped: %d rows", len(rows))
	}
	// the rows of the code line, between the language label and the bottom edge
	a.Ag.Sel = core.AgentSel{On: true, A: core.AgentPos{1, 0}, B: core.AgentPos{len(rows) - 2, w - 1}}
	if got := a.Ag.Sel.Text(rows, w); got != strings.ReplaceAll(code, "\t", "    ") {
		t.Fatalf("copied\n%q\nwant\n%q", got, code)
	}
}
