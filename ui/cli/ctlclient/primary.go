package ctlclient

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"time"

	"github.com/mm1995tk/shijimi-editor/core/ctl"
)

// AnnouncePrimary tells "shj serve" that this shj, whose control socket is
// ownSocket, runs in the primary terminal of a GUI workspace. It does nothing
// unless ctl.EnvGUIPrimary is set, and it removes the variable either way so
// that a shj started from this one (or from a terminal panel of it) is never
// taken for the primary. It is best effort: the editor works the same when the
// server is gone or refuses. The returned function ends the announcement; the
// server also sees it end when this process does. primary is whether the server
// accepted it as the primary shj: only then does the GUI show the file tree, so
// only then does this shj leave its own out.
func AnnouncePrimary(suffix, ownSocket string) (end func(), primary bool) {
	termID := os.Getenv(ctl.EnvGUIPrimary)
	if termID == "" {
		return func() {}, false
	}
	os.Unsetenv(ctl.EnvGUIPrimary)
	path, err := ctl.ServePrimarySocketPath(suffix)
	if err != nil {
		return func() {}, false
	}
	return announce(path, ctl.PrimaryHello{TermID: termID, Socket: ownSocket, PID: os.Getpid()})
}

func announce(path string, h ctl.PrimaryHello) (func(), bool) {
	c, err := net.DialTimeout("unix", path, 2*time.Second)
	if err != nil {
		return func() {}, false
	}
	_ = c.SetDeadline(time.Now().Add(2 * time.Second))
	line, _ := json.Marshal(h)
	if _, err := c.Write(append(line, '\n')); err != nil {
		c.Close()
		return func() {}, false
	}
	var r ctl.PrimaryReply
	if err := json.NewDecoder(bufio.NewReader(c)).Decode(&r); err != nil || !r.OK {
		c.Close()
		return func() {}, false
	}
	_ = c.SetDeadline(time.Time{})
	return func() { c.Close() }, true
}
