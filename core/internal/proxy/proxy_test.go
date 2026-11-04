package proxy

import (
	"io"
	"net"
	"testing"
	"time"
)

func echoServer(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); io.Copy(c, c) }()
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestForwardRelaysAndStops(t *testing.T) {
	target := echoServer(t)
	m := NewManager()
	defer m.StopAll()

	f, err := m.Start("", target, 0)
	if err != nil {
		t.Fatal(err)
	}
	if f.Listen[:len(DefaultListenHost)] != DefaultListenHost {
		t.Fatalf("listen = %s, want %s", f.Listen, DefaultListenHost)
	}
	c, err := net.Dial("tcp", f.Listen)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.Write([]byte("ping"))
	buf := make([]byte, 4)
	c.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.ReadFull(c, buf); err != nil || string(buf) != "ping" {
		t.Fatalf("got %q, %v", buf, err)
	}

	if got := m.List(); len(got) != 1 || got[0].Port != f.Port {
		t.Fatalf("List = %v", got)
	}
	if err := m.Stop(f.Port); err != nil {
		t.Fatal(err)
	}
	if len(m.List()) != 0 {
		t.Fatal("forward still listed after Stop")
	}
	if err := m.Stop(f.Port); err == nil {
		t.Fatal("second Stop should fail")
	}
	if _, err := net.DialTimeout("tcp", f.Listen, 200*time.Millisecond); err == nil {
		t.Fatal("listener still open after Stop")
	}
}

func TestStartRejectsBadPorts(t *testing.T) {
	m := NewManager()
	if _, err := m.Start("", 0, 0); err == nil {
		t.Fatal("target 0 should fail")
	}
	if _, err := m.Start("", 80, 70000); err == nil {
		t.Fatal("listen 70000 should fail")
	}
}

func TestStartRejectsSelfLoop(t *testing.T) {
	m := NewManager()
	defer m.StopAll()
	if _, err := m.Start("", 3000, 3000); err == nil {
		t.Fatal("listen == target should fail")
	}
	for range 20 {
		f, err := m.Start("", 3000, 0)
		if err != nil {
			t.Fatal(err)
		}
		if f.Port == 3000 {
			t.Fatal("OS-picked port equals target")
		}
	}
}

func TestRefuseSelf(t *testing.T) {
	f := &Forward{Port: 4000}
	if err := f.refuseSelf("tcp", "127.0.0.1:4000", nil); err == nil {
		t.Fatal("dial to own port should be refused")
	}
	if err := f.refuseSelf("tcp", "127.0.0.1:4001", nil); err != nil {
		t.Fatal(err)
	}
}

func TestNoRelayAfterClose(t *testing.T) {
	f := &Forward{conns: map[net.Conn]struct{}{}, closed: true}
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	if f.track(a, true) {
		t.Fatal("track must refuse after close")
	}
}
