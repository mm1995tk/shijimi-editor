package core

import (
	"net"
	"strconv"
	"strings"
	"testing"
)

func TestBrowserForwardListStop(t *testing.T) {
	a := newApp(t.TempDir(), "")
	defer a.proxy.StopAll()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	target := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)

	a.browser("forward " + target)
	if a.MsgErr || len(a.proxy.List()) != 1 {
		t.Fatalf("forward failed: %q", a.Msg)
	}
	f := a.proxy.List()[0]
	if !strings.HasPrefix(f.Listen, "127.0.0.1:") {
		t.Fatalf("default listen = %s", f.Listen)
	}
	a.browser("list")
	if !strings.Contains(a.Msg, target) {
		t.Fatalf("list msg = %q", a.Msg)
	}
	a.browser("stop " + strconv.Itoa(f.Port))
	if a.MsgErr || len(a.proxy.List()) != 0 {
		t.Fatalf("stop failed: %q", a.Msg)
	}
	a.browser("forward abc")
	if !a.MsgErr {
		t.Fatal("bad port should fail")
	}
}
