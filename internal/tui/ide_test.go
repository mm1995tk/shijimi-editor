package tui

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

	"github.com/gorilla/websocket"

	"github.com/mm1995tk/shijimi-editor/internal/core"
	"github.com/mm1995tk/shijimi-editor/internal/ide"
)

// ideCall runs a tool and returns the first text block; pending replies are
// returned through the channel.
func ideCallRaw(a *App, tool string, args any) (chan IDEReply, IDEReply, bool) {
	b, _ := json.Marshal(args)
	ch := make(chan IDEReply, 1)
	for _, e := range a.handleIDE(IDECall{Tool: tool, Args: b, Reply: ch}) {
		if r, ok := e.(ReplyIDE); ok {
			r.To <- r.Reply
		}
	}
	select {
	case r := <-ch:
		return ch, r, true
	default:
		return ch, IDEReply{}, false
	}
}

func ideJSON(t *testing.T, a *App, tool string, args any) map[string]any {
	t.Helper()
	_, r, done := ideCallRaw(a, tool, args)
	if !done || r.Err != nil {
		t.Fatalf("%s: done=%v err=%v", tool, done, r.Err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(r.Content[0].Text), &m); err != nil {
		t.Fatalf("%s: %v in %q", tool, err, r.Content[0].Text)
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
	if r.Err != nil || !strings.Contains(r.Content[0].Text, f) {
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
	if _, r, _ := ideCallRaw(a, "openFile", map[string]any{"filePath": "missing.txt"}); r.Err == nil {
		t.Error("missing file opened")
	}
	if _, r, _ := ideCallRaw(a, "bogus", nil); r.Err == nil {
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
	if done || a.props.Len() != 1 {
		t.Fatalf("done=%v props=%d", done, a.props.Len())
	}
	a.update(Key{Code: KeyEnter})
	r := <-ch
	if len(r.Content) != 2 || r.Content[0].Text != "FILE_SAVED" || r.Content[1].Text != "one\n2\nthree\n" {
		t.Fatalf("reply = %+v", r)
	}
	if b, _ := os.ReadFile(f); string(b) != "one\ntwo\nthree\n" {
		t.Errorf("editor wrote the file; Claude Code does that: %q", b)
	}
	if a.props.Len() != 0 {
		t.Error("proposal remains")
	}
}

func TestIDEOpenDiffRejectAndNewFile(t *testing.T) {
	a, f := ideApp(t)
	args := map[string]any{"old_file_path": f, "new_file_path": f, "new_file_contents": "zzz\n", "tab_name": "edit"}
	ch, _, _ := ideCallRaw(a, "openDiff", args)
	a.update(Key{Code: KeyEscape})
	r := <-ch
	if r.Content[0].Text != "DIFF_REJECTED" || r.Content[1].Text != "edit" {
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
	if _, r, _ := ideCallRaw(a, "close_tab", map[string]any{"tab_name": "one"}); r.Content[0].Text != "TAB_CLOSED" {
		t.Errorf("close_tab = %+v", r)
	}
	if _, r, _ := ideCallRaw(a, "close_tab", map[string]any{"tab_name": "one"}); r.Content[0].Text != "TAB_CLOSED" {
		t.Errorf("close_tab after decision = %+v", r)
	}
	if r := <-c1; r.Content[0].Text != "DIFF_REJECTED" {
		t.Errorf("closed tab reply = %+v", r)
	}
	if strings.Contains(a.msg, "却下しました") || !strings.Contains(a.msg, "差分を閉じました") {
		t.Errorf("close_tab status = %q", a.msg)
	}
	a.cancelProposal(c3)
	if a.props.Len() != 1 || a.props.First().Name != "two" {
		t.Errorf("props after cancel = %d", a.props.Len())
	}
	if _, r, _ := ideCallRaw(a, "closeAllDiffTabs", nil); r.Content[0].Text != "CLOSED_1_DIFF_TABS" {
		t.Errorf("closeAll = %+v", r)
	}
	if a.props.Len() != 0 {
		t.Error("proposals remain")
	}
}

func TestIDENotifications(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	a, f := ideApp(t)
	srv, err := ide.Start(a.root, core.IDETools(), func(context.Context, string, json.RawMessage) ([]ide.Content, error) { return nil, nil })
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
