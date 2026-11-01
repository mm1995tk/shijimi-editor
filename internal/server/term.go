package server

// ターミナル管理と term.*。設計は docs/design/gui-protocol.md の 3 章「ターミナル」。
//
// PTY の出力は Screen に変換せず、そのまま term.output で流す（internal/vt は使わない）。
// 出力は 1 ターミナルにつき最大 1 MiB のリングバッファに溜め、seq はそのバイト数の累計。
// GUI が term.ack で返した seq から 256 KiB より先は送らない。止まっても PTY は読み続け、
// 溢れた分は捨てる。捨てた分は term.attach で（残っている分だけ）取り直す。
//
// command-end（OSC 133;D）の印はサーバーでは取り出さない。理由は docs/design/gui-protocol.md の「ターミナル」を見る。

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/creack/pty"

	"github.com/mm1995tk/shijimi-editor/internal/rpc"
)

const (
	ringSize   = 1 << 20   // 1 ターミナルにつき溜めておく出力
	ackWindow  = 256 << 10 // ack 済みの seq から先へ送ってよい量
	chunkSize  = 32 << 10  // term.output 1 通の最大
	maxTermDim = 1 << 12
	maxTitle   = 256
)

// termManager はサーバー全体のターミナル。
// command-end（OSC 133;D）をサーバーで取り出さない理由は docs/design/gui-protocol.md の「ターミナル」を見る。
type termManager struct {
	srv    *Server
	mu     sync.Mutex
	next   int
	terms  map[string]*terminal
	wg     sync.WaitGroup
	closed bool
}

func newTermManager(s *Server) *termManager {
	return &termManager{srv: s, terms: map[string]*terminal{}}
}

// all は今あるターミナルを返す。
func (m *termManager) all() []*terminal {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*terminal, 0, len(m.terms))
	for _, t := range m.terms {
		out = append(out, t)
	}
	return out
}

func (m *termManager) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.terms)
}

// shutdown は全部のシェルに SIGHUP を送り、終わるのを待つ。
func (m *termManager) shutdown() {
	m.mu.Lock()
	m.closed = true
	var all []*terminal
	for _, t := range m.terms {
		all = append(all, t)
	}
	m.mu.Unlock()
	for _, t := range all {
		t.hangup()
	}
	m.wg.Wait()
}

type terminal struct {
	mgr  *termManager
	id   string
	pid  int
	ptmx *os.File
	cmd  *exec.Cmd

	// sendMu は term.output の送り出しと term.attach を 1 つずつにする（順序と二重送りを防ぐ）。
	sendMu sync.Mutex

	mu         sync.Mutex
	cwd        string // シェルの今の cwd（workspace.go が更新）
	wsID       string // 属するワークスペース。無ければ ""
	wsRoot     string
	wsKind     string
	cols, rows int
	title      string
	alive      bool
	ring       []byte // 循環バッファ。直近 ringSize バイトが入っている
	total      int64  // これまでに読んだバイト数（seq）
	sent       int64  // ここまで送った
	acked      int64  // GUI が処理したと言ったところ
	osc        oscScan
}

// ringBytes は seq が [from, to) の範囲のバイト列を返す。from が捨てられた分なら残っている先頭から。
// mu を持って呼ぶ。
func (t *terminal) ringBytes(from, to int64) (int64, []byte) {
	oldest := t.oldest()
	from = max(from, oldest)
	to = min(to, t.total)
	if from >= to {
		return max(from, to), nil
	}
	out := make([]byte, to-from)
	pos := int(from % ringSize)
	n := copy(out, t.ring[pos:])
	copy(out[n:], t.ring)
	return from, out
}

// oldest はリングに残っている一番古い seq。mu を持って呼ぶ。
func (t *terminal) oldest() int64 { return max(0, t.total-ringSize) }

// ringAppend は mu を持って呼ぶ。ring は常に ringSize バイトの循環バッファで、seq s は ring[s%ringSize] に入る。
func (t *terminal) ringAppend(b []byte) {
	for len(b) > 0 {
		n := copy(t.ring[t.total%ringSize:], b)
		t.total += int64(n)
		b = b[n:]
	}
}

func (t *terminal) info() map[string]any {
	t.mu.Lock()
	defer t.mu.Unlock()
	var wid any
	if t.wsID != "" {
		wid = t.wsID
	}
	return map[string]any{
		"termId": t.id, "pid": t.pid, "cwd": t.cwd, "workspaceId": wid,
		"title": t.title, "cols": t.cols, "rows": t.rows, "alive": t.alive,
	}
}

func (t *terminal) hangup() {
	// SIGHUP を無視するシェルで shutdown が止まらないように、少し待って SIGKILL。
	if t.cmd.Process != nil {
		p := t.cmd.Process
		time.AfterFunc(3*time.Second, func() { _ = p.Kill() })
	}
	if t.cmd.Process != nil {
		_ = t.cmd.Process.Signal(syscall.SIGHUP)
	}
	// ptmx を閉じても SIGHUP が届く。シェルが SIGHUP を無視しても pump は read の失敗で終わる。
	t.ptmx.Close()
}

func (m *termManager) create(p termCreateParams) (any, error) {
	if p.Cols <= 0 || p.Rows <= 0 || p.Cols > maxTermDim || p.Rows > maxTermDim {
		return nil, rpc.Errorf(rpc.CodeInvalidParams, "cols and rows must be 1..%d", maxTermDim)
	}
	cwd := p.Cwd
	if cwd == "" {
		cwd = m.srv.ws.focusRoot()
	}
	if cwd == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "/"
		}
		cwd = home
	}
	if fi, err := os.Stat(cwd); err != nil || !fi.IsDir() {
		return nil, rpc.Errorf(rpc.CodeInvalidParams, "cwd is not a directory: %s", cwd)
	}
	sh := p.Shell
	if sh == "" {
		sh = os.Getenv("SHELL")
	}
	if sh == "" {
		sh = "/bin/sh"
	}
	cmd := exec.Command(sh)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")
	for k, v := range p.Env {
		if k == "" || strings.ContainsAny(k, "=\x00") {
			return nil, rpc.Errorf(rpc.CodeInvalidParams, "invalid env name: %q", k)
		}
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, rpc.Errorf(rpc.CodeBusy, "server is shutting down")
	}
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(p.Rows), Cols: uint16(p.Cols)})
	if err != nil {
		m.mu.Unlock()
		return nil, rpc.Errorf(rpc.CodeInternalError, "start shell: %v", err)
	}
	m.next++
	t := &terminal{
		mgr: m, id: fmt.Sprintf("t%d", m.next), pid: cmd.Process.Pid,
		ptmx: ptmx, cmd: cmd, cols: p.Cols, rows: p.Rows, alive: true,
		ring: make([]byte, ringSize),
	}
	t.cwd = normalizePath(cwd) // 以後 cwd の変化を見るので、読み取った値（実パス）に合わせる
	t.wsID, t.wsRoot, t.wsKind = m.srv.ws.resolve(cwd)
	m.terms[t.id] = t
	m.wg.Go(t.pump)
	m.mu.Unlock()
	m.srv.ws.recompute()
	m.srv.log.Info("term created", "termId", t.id, "pid", t.pid, "shell", sh, "cwd", cwd)
	return map[string]any{"termId": t.id, "pid": t.pid, "cwd": t.cwd}, nil
}

func (m *termManager) get(id string) (*terminal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.terms[id]
	if t == nil {
		return nil, rpc.Errorf(rpc.CodeNotFound, "no such terminal: %s", id)
	}
	return t, nil
}

// pump は PTY を読み続けてリングバッファに溜め、送れるなら送る。止まった GUI には送らないが読むのはやめない。
func (t *terminal) pump() {
	buf := make([]byte, chunkSize)
	for {
		n, err := t.ptmx.Read(buf)
		if n > 0 {
			t.mu.Lock()
			t.ringAppend(buf[:n])
			title, changed := t.osc.feed(buf[:n])
			if changed {
				t.title = title
			}
			t.mu.Unlock()
			if changed {
				t.mgr.srv.notifyOwner("term.title", map[string]any{"termId": t.id, "title": title})
			}
			t.flush()
		}
		if err != nil {
			break
		}
	}
	_ = t.cmd.Wait()
	t.ptmx.Close()
	exit := map[string]any{"termId": t.id}
	if ws, ok := t.cmd.ProcessState.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		exit["code"] = 128 + int(ws.Signal())
		exit["signal"] = ws.Signal().String()
	} else {
		exit["code"] = t.cmd.ProcessState.ExitCode()
	}
	t.mu.Lock()
	t.alive = false
	t.mu.Unlock()
	t.flush() // 最後の出力が先に届くように
	t.mgr.srv.notifyOwner("term.exit", exit)
	t.mgr.srv.ws.recompute()
	t.mgr.srv.log.Info("term exited", "termId", t.id, "code", exit["code"])
}

// flush は送ってよい分（ack から 256 KiB まで）を term.output にして owner に送る。
func (t *terminal) flush() {
	t.sendMu.Lock()
	defer t.sendMu.Unlock()
	for {
		t.mu.Lock()
		// 窓は ack 済みか、リングに残っている先頭のどちらか進んだ方から数える。溢れて捨てられた分は
		// GUI が受け取れず ack も進められないので、ack だけを基準にすると永久に止まる。
		// 捨てられた分は seq が飛んで届くので、GUI は欠けを知って term.attach で取り直せる。
		base := max(t.acked, t.oldest())
		from, data := t.ringBytes(t.sent, min(t.total, base+ackWindow, t.sent+chunkSize))
		// 溢れて捨てられた分は送れないので、送った扱いにして先へ進む。
		if len(data) == 0 {
			t.sent = max(t.sent, from)
			t.mu.Unlock()
			return
		}
		seq := from + int64(len(data))
		id := t.id
		t.mu.Unlock()
		if !t.mgr.srv.notifyOwner("term.output", map[string]any{
			"termId": id, "seq": seq, "data": base64.StdEncoding.EncodeToString(data),
		}) {
			return // つながっている GUI がいない。溜めたままにして、attach で取り直してもらう。
		}
		t.mu.Lock()
		t.sent = seq
		t.mu.Unlock()
	}
}

func (t *terminal) ack(seq int64) {
	t.mu.Lock()
	if seq > t.acked && seq <= t.total {
		t.acked = seq
	}
	t.mu.Unlock()
	t.flush()
}

func (t *terminal) attach(from *int64) (any, error) {
	t.sendMu.Lock()
	defer t.sendMu.Unlock()
	t.mu.Lock()
	defer t.mu.Unlock()
	f := int64(0)
	if from != nil {
		f = max(0, *from)
	}
	_, data := t.ringBytes(f, t.total)
	// 取り直した分は GUI が受け取ったことになる。ここから先を流す。
	t.sent, t.acked = t.total, t.total
	return map[string]any{
		"seq": t.total, "replay": base64.StdEncoding.EncodeToString(data), "cols": t.cols, "rows": t.rows,
	}, nil
}

type termCreateParams struct {
	Cwd   string            `json:"cwd"`
	Cols  int               `json:"cols"`
	Rows  int               `json:"rows"`
	Shell string            `json:"shell"`
	Env   map[string]string `json:"env"`
}

// notifyOwner は今の GUI に通知を送る。送れたら true。
func (s *Server) notifyOwner(method string, params any) bool {
	s.mu.Lock()
	o := s.owner
	s.mu.Unlock()
	if o == nil {
		return false
	}
	return o.conn.Notify(method, params) == nil
}

// termRequest は term.* のうち返事のあるもの。扱わないメソッドは handled == false。
func (ss *session) termRequest(req *rpc.Message) (res any, err error, handled bool) {
	m := ss.srv.terms
	switch req.Method {
	case "term.create", "term.list", "term.attach", "term.close", "term.resize", "term.input":
	default:
		return nil, nil, false
	}
	if err := ss.needHello(); err != nil {
		return nil, err, true
	}
	switch req.Method {
	case "term.create":
		var p termCreateParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, rpc.Errorf(rpc.CodeInvalidParams, "invalid params: %v", err), true
		}
		res, err = m.create(p)
		return res, err, true
	case "term.list":
		m.mu.Lock()
		all := make([]*terminal, 0, len(m.terms))
		for _, t := range m.terms {
			all = append(all, t)
		}
		m.mu.Unlock()
		list := make([]map[string]any, 0, len(all))
		for _, t := range all {
			list = append(list, t.info())
		}
		sortTerms(list)
		return map[string]any{"terms": list}, nil, true
	}
	var p struct {
		TermID  string `json:"termId"`
		FromSeq *int64 `json:"fromSeq"`
		Cols    int    `json:"cols"`
		Rows    int    `json:"rows"`
		Data    string `json:"data"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		return nil, rpc.Errorf(rpc.CodeInvalidParams, "invalid params: %v", err), true
	}
	t, err := m.get(p.TermID)
	if err != nil {
		return nil, err, true
	}
	switch req.Method {
	case "term.attach":
		res, err = t.attach(p.FromSeq)
		return res, err, true
	case "term.close":
		m.mu.Lock()
		delete(m.terms, t.id)
		m.mu.Unlock()
		t.hangup()
		t.mu.Lock()
		t.alive = false
		t.mu.Unlock()
		ss.srv.ws.recompute()
		return struct{}{}, nil, true
	case "term.resize":
		if p.Cols <= 0 || p.Rows <= 0 || p.Cols > maxTermDim || p.Rows > maxTermDim {
			return nil, rpc.Errorf(rpc.CodeInvalidParams, "cols and rows must be 1..%d", maxTermDim), true
		}
		if err := pty.Setsize(t.ptmx, &pty.Winsize{Rows: uint16(p.Rows), Cols: uint16(p.Cols)}); err != nil {
			return nil, rpc.Errorf(rpc.CodeNotFound, "terminal has exited: %s", t.id), true
		}
		t.mu.Lock()
		t.cols, t.rows = p.Cols, p.Rows
		t.mu.Unlock()
		return struct{}{}, nil, true
	default: // term.input
		b, err := base64.StdEncoding.DecodeString(p.Data)
		if err != nil {
			return nil, rpc.Errorf(rpc.CodeInvalidParams, "data is not base64: %v", err), true
		}
		ss.srv.ws.touch(t)
		if _, err := t.ptmx.Write(b); err != nil {
			return nil, rpc.Errorf(rpc.CodeNotFound, "terminal has exited: %s", t.id), true
		}
		return struct{}{}, nil, true
	}
}

// termNotify は term.ack を受ける。
func (ss *session) termNotify(req *rpc.Message) {
	if req.Method != "term.ack" || ss.isSuperseded() || ss.needHello() != nil {
		return
	}
	var p struct {
		TermID string `json:"termId"`
		Seq    int64  `json:"seq"`
	}
	if json.Unmarshal(req.Params, &p) != nil {
		return
	}
	if t, err := ss.srv.terms.get(p.TermID); err == nil {
		t.ack(p.Seq)
	}
}

func sortTerms(l []map[string]any) {
	for i := 1; i < len(l); i++ {
		for j := i; j > 0 && termNum(l[j]) < termNum(l[j-1]); j-- {
			l[j], l[j-1] = l[j-1], l[j]
		}
	}
}

func termNum(m map[string]any) int { return termIDNum(m["termId"].(string)) }

func termIDNum(id string) int {
	var n int
	fmt.Sscanf(id, "t%d", &n)
	return n
}

// oscScan は出力の流れから OSC 0 / 2（タイトル）だけを拾う。画面は作らない。
type oscScan struct {
	state int // 0 通常, 1 ESC の次, 2 OSC の中, 3 OSC の中で ESC
	buf   []byte
}

// feed は b を読み、タイトルが決まったら (title, true) を返す（1 度の呼びで最後のもの）。
func (o *oscScan) feed(b []byte) (title string, changed bool) {
	for _, c := range b {
		switch o.state {
		case 0:
			if c == 0x1b {
				o.state = 1
			}
		case 1:
			if c == ']' {
				o.state, o.buf = 2, o.buf[:0]
			} else if c != 0x1b {
				o.state = 0
			}
		case 2:
			switch c {
			case 0x07:
				title, changed = o.end(title, changed)
			case 0x1b:
				o.state = 3
			default:
				if len(o.buf) < 4*maxTitle {
					o.buf = append(o.buf, c)
				}
			}
		case 3:
			if c == '\\' {
				title, changed = o.end(title, changed)
			} else {
				o.state = 0
			}
		}
	}
	return
}

func (o *oscScan) end(title string, changed bool) (string, bool) {
	o.state = 0
	s := string(o.buf)
	if rest, ok := strings.CutPrefix(s, "0;"); ok {
		s = rest
	} else if rest, ok := strings.CutPrefix(s, "2;"); ok {
		s = rest
	} else {
		return title, changed
	}
	s = strings.ToValidUTF8(s, "")
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	for len(s) > maxTitle {
		_, size := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-size]
	}
	return s, true
}
