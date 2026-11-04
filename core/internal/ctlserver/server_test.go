package ctlserver

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/mm1995tk/shijimi-editor/core/ctl"
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
	call := func(r ctl.Request) (resp ctl.Response, err error) {
		c, err := net.Dial("unix", sock)
		if err != nil {
			return resp, err
		}
		defer c.Close()
		if err := json.NewEncoder(c).Encode(r); err != nil {
			return resp, err
		}
		err = json.NewDecoder(c).Decode(&resp)
		return resp, err
	}
	resp, err := call(ctl.Request{Cmd: "diff"})
	if err != nil || !resp.OK {
		t.Fatalf("diff: %+v %v", resp, err)
	}
	resp, _ = call(ctl.Request{Cmd: "x"})
	if resp.OK || resp.Error != "bad x" {
		t.Fatalf("error: %+v", resp)
	}
}
