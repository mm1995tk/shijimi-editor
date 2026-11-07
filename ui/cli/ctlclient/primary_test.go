package ctlclient

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mm1995tk/shijimi-editor/core/ctl"
)

// 環境変数があれば名乗って（つなぎっぱなし）、変数は消す。end で切れる。
func TestAnnouncePrimary(t *testing.T) {
	dir, err := os.MkdirTemp("", "shjp")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	t.Setenv("XDG_RUNTIME_DIR", dir)
	path, err := ctl.ServePrimarySocketPath("")
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	got := make(chan ctl.PrimaryHello, 1)
	closed := make(chan struct{})
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		br := bufio.NewReader(c)
		var h ctl.PrimaryHello
		json.NewDecoder(br).Decode(&h)
		got <- h
		json.NewEncoder(c).Encode(ctl.PrimaryReply{OK: true})
		br.ReadByte() // 切れるまで
		close(closed)
	}()

	t.Setenv(ctl.EnvGUIPrimary, "t7")
	end, primary, _, _, _ := AnnouncePrimary("", filepath.Join(dir, "1.sock"))
	if !primary {
		t.Error("an accepted announcement is a primary")
	}
	if os.Getenv(ctl.EnvGUIPrimary) != "" {
		t.Error("the variable must be removed")
	}
	select {
	case h := <-got:
		if h.TermID != "t7" || h.Socket != filepath.Join(dir, "1.sock") || h.PID != os.Getpid() {
			t.Errorf("hello: %+v", h)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no hello")
	}
	end()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("the connection must close at the end")
	}
}

// 環境変数が無ければ何もしない（つなぎにも行かない）。
func TestAnnouncePrimaryWithoutEnv(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv(ctl.EnvGUIPrimary, "")
	end, primary, report, _, _ := AnnouncePrimary("", "/x.sock")
	report(ctl.PrimaryFile{File: "/ignored"})
	end()
	if primary {
		t.Error("without the variable there is no primary")
	}
}

// Pane.Do sends one ctl.PaneRequest with the shj's termId and pid on a connection of its own.
func TestPaneDo(t *testing.T) {
	dir, err := os.MkdirTemp("", "shjp")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	path := filepath.Join(dir, ctl.ServePrimaryName)
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	got := make(chan ctl.PaneRequest, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		var rq ctl.PaneRequest
		json.NewDecoder(bufio.NewReader(c)).Decode(&rq)
		got <- rq
		json.NewEncoder(c).Encode(ctl.PaneReply{OK: rq.Op != "bad", Error: "no", ID: "t5"})
	}()
	p := &Pane{path: path, termID: "t7", pid: 42}
	r, err := p.Do(ctl.PaneRequest{Op: "split", Cwd: "/x"})
	if rq := <-got; rq.TermID != "t7" || rq.PID != 42 || rq.Op != "split" || rq.Cwd != "/x" {
		t.Errorf("request: %+v", rq)
	}
	if err != nil || r.ID != "t5" {
		t.Errorf("reply: %+v %v", r, err)
	}
}

// kisui が版の範囲を理由に断ったら、VersionError で範囲を返す。名乗りには自分の版を載せる。
func TestAnnouncePrimaryVersionRefused(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)
	path, err := ctl.ServePrimarySocketPath("")
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	got := make(chan ctl.PrimaryHello, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		var h ctl.PrimaryHello
		json.NewDecoder(bufio.NewReader(c)).Decode(&h)
		got <- h
		json.NewEncoder(c).Encode(ctl.PrimaryReply{Error: "x", Code: ctl.ReplyCodeVersion, Min: 3, Max: 4})
	}()
	t.Setenv(ctl.EnvGUIPrimary, "t7")
	_, primary, _, _, err := AnnouncePrimary("", filepath.Join(dir, "1.sock"))
	if primary {
		t.Error("a refused announcement is not a primary")
	}
	var ve *VersionError
	if !errors.As(err, &ve) || ve.Min != 3 || ve.Max != 4 {
		t.Errorf("err = %v", err)
	}
	if h := <-got; h.Protocol != ctl.PrimaryProtocol {
		t.Errorf("hello: %+v", h)
	}
}

// FocusExplorer sends op "explorer"; a refusal (a kisui from before it) is an error.
func TestPaneFocusExplorer(t *testing.T) {
	dir, err := os.MkdirTemp("", "shjp")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	path := filepath.Join(dir, ctl.ServePrimaryName)
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	got := make(chan ctl.PaneRequest, 2)
	go func() {
		for i := range 2 {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			var rq ctl.PaneRequest
			json.NewDecoder(bufio.NewReader(c)).Decode(&rq)
			got <- rq
			json.NewEncoder(c).Encode(ctl.PaneReply{OK: i == 0, Error: `unknown op: "explorer"`})
			c.Close()
		}
	}()
	p := &Pane{path: path, termID: "t7", pid: 42}
	if err := p.FocusExplorer(); err != nil {
		t.Errorf("accepted: %v", err)
	}
	if rq := <-got; rq.Op != "explorer" || rq.TermID != "t7" || rq.PID != 42 {
		t.Errorf("request: %+v", rq)
	}
	if err := p.FocusExplorer(); err == nil {
		t.Error("a refusal must be an error")
	}
}
