package ui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

const maxTermLines = 5000

type termLine struct {
	text string
	kind int // 0 = output, 1 = command echo, 2 = info
}

type termOutputMsg struct {
	id   int
	line string
}

type termDoneMsg struct {
	id  int
	err error
}

// terminal is the bottom panel. It runs one shell command at a time and
// streams its output (it is not a full terminal emulator).
type terminal struct {
	lines   []termLine
	input   textinput.Model
	cwd     string
	scroll  int // lines scrolled up from the bottom
	running bool
	runID   int
	cancel  context.CancelFunc
	ch      chan tea.Msg
	history []string
	histIdx int
}

func newTerminal(cwd string) *terminal {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "コマンドを入力して Enter"
	t := &terminal{input: ti, cwd: cwd}
	t.info("シェル: " + shell() + "   (Ctrl+C: 実行中のコマンドを停止 / clear: 表示を消す)")
	return t
}

func shell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	return "/bin/sh"
}

func (t *terminal) add(l termLine) {
	t.lines = append(t.lines, l)
	if len(t.lines) > maxTermLines {
		t.lines = t.lines[len(t.lines)-maxTermLines:]
	}
}

func (t *terminal) info(s string) { t.add(termLine{text: s, kind: 2}) }

func (t *terminal) focus() tea.Cmd { return t.input.Focus() }
func (t *terminal) blur()          { t.input.Blur() }

func (t *terminal) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case termOutputMsg:
		if msg.id != t.runID {
			return nil
		}
		t.add(termLine{text: msg.line})
		return t.wait()
	case termDoneMsg:
		if msg.id != t.runID {
			return nil
		}
		t.running = false
		t.cancel = nil
		if msg.err != nil {
			t.info("[終了] " + msg.err.Error())
		}
		return nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			if t.running && t.cancel != nil {
				t.cancel()
				t.info("^C")
			}
			return nil
		case "enter":
			return t.run(t.input.Value())
		case "up":
			if len(t.history) > 0 {
				t.histIdx = max(0, t.histIdx-1)
				t.input.SetValue(t.history[t.histIdx])
				t.input.CursorEnd()
			}
			return nil
		case "down":
			if t.histIdx < len(t.history)-1 {
				t.histIdx++
				t.input.SetValue(t.history[t.histIdx])
			} else {
				t.histIdx = len(t.history)
				t.input.SetValue("")
			}
			t.input.CursorEnd()
			return nil
		case "pgup", "shift+up":
			t.scroll = min(t.scroll+5, max(0, len(t.lines)-1))
			return nil
		case "pgdown", "shift+down":
			t.scroll = max(0, t.scroll-5)
			return nil
		}
	}
	var cmd tea.Cmd
	t.input, cmd = t.input.Update(msg)
	return cmd
}

func (t *terminal) run(line string) tea.Cmd {
	line = strings.TrimSpace(line)
	t.input.SetValue("")
	t.scroll = 0
	if line == "" {
		return nil
	}
	if len(t.history) == 0 || t.history[len(t.history)-1] != line {
		t.history = append(t.history, line)
	}
	t.histIdx = len(t.history)
	t.add(termLine{text: t.promptText() + line, kind: 1})

	if line == "clear" {
		t.lines = nil
		return nil
	}
	if line == "cd" || strings.HasPrefix(line, "cd ") {
		dir := strings.TrimSpace(strings.TrimPrefix(line, "cd"))
		if dir == "" || dir == "~" {
			dir, _ = os.UserHomeDir()
		} else if strings.HasPrefix(dir, "~/") {
			home, _ := os.UserHomeDir()
			dir = filepath.Join(home, dir[2:])
		} else if !filepath.IsAbs(dir) {
			dir = filepath.Join(t.cwd, dir)
		}
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			t.info("cd: ディレクトリがありません: " + dir)
		} else {
			t.cwd = filepath.Clean(dir)
		}
		return nil
	}
	if t.running {
		t.info("前のコマンドが実行中です (Ctrl+C で停止)")
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, shell(), "-c", line)
	cmd.Dir = t.cwd
	cmd.Env = append(os.Environ(), "TERM=dumb", "NO_COLOR=1", "PAGER=cat", "GIT_PAGER=cat")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = time.Second
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		cancel()
		t.info(err.Error())
		return nil
	}
	t.runID++
	id := t.runID
	t.running = true
	t.cancel = cancel
	ch := make(chan tea.Msg, 256)
	t.ch = ch
	errCh := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		cancel()
		pw.Close()
		errCh <- err
	}()
	go func() {
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			ch <- termOutputMsg{id: id, line: cleanOutput(sc.Text())}
		}
		io.Copy(io.Discard, pr)
		ch <- termDoneMsg{id: id, err: <-errCh}
	}()
	return t.wait()
}

func (t *terminal) wait() tea.Cmd {
	ch := t.ch
	return func() tea.Msg { return <-ch }
}

func cleanOutput(s string) string {
	s = ansi.Strip(s)
	if i := strings.LastIndexByte(s, '\r'); i >= 0 {
		s = s[i+1:]
	}
	return strings.ReplaceAll(s, "\t", "    ")
}

func (t *terminal) promptText() string {
	dir := filepath.Base(t.cwd)
	if home, err := os.UserHomeDir(); err == nil && t.cwd == home {
		dir = "~"
	}
	return dir + " $ "
}

func (t *terminal) render(w, h int, focused bool) []string {
	out := make([]string, 0, h)
	title := stPanelBord.Render(strings.Repeat("─", w))
	out = append(out, title)
	state := ""
	if t.running {
		state = "  ● 実行中"
	}
	out = append(out, stPanelTitle.Render(" ")+stPanelTab.Render("TERMINAL")+stPanelDim.Render(fit(state, w-9)))
	body := h - 3
	if body < 0 {
		return out[:h]
	}
	end := len(t.lines) - t.scroll
	start := max(0, end-body)
	for i := 0; i < body; i++ {
		idx := start + i
		if idx >= end || idx < 0 {
			out = append(out, stPanel.Render(fit("", w)))
			continue
		}
		l := t.lines[idx]
		st := stPanel
		switch l.kind {
		case 1:
			st = stPanelCmd
		case 2:
			st = stPanelDim
		}
		out = append(out, st.Render(fit(" "+l.text, w)))
	}
	prompt := t.promptText()
	t.input.Width = max(1, w-len([]rune(prompt))-3)
	in := t.input.View()
	if !focused {
		in = t.input.Value()
	}
	out = append(out, stPanelCmd.Render(" "+prompt)+fitStyled(in, w-1-ansi.StringWidth(prompt), stPanel))
	return out[:h]
}

// fitStyled fits s into w cells and paints the padding with st.
func fitStyled(s string, w int, st interface{ Render(...string) string }) string {
	sw := ansi.StringWidth(s)
	if sw > w {
		return ansi.Truncate(s, w, "")
	}
	return st.Render(s) + st.Render(fmt.Sprintf("%*s", w-sw, ""))
}
