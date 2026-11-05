package ctlclient

import (
	"bufio"
	"encoding/json"
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
	end := AnnouncePrimary("", filepath.Join(dir, "1.sock"))
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
	AnnouncePrimary("", "/x.sock")()
}
