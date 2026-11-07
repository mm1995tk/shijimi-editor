package ctlclient

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/mm1995tk/shijimi-editor/core/ctl"
)

// AnnouncePrimary tells "kisui serve" that this shj, whose control socket is
// ownSocket, runs in the primary terminal of a GUI workspace. It does nothing
// unless ctl.EnvGUIPrimary is set, and it removes the variable either way so
// that a shj started from this one (or from a terminal panel of it) is never
// taken for the primary. It is best effort: the editor works the same when the
// server is gone or refuses. The returned function ends the announcement; the
// server also sees it end when this process does. primary is whether the server
// accepted it as the primary shj: only then does the GUI show the file tree, so
// only then does this shj leave its own out.
//
// report tells the server which file this shj shows (ctl.PrimaryFile); it does
// nothing when primary is false. It does not wait for the server. pane is how
// to use the GUI's panes; it is nil when primary is false.
func AnnouncePrimary(suffix, ownSocket string) (end func(), primary bool, report func(file string), pane *Pane, err error) {
	termID := os.Getenv(ctl.EnvGUIPrimary)
	if termID == "" {
		return func() {}, false, func(string) {}, nil, nil
	}
	os.Unsetenv(ctl.EnvGUIPrimary)
	path, err := ctl.ServePrimarySocketPath(suffix)
	if err != nil {
		return func() {}, false, func(string) {}, nil, nil
	}
	return announce(path, ctl.PrimaryHello{TermID: termID, Socket: ownSocket, PID: os.Getpid(), Protocol: ctl.PrimaryProtocol})
}

func announce(path string, h ctl.PrimaryHello) (func(), bool, func(string), *Pane, error) {
	c, err := net.DialTimeout("unix", path, 2*time.Second)
	if err != nil {
		return func() {}, false, func(string) {}, nil, nil
	}
	_ = c.SetDeadline(time.Now().Add(2 * time.Second))
	line, _ := json.Marshal(h)
	if _, err := c.Write(append(line, '\n')); err != nil {
		c.Close()
		return func() {}, false, func(string) {}, nil, nil
	}
	var r ctl.PrimaryReply
	if err := json.NewDecoder(bufio.NewReader(c)).Decode(&r); err != nil || !r.OK {
		c.Close()
		if err == nil && r.Code == ctl.ReplyCodeVersion {
			return func() {}, false, func(string) {}, nil, &VersionError{Min: r.Min, Max: r.Max}
		}
		return func() {}, false, func(string) {}, nil, nil
	}
	_ = c.SetDeadline(time.Time{})
	// report never waits for the server: the newest file waits in a channel of
	// one place and a goroutine writes it, so a slow server costs nothing and
	// the last file always arrives.
	latest := make(chan string, 1)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case file := <-latest:
				b, _ := json.Marshal(ctl.PrimaryFile{File: file})
				_ = c.SetWriteDeadline(time.Now().Add(time.Second))
				_, _ = c.Write(append(b, '\n'))
			}
		}
	}()
	report := func(file string) {
		for {
			select {
			case latest <- file:
				return
			default:
			}
			select {
			case <-latest: // drop the older one
			default:
			}
		}
	}
	var once sync.Once
	return func() { once.Do(func() { close(done); c.Close() }) }, true, report, &Pane{path: path, termID: h.TermID, pid: h.PID}, nil
}

// VersionError says "kisui serve" refused the announcement because this shj's
// protocol version (ctl.PrimaryProtocol) is outside the range Min to Max that
// it accepts.
type VersionError struct{ Min, Max int }

func (e *VersionError) Error() string {
	return fmt.Sprintf("kisui-terminal accepts protocol versions %d to %d; this shj speaks %d", e.Min, e.Max, ctl.PrimaryProtocol)
}

// Pane is how the shj that the server accepted as the primary uses the GUI's
// panes (ctl.PaneRequest). Every call is a connection of its own.
type Pane struct {
	path   string
	termID string
	pid    int
}

// Do sends req (Op and the fields it needs) and returns the reply. The shj's
// termId and pid are filled in.
func (p *Pane) Do(req ctl.PaneRequest) (ctl.PaneReply, error) {
	req.TermID, req.PID = p.termID, p.pid
	c, err := net.DialTimeout("unix", p.path, 2*time.Second)
	if err != nil {
		return ctl.PaneReply{}, err
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	line, _ := json.Marshal(req)
	if _, err := c.Write(append(line, '\n')); err != nil {
		return ctl.PaneReply{}, err
	}
	var r ctl.PaneReply
	if err := json.NewDecoder(bufio.NewReader(c)).Decode(&r); err != nil {
		return ctl.PaneReply{}, err
	}
	if !r.OK {
		return r, errors.New(r.Error)
	}
	return r, nil
}

// FocusExplorer asks the GUI to move the keys to its file explorer (Space e).
// It fails when the server does not know the request (a kisui from before it).
func (p *Pane) FocusExplorer() error {
	_, err := p.Do(ctl.PaneRequest{Op: "explorer"})
	return err
}
