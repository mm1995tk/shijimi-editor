package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// agentSelApp opens the panel with a log and returns the layout.
func agentSelApp(t *testing.T, log ...logEntry) (*App, layout) {
	t.Helper()
	a := newTestApp(t, "x\n")
	a.handleKey(Key{Code: 'i', Mod: ModAlt})
	a.ag.log = log
	a.render()
	return a, a.layout()
}

func agentRowY(a *App, l layout, i int) int { return l.top + a.ag.view.y0 + (i - a.ag.view.start) }

func TestAgentDragSelectCopies(t *testing.T) {
	a, l := agentSelApp(t, logEntry{kind: logInfo, text: "hello world\n日本語です"})
	y := agentRowY(a, l, 0)
	// the log starts with one column of indent
	a.HandleMouse(a.locate(l.agentX+7, y, Mouse{Action: MousePress, Button: MouseLeft}))
	a.HandleMouse(a.locate(l.agentX+4, y+1, Mouse{Action: MouseMotion, Button: MouseLeft}))
	a.HandleMouse(a.locate(l.agentX+4, y+1, Mouse{Action: MouseRelease, Button: MouseLeft}))
	if want := "world\n日本"; a.clipboard != want {
		t.Fatalf("clipboard = %q, want %q", a.clipboard, want)
	}
	if !strings.Contains(a.msg, "8 文字") || !a.ag.sel.on {
		t.Fatalf("msg = %q sel = %+v", a.msg, a.ag.sel)
	}
	drawn := strings.Join(a.renderAgent(l.agentW, l.bodyH, true), "\n")
	a.ag.sel.on = false
	plain := strings.Join(a.renderAgent(l.agentW, l.bodyH, true), "\n")
	a.ag.sel.on = true
	if drawn == plain || ansi.Strip(drawn) != ansi.Strip(plain) {
		t.Fatal("selection should change only the style of the text")
	}
	a.Update(Key{Code: 'a', Text: "a"})
	if a.ag.sel.on {
		t.Fatal("a key press should clear the selection")
	}
}

func TestAgentSelectionJoinsWrappedRows(t *testing.T) {
	long := strings.Repeat("word ", 20) + "https://example.com/" + strings.Repeat("x", 80)
	a, l := agentSelApp(t,
		logEntry{kind: logAgent, text: long},
		logEntry{kind: logInfo, text: long})
	n := len(a.ag.view.rows)
	if n < 6 {
		t.Fatalf("only %d rows", n)
	}
	a.ag.scroll = 0
	a.render()
	top := a.ag.view.start
	w := a.ag.view.w
	// from the first row to the last, all of it
	a.ag.sel = agentSel{on: true, a: agentPos{top, 0}, b: agentPos{top + n - 1, w - 1}}
	got := a.ag.sel.text(a.ag.view.rows, w)
	if want := long + "\n" + long; got != want {
		t.Fatalf("copied\n%q\nwant\n%q", got, want)
	}
	_ = l
}

func TestAgentSelectionOutsidePanel(t *testing.T) {
	a, l := agentSelApp(t, logEntry{kind: logInfo, text: "abc\ndef"})
	y := agentRowY(a, l, 0)
	a.HandleMouse(a.locate(l.agentX+1, y, Mouse{Action: MousePress, Button: MouseLeft}))
	// dragged far left of the panel and below it
	a.HandleMouse(a.locate(0, y+1, Mouse{Action: MouseMotion, Button: MouseLeft}))
	a.HandleMouse(a.locate(0, a.h-1, Mouse{Action: MouseRelease, Button: MouseLeft}))
	if a.ag.sel.dragging || a.clipboard == "" || strings.Contains(a.clipboard, "x") {
		t.Fatalf("sel = %+v clipboard = %q", a.ag.sel, a.clipboard)
	}
}

func TestAgentClickDoesNotCopy(t *testing.T) {
	a, l := agentSelApp(t, logEntry{kind: logInfo, text: "hello"})
	a.focus = FocusEditor
	a.clipboard = "keep"
	y := agentRowY(a, l, 0)
	a.HandleMouse(a.locate(l.agentX+2, y, Mouse{Action: MousePress, Button: MouseLeft}))
	a.HandleMouse(a.locate(l.agentX+2, y, Mouse{Action: MouseRelease, Button: MouseLeft}))
	if a.clipboard != "keep" || a.ag.sel.on || a.focus != FocusAgent {
		t.Fatalf("clipboard = %q sel = %+v focus = %v", a.clipboard, a.ag.sel, a.focus)
	}
}

func TestAgentSelectionStaysOnTextWhenScrolled(t *testing.T) {
	var log []logEntry
	for i := range 60 {
		log = append(log, logEntry{kind: logInfo, text: strings.Repeat("a", i%9+1)})
	}
	a, l := agentSelApp(t, log...)
	y := agentRowY(a, l, a.ag.view.start+2)
	a.HandleMouse(a.locate(l.agentX+1, y, Mouse{Action: MousePress, Button: MouseLeft}))
	a.HandleMouse(a.locate(l.agentX+3, y, Mouse{Action: MouseRelease, Button: MouseLeft}))
	before := ansi.Strip(a.ag.view.rows[a.ag.sel.a.row].plain)
	a.HandleMouse(a.locate(l.agentX+3, y, Mouse{Action: MousePress, Button: MouseWheelUp}))
	a.render()
	if !a.ag.sel.on || ansi.Strip(a.ag.view.rows[a.ag.sel.a.row].plain) != before {
		t.Fatalf("selection moved: %+v", a.ag.sel)
	}
}

func TestMarkRowKeepsColors(t *testing.T) {
	row := stStatusError.Render("abcd")
	got := MarkRow(row, 4, 1, 3)
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
	a, _ := agentSelApp(t, logEntry{kind: logAgent, text: "> " + long})
	rows, w := a.ag.view.rows, a.ag.view.w
	if len(rows) < 3 {
		t.Fatalf("only %d rows", len(rows))
	}
	s := agentSel{on: true, a: agentPos{0, 0}, b: agentPos{len(rows) - 1, w - 1}}
	if got, want := s.text(rows, w), "▎ "+long; got != want {
		t.Fatalf("copied %q, want %q", got, want)
	}
}

func TestAgentSelectionKeepsSpacesOfWrappedCode(t *testing.T) {
	code := "x := 1" + strings.Repeat(" ", 37) + "// note\twith\ta tab and 日本語 that goes on and on past the edge"
	a, _ := agentSelApp(t, logEntry{kind: logAgent, text: "```go\n" + code + "\n```"})
	rows, w := a.ag.view.rows, a.ag.view.w
	if len(rows) < 5 {
		t.Fatalf("code not wrapped: %d rows", len(rows))
	}
	// the rows of the code line, between the language label and the bottom edge
	a.ag.sel = agentSel{on: true, a: agentPos{1, 0}, b: agentPos{len(rows) - 2, w - 1}}
	if got := a.ag.sel.text(rows, w); got != strings.ReplaceAll(code, "\t", "    ") {
		t.Fatalf("copied\n%q\nwant\n%q", got, code)
	}
}
