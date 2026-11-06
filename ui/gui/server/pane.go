package server

// プライマリの shj が端末のコマンド（Ctrl+`・:sh・:run など）に使う欄。設計は docs/design/gui-protocol.md の「プライマリの shj の欄」。
//
// 欄は同じワークスペースの分割したターミナルで、shj serve が term.create と同じように始める。頼むのは
// プライマリとして名乗った shj（ctl.PaneRequest。名乗りとは別の接続で 1 回ごと）。見せる・隠すは GUI の状態なので、
// pane.show・pane.hide を GUI に通知するだけにする。shj が終わったら（名乗りの接続が切れたら）その shj の欄を閉じる。

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/mm1995tk/shijimi-editor/core/ctl"
)

// paneTailMax は tail で返す行数の上限。
const paneTailMax = 200

// paneConn は ctl.PaneRequest 1 つを受けて、ctl.PaneReply を返す。
func (s *Server) paneConn(c net.Conn, line []byte) {
	var rq ctl.PaneRequest
	if err := json.Unmarshal(line, &rq); err != nil {
		replyPane(c, ctl.PaneReply{Error: err.Error()})
		return
	}
	r, err := s.ws.pane(rq)
	if err != nil {
		r = ctl.PaneReply{Error: err.Error()}
	}
	replyPane(c, r)
}

func replyPane(c net.Conn, r ctl.PaneReply) {
	r.OK = r.Error == ""
	b, _ := json.Marshal(r)
	_, _ = c.Write(append(b, '\n'))
}

// pane は rq を行う。頼んだ shj が、そのワークスペースのプライマリの shj でなければ断る。
func (w *wsManager) pane(rq ctl.PaneRequest) (ctl.PaneReply, error) {
	t, err := w.srv.terms.get(rq.TermID)
	if err != nil {
		return ctl.PaneReply{}, err
	}
	t.mu.Lock()
	primary, wsID := t.primary, t.wsID
	t.mu.Unlock()
	if !primary {
		return ctl.PaneReply{}, errors.New("not a primary terminal")
	}
	w.mu.Lock()
	st := w.reg[wsID]
	var claim *primaryShj
	if st != nil {
		claim = st.shj
	}
	if claim == nil || claim.PID != rq.PID {
		w.mu.Unlock()
		return ctl.PaneReply{}, errors.New("not the shj of the primary")
	}
	w.mu.Unlock()

	if rq.Op == "split" {
		id, err := w.splitPane(claim, wsID, rq)
		return ctl.PaneReply{ID: id}, err
	}
	if rq.Op == "hide" {
		w.srv.notifyOwner("pane.hide", map[string]any{"workspaceId": wsID})
		return ctl.PaneReply{}, nil
	}
	// 以下は、この shj が作った欄だけ。
	w.mu.Lock()
	own := false
	for _, id := range claim.panes {
		own = own || id == rq.ID
	}
	w.mu.Unlock()
	if !own {
		if rq.Op == "alive" { // 閉じたあとに聞かれても、いないだけ
			return ctl.PaneReply{}, nil
		}
		return ctl.PaneReply{}, fmt.Errorf("not a pane of this shj: %s", rq.ID)
	}
	pt, err := w.srv.terms.get(rq.ID)
	switch rq.Op {
	case "alive":
		return ctl.PaneReply{Alive: err == nil && pt.isAlive()}, nil
	case "close":
		w.closePane(claim, rq.ID)
		return ctl.PaneReply{}, nil
	}
	if err != nil {
		return ctl.PaneReply{}, err
	}
	switch rq.Op {
	case "show":
		w.srv.notifyOwner("pane.show", map[string]any{"workspaceId": wsID, "termId": rq.ID})
		return ctl.PaneReply{}, nil
	case "send":
		text := rq.Text
		if rq.Enter {
			text += "\r"
		}
		ptmx, _ := pt.proc()
		if _, err := ptmx.WriteString(text); err != nil {
			return ctl.PaneReply{}, fmt.Errorf("terminal has exited: %s", rq.ID)
		}
		return ctl.PaneReply{}, nil
	case "tail":
		return ctl.PaneReply{Lines: pt.tailLines(min(max(rq.N, 1), paneTailMax))}, nil
	}
	return ctl.PaneReply{}, fmt.Errorf("unknown op: %q", rq.Op)
}

// splitPane は claim の shj の欄を 1 つ足して GUI に出させる。フォルダは cwd（無いか、フォルダでなければワークスペースのフォルダ）。
func (w *wsManager) splitPane(claim *primaryShj, wsID string, rq ctl.PaneRequest) (string, error) {
	var env []string
	for _, kv := range rq.Env {
		k, _, ok := strings.Cut(kv, "=")
		if !ok || k == "" || strings.ContainsRune(kv, 0) {
			return "", fmt.Errorf("invalid env: %q", kv)
		}
		env = append(env, kv)
	}
	dir, ok := w.folderOf(wsID)
	if !ok {
		return "", fmt.Errorf("no such workspace: %s", wsID)
	}
	if fi, err := os.Stat(rq.Cwd); err == nil && fi.IsDir() && rq.Cwd != "" {
		dir = rq.Cwd
	}
	w.recomputeMu.Lock()
	defer w.recomputeMu.Unlock()
	if w.srv.terms.countSplit(wsID) >= maxSplitTerms {
		return "", fmt.Errorf("the workspace already has %d terminals", maxSplitTerms+1)
	}
	t, err := w.srv.terms.start(termStart{cols: 80, rows: 24, dir: dir, env: env, wsID: wsID})
	if err != nil {
		return "", err
	}
	w.mu.Lock()
	claim.panes = append(claim.panes, t.id)
	w.mu.Unlock()
	w.recomputeLocked() // workspace.changed が先に届くので、GUI は欄を知っている
	w.srv.notifyOwner("pane.show", map[string]any{"workspaceId": wsID, "termId": t.id})
	return t.id, nil
}

// closePane は claim の shj の欄を終わらせて忘れる（term.close と同じ）。
func (w *wsManager) closePane(claim *primaryShj, id string) {
	w.mu.Lock()
	claim.panes = removeStr(claim.panes, id)
	w.mu.Unlock()
	m := w.srv.terms
	t, err := m.get(id)
	if err != nil {
		return
	}
	m.mu.Lock()
	delete(m.terms, id)
	m.mu.Unlock()
	t.hangup()
	t.mu.Lock()
	t.alive = false
	t.mu.Unlock()
	w.recompute()
}

func removeStr(l []string, s string) []string {
	out := l[:0:0]
	for _, x := range l {
		if x != s {
			out = append(out, x)
		}
	}
	return out
}

func (t *terminal) isAlive() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.alive
}

// tailLines は画面の最後の n 行。ターミナルの画面は持たないので、残してある出力から制御列を取り除いて行に分ける。
func (t *terminal) tailLines(n int) []string {
	t.mu.Lock()
	_, data := t.ringBytes(0, t.total)
	t.mu.Unlock()
	text := strings.ReplaceAll(ansi.Strip(string(data)), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(strings.TrimRight(text, "\n \t"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}
