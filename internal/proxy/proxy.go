// Package proxy forwards TCP connections from a local listening port to a
// port on localhost. Being plain TCP, it carries HTTP and WebSocket alike.
package proxy

import (
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"
)

// DefaultListenHost is where forwards listen unless configured otherwise.
const DefaultListenHost = "127.0.0.1"

const dialTimeout = 5 * time.Second

// Forward is one running forward.
type Forward struct {
	Target int    // port on localhost that connections are relayed to
	Listen string // actual listening address, e.g. "127.0.0.1:8080"
	Port   int    // actual listening port

	ln    net.Listener
	mu    sync.Mutex
	conns map[net.Conn]struct{}
}

// Manager owns the running forwards.
type Manager struct {
	mu       sync.Mutex
	forwards map[int]*Forward // by listening port
}

func NewManager() *Manager {
	return &Manager{forwards: map[int]*Forward{}}
}

// Start listens on host:listenPort (0 = pick a free port) and relays each
// connection to localhost:target. An empty host means DefaultListenHost.
func (m *Manager) Start(host string, target, listenPort int) (*Forward, error) {
	if target < 1 || target > 65535 || listenPort < 0 || listenPort > 65535 {
		return nil, fmt.Errorf("ポート番号は 1〜65535 で指定してください")
	}
	if host == "" {
		host = DefaultListenHost
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(listenPort)))
	if err != nil {
		return nil, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	f := &Forward{Target: target, Listen: ln.Addr().String(), Port: port, ln: ln, conns: map[net.Conn]struct{}{}}

	m.mu.Lock()
	m.forwards[port] = f
	m.mu.Unlock()

	go f.serve()
	return f, nil
}

// Stop closes the forward listening on port.
func (m *Manager) Stop(port int) error {
	m.mu.Lock()
	f, ok := m.forwards[port]
	delete(m.forwards, port)
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("待ち受けポート %d の転送はありません", port)
	}
	f.close()
	return nil
}

// StopAll closes every forward.
func (m *Manager) StopAll() {
	m.mu.Lock()
	all := m.forwards
	m.forwards = map[int]*Forward{}
	m.mu.Unlock()
	for _, f := range all {
		f.close()
	}
}

// List returns the running forwards ordered by listening port.
func (m *Manager) List() []*Forward {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Forward, 0, len(m.forwards))
	for _, f := range m.forwards {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Port < out[j].Port })
	return out
}

func (f *Forward) close() {
	f.ln.Close()
	f.mu.Lock()
	for c := range f.conns {
		c.Close()
	}
	f.mu.Unlock()
}

func (f *Forward) serve() {
	for {
		c, err := f.ln.Accept()
		if err != nil {
			return
		}
		go f.relay(c)
	}
}

func (f *Forward) track(c net.Conn, add bool) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if add {
		f.conns[c] = struct{}{}
	} else {
		delete(f.conns, c)
	}
	return true
}

func (f *Forward) relay(src net.Conn) {
	f.track(src, true)
	defer f.track(src, false)
	defer src.Close()
	// "localhost" lets the dialer try both 127.0.0.1 and ::1, so servers that
	// listen on IPv6 only (or on 0.0.0.0) are reachable.
	dst, err := net.DialTimeout("tcp", net.JoinHostPort("localhost", strconv.Itoa(f.Target)), dialTimeout)
	if err != nil {
		return
	}
	f.track(dst, true)
	defer f.track(dst, false)
	defer dst.Close()

	done := make(chan struct{}, 2)
	pipe := func(a, b net.Conn) {
		io.Copy(a, b)
		if t, ok := a.(*net.TCPConn); ok {
			t.CloseWrite()
		}
		done <- struct{}{}
	}
	go pipe(dst, src)
	go pipe(src, dst)
	<-done
	<-done
}
