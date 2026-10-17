package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorilla/websocket"

	"github.com/mm1995tk/gh-editor/internal/buffer"
	"github.com/mm1995tk/gh-editor/internal/ide"
)

// ideCall runs a tool and returns the first text block; pending replies are
// returned through the channel.
func ideCallRaw(a *App, tool string, args any) (chan ideReply, ideReply, bool) {
	b, _ := json.Marshal(args)
	ch := make(chan ideReply, 1)
	a.handleIDE(ideMsg{tool: tool, args: b, reply: ch})
	select {
	case r := <-ch:
		return ch, r, true
	default:
		return ch, ideReply{}, false
	}
}

func ideJSON(t *testing.T, a *App, tool string, args any) map[string]any {
	t.Helper()
	_, r, done := ideCallRaw(a, tool, args)
	if !done || r.err != nil {
		t.Fatalf("%s: done=%v err=%v", tool, done, r.err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(r.content[0].Text), &m); err != nil {
		t.Fatalf("%s: %v in %q", tool, err, r.content[0].Text)
	}
	return m
}

func ideApp(t *testing.T) (*App, string) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	os.WriteFile(f, []byte("one\ntwo\nthree\n"), 0o644)
	return New(dir, ""), f
}

func TestIDEOpenFileAndSelection(t *testing.T) {
	a, f := ideApp(t)
	if m := ideJSON(t, a, "getCurrentSelection", nil); m["success"] != false {
		t.Errorf("no editor: %v", m)
	}
	_, r, _ := ideCallRaw(a, "openFile", map[string]any{"filePath": "a.txt", "startText": "tw", "endText": "wo"})
	if r.err != nil || !strings.Contains(r.content[0].Text, f) {
		t.Fatalf("openFile: %+v", r)
	}
	m := ideJSON(t, a, "getCurrentSelection", nil)
	sel := m["selection"].(map[string]any)
	if m["text"] != "two" || m["filePath"] != f || m["fileUrl"] != "file://"+f ||
		sel["isEmpty"] != false || sel["start"].(map[string]any)["line"] != float64(1) {
		t.Errorf("selection = %v", m)
	}
	ed := ideJSON(t, a, "getOpenEditors", nil)["tabs"].([]any)
	if len(ed) != 1 || ed[0].(map[string]any)["isActive"] != true || ed[0].(map[string]any)["label"] != "a.txt" {
		t.Errorf("editors = %v", ed)
	}
	if _, r, _ := ideCallRaw(a, "openFile", map[string]any{"filePath": "missing.txt"}); r.err == nil {
		t.Error("missing file opened")
	}
	if _, r, _ := ideCallRaw(a, "bogus", nil); r.err == nil {
		t.Error("bogus tool accepted")
	}
}

func TestIDEDirtyAndSave(t *testing.T) {
	a, f := ideApp(t)
	a.openFile(f)
	if m := ideJSON(t, a, "checkDocumentDirty", map[string]any{"filePath": f}); m["isDirty"] != false {
		t.Errorf("clean: %v", m)
	}
	a.curText().buf.Insert("X")
	if m := ideJSON(t, a, "checkDocumentDirty", map[string]any{"filePath": f}); m["isDirty"] != true {
		t.Errorf("dirty: %v", m)
	}
	ideJSON(t, a, "saveDocument", map[string]any{"filePath": f})
	if b, _ := os.ReadFile(f); !strings.HasPrefix(string(b), "X") {
		t.Errorf("not saved: %q", b)
	}
}

func TestIDEOpenDiffAccept(t *testing.T) {
	a, f := ideApp(t)
	a.openFile(f)
	args := map[string]any{"old_file_path": f, "new_file_path": f, "new_file_contents": "one\n2\nthree\n", "tab_name": "edit"}
	ch, _, done := ideCallRaw(a, "openDiff", args)
	if done || len(a.props) != 1 {
		t.Fatalf("done=%v props=%d", done, len(a.props))
	}
	a.update(tea.KeyMsg{Type: tea.KeyEnter})
	r := <-ch
	if len(r.content) != 2 || r.content[0].Text != "FILE_SAVED" || r.content[1].Text != "one\n2\nthree\n" {
		t.Fatalf("reply = %+v", r)
	}
	if b, _ := os.ReadFile(f); string(b) != "one\ntwo\nthree\n" {
		t.Errorf("editor wrote the file; Claude Code does that: %q", b)
	}
	if len(a.props) != 0 {
		t.Error("proposal remains")
	}
}

func TestIDEOpenDiffRejectAndNewFile(t *testing.T) {
	a, f := ideApp(t)
	args := map[string]any{"old_file_path": f, "new_file_path": f, "new_file_contents": "zzz\n", "tab_name": "edit"}
	ch, _, _ := ideCallRaw(a, "openDiff", args)
	a.update(tea.KeyMsg{Type: tea.KeyEscape})
	r := <-ch
	if r.content[0].Text != "DIFF_REJECTED" || r.content[1].Text != "edit" {
		t.Fatalf("reply = %+v", r)
	}
	if b, _ := os.ReadFile(f); string(b) != "one\ntwo\nthree\n" {
		t.Errorf("rejected diff wrote the file: %q", b)
	}
}

func TestIDECloseAndCancel(t *testing.T) {
	a, f := ideApp(t)
	args := func(n string) map[string]any {
		return map[string]any{"old_file_path": f, "new_file_path": f, "new_file_contents": "x\n", "tab_name": n}
	}
	c1, _, _ := ideCallRaw(a, "openDiff", args("one"))
	ideCallRaw(a, "openDiff", args("two"))
	c3, _, _ := ideCallRaw(a, "openDiff", args("three"))
	if _, r, _ := ideCallRaw(a, "close_tab", map[string]any{"tab_name": "one"}); r.content[0].Text != "TAB_CLOSED" {
		t.Errorf("close_tab = %+v", r)
	}
	if _, r, _ := ideCallRaw(a, "close_tab", map[string]any{"tab_name": "one"}); r.content[0].Text != "TAB_CLOSED" {
		t.Errorf("close_tab after decision = %+v", r)
	}
	if r := <-c1; r.content[0].Text != "DIFF_REJECTED" {
		t.Errorf("closed tab reply = %+v", r)
	}
	if strings.Contains(a.msg, "却下しました") || !strings.Contains(a.msg, "差分を閉じました") {
		t.Errorf("close_tab status = %q", a.msg)
	}
	a.cancelProposal(c3)
	if len(a.props) != 1 || a.props[0].name != "two" {
		t.Errorf("props after cancel = %d", len(a.props))
	}
	if _, r, _ := ideCallRaw(a, "closeAllDiffTabs", nil); r.content[0].Text != "CLOSED_1_DIFF_TABS" {
		t.Errorf("closeAll = %+v", r)
	}
	if len(a.props) != 0 {
		t.Error("proposals remain")
	}
}

func TestUnifiedDiff(t *testing.T) {
	var old, cur []string
	for i := 1; i <= 20; i++ {
		old = append(old, "l"+string(rune('a'+i)))
	}
	cur = append(cur, old...)
	cur[10] = "CHANGED"
	got := unifiedDiff(strings.Join(old, "\n")+"\n", strings.Join(cur, "\n")+"\n")
	var sb strings.Builder
	for _, l := range got {
		if l.kind == '@' {
			sb.WriteString(l.text + "\n")
			continue
		}
		sb.WriteString(string(l.kind) + l.text + "\n")
	}
	want := "@@ -8,7 +8,7 @@\n li\n lj\n lk\n-ll\n+CHANGED\n lm\n ln\n lo\n"
	if sb.String() != want {
		t.Errorf("diff =\n%s\nwant\n%s", sb.String(), want)
	}
	if d := unifiedDiff("a\n", "a\n"); len(d) != 1 {
		t.Errorf("no-change diff = %v", d)
	}
	if d := unifiedDiff("", "x\ny\n"); d[0].text != "@@ -1,0 +1,2 @@" || len(d) != 3 {
		t.Errorf("new file diff = %v", d)
	}
}

func TestSelectText(t *testing.T) {
	b := buffer.New()
	b.Insert("héllo wörld\nfoo bar")
	selectText(b, "wör", "ld")
	if got := b.SelectedText(); got != "wörld" {
		t.Errorf("selected %q", got)
	}
	selectText(b, "bar", "")
	if got := b.SelectedText(); got != "bar" {
		t.Errorf("selected %q", got)
	}
}

func TestIDENotifications(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	a, f := ideApp(t)
	srv, err := ide.Start(a.root, IDETools(), func(context.Context, string, json.RawMessage) ([]ide.Content, error) { return nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	a.SetIDE(srv)
	a.openFile(f)
	if a.mentionContext() {
		t.Error("mentioned without a client")
	}
	c, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("ws://127.0.0.1:%d", srv.Port()), http.Header{ide.AuthHeader: {srv.Token()}})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"})
	c.ReadJSON(&map[string]any{})
	read := func() map[string]any {
		var m map[string]any
		c.SetReadDeadline(time.Now().Add(3 * time.Second))
		if err := c.ReadJSON(&m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	a.notifySelection()
	a.notifySelection() // unchanged: no second message
	a.curText().buf.GotoLine(2)
	a.notifySelection()
	m1, m2 := read(), read()
	if m1["method"] != "selection_changed" || m2["params"].(map[string]any)["selection"].(map[string]any)["start"].(map[string]any)["line"] != float64(1) {
		t.Errorf("notifications = %v %v", m1, m2)
	}
	if !a.mentionContext() {
		t.Fatal("not mentioned")
	}
	if m := read(); m["method"] != "at_mentioned" || m["params"].(map[string]any)["lineStart"] != float64(1) {
		t.Errorf("at_mentioned = %v", m)
	}
}
