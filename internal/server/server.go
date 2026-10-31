// Package server は画面を持たない常駐プロセス「shj serve」の中身。
// Unix ソケットで JSON-RPC を話す。設計は docs/design/gui-protocol.md の 3 章。
//
// 今あるメソッドは session.hello / session.bye / session.ping / server.info だけ。
// ワークスペースやターミナルなどは後の Issue で足す。
package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/rpc"
)

// Protocol はこのサーバーが話すプロトコルの版。major が違うとつながらない。minor は互換。
var Protocol = ProtocolVersion{Major: 1, Minor: 0}

// ProtocolVersion はプロトコルの版。
type ProtocolVersion struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
}

// Shj はこの shj の版。
type Shj struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Channel string `json:"channel"` // "stable" か "nightly"
}

// Options は Listen に渡すもの。
type Options struct {
	SocketPath string
	Shj        Shj
	Logger     *slog.Logger // nil なら捨てる
}

// Server は Unix ソケットで待ち受けるサーバー。
type Server struct {
	opts      Options
	log       *slog.Logger
	ln        net.Listener
	serverID  string
	startedAt time.Time

	mu         sync.Mutex
	sessions   map[*session]struct{}
	lastClient time.Time // 最後に GUI がつながっていた時刻。ゼロなら一度も無い
	wg         sync.WaitGroup
}

// ErrRunning はすでに別の shj serve が動いているときのエラー。
var ErrRunning = errors.New("shj serve is already running")

// Listen はソケットを作って待ち受ける（0600）。同じ場所で別のサーバーが動いていれば ErrRunning。
// 残っているだけの古いソケットは作り直す。
func Listen(opts Options) (*Server, error) {
	if c, err := net.DialTimeout("unix", opts.SocketPath, time.Second); err == nil {
		c.Close()
		return nil, ErrRunning
	}
	_ = os.Remove(opts.SocketPath)
	ln, err := net.Listen("unix", opts.SocketPath)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(opts.SocketPath, 0o600); err != nil {
		ln.Close()
		return nil, err
	}
	id := make([]byte, 8)
	if _, err := rand.Read(id); err != nil {
		ln.Close()
		return nil, err
	}
	log := opts.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Server{
		opts:      opts,
		log:       log,
		ln:        ln,
		serverID:  hex.EncodeToString(id),
		startedAt: time.Now(),
		sessions:  map[*session]struct{}{},
	}, nil
}

// ServerID はこのサーバーの 1 回の起動ごとの番号。
func (s *Server) ServerID() string { return s.serverID }

// Serve は ctx が終わるか Close されるまで接続を受ける。終わるとソケットを消す。
func (s *Server) Serve(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		s.ln.Close()
	}()
	s.log.Info("serving", "socket", s.opts.SocketPath, "serverId", s.serverID, "pid", os.Getpid())
	for {
		c, err := s.ln.Accept()
		if err != nil {
			s.shutdown()
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		s.accept(c)
	}
}

// Close は待ち受けをやめ、つながっている接続も閉じる。
func (s *Server) Close() {
	s.ln.Close()
	s.shutdown()
}

func (s *Server) shutdown() {
	_ = os.Remove(s.opts.SocketPath)
	s.mu.Lock()
	var open []*session
	for ss := range s.sessions {
		open = append(open, ss)
	}
	s.mu.Unlock()
	for _, ss := range open {
		ss.conn.Close()
	}
	s.wg.Wait()
}

func (s *Server) accept(c net.Conn) {
	ss := &session{srv: s}
	s.mu.Lock()
	// Conn の読みは NewConn の中で始まるので、先に登録しておく。
	s.sessions[ss] = struct{}{}
	s.mu.Unlock()
	ss.conn = rpc.NewConn(c, rpc.JSON{}, ss.handle)
	s.log.Info("client connected")
	s.wg.Go(func() {
		<-ss.conn.Done()
		s.mu.Lock()
		delete(s.sessions, ss)
		s.lastClient = time.Now()
		s.mu.Unlock()
		// 状態（ファイル・core・ターミナル）はそのまま残す。
		s.log.Info("client gone", "reason", fmt.Sprint(ss.conn.Err()))
		ss.conn.Close() // fd と Handler の終わりを待つ。Done のあとなので詰まらない
	})
}

// lastClientAt は GUI が最後につながっていた時刻。つながっている間は今。一度も無ければ nil。
func (s *Server) lastClientAt() *time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.sessions) > 0 {
		t := time.Now()
		return &t
	}
	if s.lastClient.IsZero() {
		return nil
	}
	t := s.lastClient
	return &t
}

// session は 1 本の接続の状態。
type session struct {
	srv   *Server
	conn  *rpc.Conn
	mu    sync.Mutex
	hello bool
}

// HelloParams は session.hello のパラメータ。
type HelloParams struct {
	Protocol ProtocolVersion `json:"protocol"`
	Client   struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Kind    string `json:"kind"`
	} `json:"client"`
	Encoding []string `json:"encoding"`
	Resume   *struct {
		ServerID string `json:"serverId"`
		Rev      int64  `json:"rev"`
	} `json:"resume,omitempty"`
}

// HelloResult は session.hello の結果。workspaces は後の Issue で埋まる（今は空）。
type HelloResult struct {
	Protocol     ProtocolVersion `json:"protocol"`
	Encoding     string          `json:"encoding"`
	Shj          Shj             `json:"shj"`
	ServerID     string          `json:"serverId"`
	StartedAt    time.Time       `json:"startedAt"`
	Capabilities []string        `json:"capabilities"`
	Workspaces   []any           `json:"workspaces"`
	Rev          int64           `json:"rev"`
}

// InfoResult は server.info の結果。idleStopAt は B4 で埋まる（今は null）。
type InfoResult struct {
	Shj          Shj        `json:"shj"`
	ServerID     string     `json:"serverId"`
	PID          int        `json:"pid"`
	Uptime       int64      `json:"uptime"` // 秒
	LastClientAt *time.Time `json:"lastClientAt"`
	IdleStopAt   *time.Time `json:"idleStopAt"`
	Workspaces   int        `json:"workspaces"`
	Terminals    int        `json:"terminals"`
}

func (ss *session) handle(_ context.Context, req *rpc.Message) (any, error) {
	if req.IsNotification() {
		return nil, nil // 知らない通知は無視する
	}
	switch req.Method {
	case "session.hello":
		return ss.helloReq(req)
	case "session.ping":
		return struct{}{}, nil
	case "session.bye":
		// GUI を閉じるだけ。shj とシェルは止めない。接続は GUI が閉じる。
		return struct{}{}, nil
	case "server.info":
		if err := ss.needHello(); err != nil {
			return nil, err
		}
		s := ss.srv
		return InfoResult{
			Shj:          s.opts.Shj,
			ServerID:     s.serverID,
			PID:          os.Getpid(),
			Uptime:       int64(time.Since(s.startedAt) / time.Second),
			LastClientAt: s.lastClientAt(),
		}, nil
	}
	return nil, rpc.Errorf(rpc.CodeMethodNotFound, "method not found: %s", req.Method)
}

func (ss *session) needHello() error {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if !ss.hello {
		return rpc.Errorf(rpc.CodeInvalidRequest, "session.hello must come first")
	}
	return nil
}

func (ss *session) helloReq(req *rpc.Message) (any, error) {
	var p HelloParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		return nil, rpc.Errorf(rpc.CodeInvalidParams, "invalid params: %v", err)
	}
	if p.Protocol.Major != Protocol.Major {
		e := rpc.Errorf(rpc.CodeIncompatibleProtocol, "incompatible protocol: server speaks major %d, client major %d", Protocol.Major, p.Protocol.Major)
		e.Data, _ = json.Marshal(map[string]any{"protocol": Protocol})
		return nil, e
	}
	// いまは json だけ。encoding を省いたら json とみなす。
	if len(p.Encoding) > 0 && !slices.Contains(p.Encoding, "json") {
		return nil, rpc.Errorf(rpc.CodeInvalidParams, "no common encoding: server supports json")
	}
	ss.mu.Lock()
	ss.hello = true
	ss.mu.Unlock()
	s := ss.srv
	ss.srv.log.Info("hello", "client", p.Client.Name, "version", p.Client.Version, "kind", p.Client.Kind)
	return HelloResult{
		Protocol:     Protocol,
		Encoding:     "json",
		Shj:          s.opts.Shj,
		ServerID:     s.serverID,
		StartedAt:    s.startedAt,
		Capabilities: []string{},
		Workspaces:   []any{},
	}, nil
}
