package ui

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

var urlRe = regexp.MustCompile(`https?://[^\s<>"'` + "`" + `]+`)

// openers lists the commands that open a URL in the default browser, per OS.
var openers = map[string][]string{
	"linux":  {"xdg-open"},
	"darwin": {"open"},
}

// openExternal runs the opener and waits for it, so a non-zero exit is
// reported. It is replaced in tests.
var openExternal = func(url string) error {
	for _, name := range openers[runtime.GOOS] {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		var stderr bytes.Buffer
		cmd := exec.Command(path, url)
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			if msg := strings.TrimSpace(stderr.String()); msg != "" {
				return fmt.Errorf("%s が失敗しました: %s", name, msg)
			}
			return fmt.Errorf("%s が失敗しました: %v", name, err)
		}
		return nil
	}
	if len(openers[runtime.GOOS]) == 0 {
		return fmt.Errorf("この OS には対応していません: %s", runtime.GOOS)
	}
	return fmt.Errorf("URL を開くコマンドが見つかりません（%s をインストールしてください）", strings.Join(openers[runtime.GOOS], " / "))
}

// urlAt returns the URL in line that covers column col (a rune index).
func urlAt(line []rune, col int) (string, bool) {
	s := string(line)
	for _, m := range urlRe.FindAllStringIndex(s, -1) {
		start := len([]rune(s[:m[0]]))
		u := strings.TrimRight(s[m[0]:m[1]], ".,;:!?)]}")
		if col >= start && col < start+len([]rune(u)) {
			return u, true
		}
	}
	return "", false
}

// openURLMsg reports the result of running the opener.
type openURLMsg struct {
	url string
	err error
}

var urlFullRe = regexp.MustCompile(`^https?://\S+$`)

// openURL opens arg, or the URL under the cursor when arg is empty.
func (a *App) openURL(arg string) tea.Cmd {
	url := strings.TrimSpace(arg)
	if url == "" {
		t := a.curText()
		if t == nil {
			a.fail(errors.New("URL を指定してください"))
			return nil
		}
		cur := t.buf.Cur
		u, ok := urlAt(t.buf.Lines[cur.Row], cur.Col)
		if !ok {
			a.fail(errors.New("カーソル位置に URL がありません（:open-url <URL> で指定できます）"))
			return nil
		}
		url = u
	}
	if !urlFullRe.MatchString(url) {
		a.fail(fmt.Errorf("http:// または https:// で始まる URL を 1 つだけ指定してください: %s", url))
		return nil
	}
	return func() tea.Msg { return openURLMsg{url: url, err: openExternal(url)} }
}
