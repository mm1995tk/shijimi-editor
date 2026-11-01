package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
	"github.com/mm1995tk/shijimi-editor/internal/config"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// formatterTimeout bounds one run of an external formatter.
const formatterTimeout = 10 * time.Second

// FormatterDoneMsg carries the output of an external formatter.
type FormatterDoneMsg struct {
	tab  *Tab
	in   string // buffer text the formatter was given
	out  string
	Err  error
	save bool
}

// runFormatter formats the current buffer with the language's formatter
// command: the buffer goes to stdin, stdout becomes the new text.
func (a *Core) runFormatter(t *Tab, f *config.Formatter, save bool) []Effect {
	in := t.Buf.Text()
	root := a.Root
	name := a.Rel(t.Buf.Path)
	args := make([]string, len(f.Args))
	for i, s := range f.Args {
		args[i] = strings.ReplaceAll(s, "%{buffer_name}", name)
	}
	cmdName := f.Command
	return async(func() Event {
		ctx, cancel := context.WithTimeout(context.Background(), formatterTimeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, cmdName, args...)
		cmd.Dir = root
		cmd.Stdin = strings.NewReader(in)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		cmd.WaitDelay = time.Second
		err := cmd.Run()
		msg := FormatterDoneMsg{tab: t, in: in, save: save}
		switch {
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			msg.Err = fmt.Errorf(i18n.T("fmt.timeout"), cmdName, int(formatterTimeout.Seconds()))
		case err != nil:
			msg.Err = errors.New(firstNonEmptyLine(stderr.String(), err.Error()))
		default:
			msg.out = stdout.String()
		}
		return msg
	})
}

// firstNonEmptyLine returns the first non-empty line of s, or def when there is none.
func firstNonEmptyLine(s, def string) string {
	for l := range strings.SplitSeq(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return def
}

// handleFormatterDone replaces only the lines the formatter changed, as one
// undo step, so the cursor and scroll stay where they were.
func (a *Core) handleFormatterDone(msg FormatterDoneMsg) []Effect {
	if msg.Err != nil {
		a.Fail(msg.Err)
		return nil
	}
	b := msg.tab.Buf
	if b.Text() != msg.in {
		a.Fail(errors.New(i18n.T("fmt.stale")))
		return nil
	}
	if edit, ok := lineDiffEdit(b, msg.out); ok {
		b.ApplyEdits([]buffer.Edit{edit})
	}
	if msg.save {
		return a.saveTab(msg.tab)
	}
	a.InfoText(i18n.T("app.formatted"))
	return nil
}

// lineDiffEdit returns one edit that turns the buffer into text by replacing
// only the lines between the common leading and trailing lines.
func lineDiffEdit(b *buffer.Buffer, text string) (buffer.Edit, bool) {
	old := b.Lines
	nw := strings.Split(text, "\n")
	n := min(len(old), len(nw))
	p := 0
	for p < n && string(old[p]) == nw[p] {
		p++
	}
	s := 0
	for s < n-p && string(old[len(old)-1-s]) == nw[len(nw)-1-s] {
		s++
	}
	if p == len(old) && p == len(nw) {
		return buffer.Edit{}, false
	}
	repl := strings.Join(nw[p:len(nw)-s], "\n")
	var start, end buffer.Pos
	switch {
	case p < len(old)-s: // some old lines are replaced
		start = buffer.Pos{Row: p}
		end = buffer.Pos{Row: len(old) - s - 1, Col: len(old[len(old)-s-1])}
		if p == len(nw)-s { // pure deletion: take a line break along
			if s > 0 {
				end = buffer.Pos{Row: len(old) - s}
			} else {
				start = buffer.Pos{Row: p - 1, Col: len(old[p-1])}
			}
		}
	case p > 0: // pure insertion after line p-1
		start = buffer.Pos{Row: p - 1, Col: len(old[p-1])}
		end = start
		repl = "\n" + repl
	default: // pure insertion at the very start
		repl += "\n"
	}
	return buffer.Edit{Start: start, End: end, Text: repl}, true
}
