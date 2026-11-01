package core

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

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
var OpenExternal = func(url string) error {
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
	// copyOnFail copies url to the clipboard when opening failed, instead of
	// showing the error.
	copyOnFail bool
}

var urlFullRe = regexp.MustCompile(`^https?://\S+$`)

// openURL opens arg, or the URL under the cursor when arg is empty.
func (a *Core) openURL(arg string) []Effect {
	url := strings.TrimSpace(arg)
	if url == "" {
		t := a.CurText()
		if t == nil {
			a.Fail(errors.New(i18n.T("open.needurl")))
			return nil
		}
		cur := t.Buf.Cur
		u, ok := urlAt(t.Buf.Lines[cur.Row], cur.Col)
		if !ok {
			a.Fail(errors.New(i18n.T("open.nourl")))
			return nil
		}
		url = u
	}
	if !urlFullRe.MatchString(url) {
		a.Fail(fmt.Errorf(i18n.T("open.badurl"), url))
		return nil
	}
	return async(func() Event { return openURLMsg{url: url, err: a.openInBrowser(url)} })
}

// isRemoteSession reports whether the editor runs over SSH, where a browser
// opened here would not appear on the user's screen. It is replaced in tests.
var IsRemoteSession = func() bool {
	return os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != ""
}

// copyPRURL copies the URL of the pull request being reviewed.
func (a *Core) copyPRURL() []Effect {
	a.CopyText(a.Gd.Pr.URL)
	a.Info(i18n.T("rv.urlcopied"), a.Gd.Pr.URL)
	return nil
}

// openPRURL opens the pull request in the browser. Over SSH, or when the
// opener fails, it copies the URL instead and says so.
func (a *Core) openPRURL() []Effect {
	url := a.Gd.Pr.URL
	if a.host == nil && IsRemoteSession() {
		a.CopyText(url)
		a.Info(i18n.T("rv.sshcopied"), url)
		return nil
	}
	return async(func() Event { return openURLMsg{url: url, err: a.openInBrowser(url), copyOnFail: true} })
}
