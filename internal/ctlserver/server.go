// Package ctlserver is the editor's side of the control socket: it listens on
// a Unix socket and hands each request to a ctl.Handler.
package ctlserver

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/ctl"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// Server accepts connections on a Unix socket.
type Server struct {
	Path string
	link string // the herdr link to Path, or ""
	ln   net.Listener
}

// Listen creates the socket (mode 0600) and serves h until Close. Inside a
// herdr pane it also links the socket under a name built from the herdr
// environment (see herdrLink), so that shj finds it without SHIJIMI_SOCKET.
func Listen(path string, h ctl.Handler) (*Server, error) {
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return nil, err
	}
	s := &Server{Path: path, ln: ln}
	// The link is best-effort: shj still works through SHIJIMI_SOCKET.
	if link := herdrLink(filepath.Dir(path)); link != "" {
		if err := os.MkdirAll(filepath.Dir(link), 0o700); err == nil {
			_ = os.Remove(link)
			if err := os.Symlink(path, link); err == nil {
				s.link = link
			}
		}
	}
	go s.serve(h)
	return s, nil
}

// Close stops serving and removes the socket file and the herdr link.
func (s *Server) Close() {
	s.ln.Close()
	_ = os.Remove(s.Path)
	if s.link != "" {
		_ = os.Remove(s.link)
		// The workspace directory goes too once it is empty.
		_ = os.Remove(filepath.Dir(s.link))
	}
}

// herdrLink returns where the editor of this herdr pane links its socket:
// <dir>/herdr-<workspace>/<pane>.sock, or <dir>/herdr-<workspace>/pid-<pid>.sock
// when herdr did not tell the pane. It returns "" outside herdr.
func herdrLink(dir string) string {
	if os.Getenv(ctl.EnvHerdr) != "1" {
		return ""
	}
	ws := os.Getenv(ctl.EnvHerdrWorkspace)
	if ws == "" {
		return ""
	}
	name := "pid-" + strconv.Itoa(os.Getpid())
	if pane := os.Getenv(ctl.EnvHerdrPane); pane != "" {
		name = ctl.SafeName(pane)
	}
	return filepath.Join(ctl.HerdrDir(dir, ws), name+".sock")
}

func (s *Server) serve(h ctl.Handler) {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		go func() {
			defer c.Close()
			_ = c.SetDeadline(time.Now().Add(10 * time.Second))
			var req ctl.Request
			var resp ctl.Response
			line, err := bufio.NewReader(c).ReadBytes('\n')
			if err == nil || len(line) > 0 {
				err = json.Unmarshal(line, &req)
			}
			if err != nil {
				resp = ctl.Response{Error: i18n.T("ctl.badrequest") + err.Error()}
			} else {
				resp = h(req)
			}
			_ = json.NewEncoder(c).Encode(resp)
		}()
	}
}
