package ctlserver

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mm1995tk/shijimi-editor/core/ctl"
	"github.com/mm1995tk/shijimi-editor/ui/cli/ctlclient"
)

func TestServerRoundTrip(t *testing.T) {
	dir, err := os.MkdirTemp("", "ghe")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	sock := filepath.Join(dir, "s.sock")
	srv, err := Listen(sock, func(r ctl.Request) ctl.Response {
		if r.Cmd == "diff" {
			return ctl.Response{OK: true}
		}
		return ctl.Response{Error: "bad " + r.Cmd}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	st, _ := os.Stat(sock)
	if st.Mode().Perm() != 0o600 {
		t.Errorf("socket mode = %v, want 0600", st.Mode().Perm())
	}
	t.Setenv(ctl.EnvSocket, sock)
	resp, err := ctlclient.Call(ctl.Request{Cmd: "diff"})
	if err != nil || !resp.OK {
		t.Fatalf("diff: %+v %v", resp, err)
	}
	resp, _ = ctlclient.Call(ctl.Request{Cmd: "x"})
	if resp.OK || resp.Error != "bad x" {
		t.Fatalf("error: %+v", resp)
	}
}
