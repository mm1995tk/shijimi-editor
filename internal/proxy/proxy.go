// Package proxy forwards TCP connections from a local listening port to a
// port on localhost. Being plain TCP, it carries HTTP and WebSocket alike.
package proxy

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// DefaultListenHost is where forwards listen unless configured otherwise.
const DefaultListenHost = "127.0.0.1"

const dialTimeout = 5 * time.Second

// Forward is one running forward.
type Forward struct {
	Target int    // port on localhost that connections are relayed to
	Listen string // actual listening address, e.g. "127.0.0.1:8080"
	Port   int    // actual listening port

	ln     net.Listener
	mu     sync.Mutex
	closed bool
	conns  map[net.Conn]struct{}
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
		return nil, errors.New(i18n.T("proxy.portrange"))
	}
	if listenPort == target {
		// The forward would connect to itself.
		return nil, fmt.Errorf(i18n.T("proxy.sameport"), target)
	}
	if host == "" {
		host = DefaultListenHost
	}
	var ln net.Listener
	var port int
	for try := 0; ; try++ {
		l, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(listenPort)))
		if err != nil {
			return nil, err
		}
		ln, port = l, l.Addr().(*net.TCPAddr).Port
		if port != target {
			break
		}
		// The OS picked the target port; pick again.
		l.Close()
		if try >= 5 {
			return nil, errors.New(i18n.T("proxy.nofree"))
		}
	}
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
		return fmt.Errorf(i18n.T("proxy.notforwarded"), port)
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
	f.closed = true
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
	if !add {
		delete(f.conns, c)
		return true
	}
	if f.closed {
		return false
	}
	f.conns[c] = struct{}{}
	return true
}

// refuseSelf stops a dial from reaching this forward's own listener, which
// would relay to itself forever (Start rejects Target == Port, but localhost
// resolution must not loop back either).
func (f *Forward) refuseSelf(_, address string, _ syscall.RawConn) error {
	if _, p, err := net.SplitHostPort(address); err == nil && p == strconv.Itoa(f.Port) {
		return fmt.Errorf("self connection refused")
	}
	return nil
}

func (f *Forward) relay(src net.Conn) {
	defer src.Close()
	if !f.track(src, true) {
		return
	}
	defer f.track(src, false)
	// "localhost" lets the dialer try both 127.0.0.1 and ::1, so servers that
	// listen on IPv6 only (or on 0.0.0.0) are reachable.
	d := net.Dialer{Timeout: dialTimeout, Control: f.refuseSelf}
	dst, err := d.Dial("tcp", net.JoinHostPort("localhost", strconv.Itoa(f.Target)))
	if err != nil {
		return
	}
	defer dst.Close()
	if !f.track(dst, true) {
		return
	}
	defer f.track(dst, false)

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
