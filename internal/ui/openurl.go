package ui

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
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
				return fmt.Errorf(i18n.T("open.failed"), name, msg)
			}
			return fmt.Errorf(i18n.T("open.failedv"), name, err)
		}
		return nil
	}
	if len(openers[runtime.GOOS]) == 0 {
		return fmt.Errorf(i18n.T("open.unsupportedos"), runtime.GOOS)
	}
	return fmt.Errorf(i18n.T("open.noopener"), strings.Join(openers[runtime.GOOS], " / "))
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
			a.fail(errors.New(i18n.T("open.needurl")))
			return nil
		}
		cur := t.buf.Cur
		u, ok := urlAt(t.buf.Lines[cur.Row], cur.Col)
		if !ok {
			a.fail(errors.New(i18n.T("open.nourl")))
			return nil
		}
		url = u
	}
	if !urlFullRe.MatchString(url) {
		a.fail(fmt.Errorf(i18n.T("open.badurl"), url))
		return nil
	}
	return func() tea.Msg { return openURLMsg{url: url, err: openExternal(url)} }
}
