package server

// ワークスペース。設計は docs/design/gui-protocol.md の「ワークスペース」。
//
// ワークスペースはターミナルのシェルの cwd から決める。シェルの cwd を 1 秒に 1 回読み、
// 変わったターミナルだけ term.cwd を通知する。ワークスペースの一覧は、生きているターミナルから
// 作り直して前回と比べ、workspace.changed にする。
// core はまだ 1 つなので（P14）、files は空にしてある。

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/mm1995tk/shijimi-editor/internal/rpc"
)

// Workspace の名前は ctl / ide 用のインターフェースが使っているので、ここでは wsInfo。
type wsInfo struct {
	ID     string   `json:"id"`
	Root   string   `json:"root"`
	Kind   string   `json:"kind"` // "git" | "shijimi" | "cwd"
	Name   string   `json:"name"`
	Branch string   `json:"branch,omitempty"`
	Terms  []string `json:"terms"`
	Files  []string `json:"files"`
}

type wsManager struct {
	srv *Server
	// recomputeMu は recompute 全体を 1 つずつにする（通知の順序を保つ）。
	recomputeMu sync.Mutex

	mu       sync.Mutex
	home     string
	list     map[string]*wsInfo // id -> 今のワークスペース
	focus    string             // フォーカスしているワークスペースの id。無ければ ""
	lastTerm map[string]string  // id -> 最後に入力したターミナル
}

func newWSManager(s *Server) *wsManager {
	home, _ := os.UserHomeDir()
	return &wsManager{srv: s, home: normalizePath(home), list: map[string]*wsInfo{}, lastTerm: map[string]string{}}
}

// resolve は cwd からワークスペースの id を決める。ワークスペースにしないときは ""。
func (w *wsManager) resolve(cwd string) (id, root, kind string) {
	root, kind, ok := findRoot(normalizePath(cwd), w.home)
	if !ok {
		return "", "", ""
	}
	return workspaceID(root), root, kind
}

// snapshot は今のワークスペースの一覧（root の順）と focus。
func (w *wsManager) snapshot() ([]*wsInfo, string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := append([]*wsInfo{}, slices.Collect(maps.Values(w.list))...)
	sortWS(out)
	return out, w.focus
}

func (w *wsManager) count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.list)
}

func (w *wsManager) focusRoot() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if x := w.list[w.focus]; x != nil {
		return x.Root
	}
	return ""
}

// recompute は生きているターミナルからワークスペースを作り直し、前回との差を workspace.changed で通知する。
func (w *wsManager) recompute() {
	w.recomputeMu.Lock()
	defer w.recomputeMu.Unlock()
	next := buildWorkspaces(w.srv.terms.all())
	w.mu.Lock()
	prev := w.list
	w.list = next
	if _, ok := next[w.focus]; !ok {
		// フォーカスが消えた。GUI は removed から知る。P14 で Screen の宛先が要るときに通知を足す。
		w.focus = ""
	}
	added, updated, removed := diffWorkspaces(prev, next)
	for _, id := range removed {
		delete(w.lastTerm, id)
	}
	w.mu.Unlock()
	if p := changedParams(added, updated, removed); p != nil {
		w.srv.notifyOwner("workspace.changed", p)
	}
}

// buildWorkspaces は生きているターミナルをルートごとにまとめる。
func buildWorkspaces(terms []*terminal) map[string]*wsInfo {
	next := map[string]*wsInfo{}
	for _, t := range terms {
		t.mu.Lock()
		alive, id, root, kind, tid := t.alive, t.wsID, t.wsRoot, t.wsKind, t.id
		t.mu.Unlock()
		if !alive || id == "" {
			continue
		}
		x := next[id]
		if x == nil {
			x = &wsInfo{ID: id, Root: root, Kind: kind, Name: filepath.Base(root), Terms: []string{}, Files: []string{}}
			if kind == "git" {
				x.Branch = gitBranch(root)
			}
			next[id] = x
		}
		x.Terms = append(x.Terms, tid)
	}
	for _, x := range next {
		slices.SortFunc(x.Terms, func(a, b string) int { return termIDNum(a) - termIDNum(b) })
	}
	return next
}

// diffWorkspaces は前回と今回の差。
func diffWorkspaces(prev, next map[string]*wsInfo) (added, updated []*wsInfo, removed []string) {
	for id, x := range next {
		p, ok := prev[id]
		switch {
		case !ok:
			added = append(added, x)
		case !wsEqual(p, x):
			updated = append(updated, x)
		}
	}
	for id := range prev {
		if _, ok := next[id]; !ok {
			removed = append(removed, id)
		}
	}
	sortWS(added)
	sortWS(updated)
	slices.Sort(removed)
	return
}

// changedParams は workspace.changed のパラメータ。差が無ければ nil。
func changedParams(added, updated []*wsInfo, removed []string) map[string]any {
	if len(added)+len(updated)+len(removed) == 0 {
		return nil
	}
	p := map[string]any{}
	if len(added) > 0 {
		p["added"] = added
	}
	if len(removed) > 0 {
		p["removed"] = removed
	}
	if len(updated) > 0 {
		p["updated"] = updated
	}
	return p
}

func sortWS(l []*wsInfo) { sort.Slice(l, func(i, j int) bool { return l[i].Root < l[j].Root }) }

func wsEqual(a, b *wsInfo) bool {
	return a.Root == b.Root && a.Kind == b.Kind && a.Name == b.Name && a.Branch == b.Branch &&
		slices.Equal(a.Terms, b.Terms) && slices.Equal(a.Files, b.Files)
}

// watch は cwd を 1 秒に 1 回（Options.CwdInterval）読む。done が閉じるまで。
func (w *wsManager) watch(done <-chan struct{}) {
	iv := w.srv.opts.CwdInterval
	if iv <= 0 {
		iv = time.Second
	}
	tk := time.NewTicker(iv)
	defer tk.Stop()
	for {
		select {
		case <-done:
			return
		case <-tk.C:
			w.poll()
		}
	}
}

// poll は全ターミナルの cwd を読み、変わったものだけ term.cwd を送る。ブランチの変化も拾うので recompute も毎回する。
func (w *wsManager) poll() {
	for _, t := range w.srv.terms.all() {
		t.mu.Lock()
		alive, pid, old := t.alive, t.pid, t.cwd
		t.mu.Unlock()
		if !alive {
			continue
		}
		cwd, err := shellCwd(pid)
		if err != nil || cwd == "" || cwd == old {
			continue
		}
		w.setCwd(t, cwd)
	}
	w.recompute()
}

// setCwd は t の cwd とワークスペースを更新し、term.cwd を送る。workspace.changed は recompute が送る。
func (w *wsManager) setCwd(t *terminal, cwd string) {
	id, root, kind := w.resolve(cwd)
	t.mu.Lock()
	t.cwd, t.wsID, t.wsRoot, t.wsKind = cwd, id, root, kind
	tid := t.id
	t.mu.Unlock()
	var wid any
	if id != "" {
		wid = id
	}
	w.srv.notifyOwner("term.cwd", map[string]any{"termId": tid, "cwd": cwd, "workspaceId": wid})
}

func (w *wsManager) touch(t *terminal) {
	t.mu.Lock()
	id := t.wsID
	t.mu.Unlock()
	if id == "" {
		return
	}
	w.mu.Lock()
	w.lastTerm[id] = t.id
	w.mu.Unlock()
}

// request は workspace.* のうち返事のあるもの。
func (ss *session) workspaceRequest(req *rpc.Message) (res any, err error, handled bool) {
	switch req.Method {
	case "workspace.list", "workspace.focus", "workspace.close_files":
	default:
		return nil, nil, false
	}
	if err := ss.needHello(); err != nil {
		return nil, err, true
	}
	w := ss.srv.ws
	if req.Method == "workspace.list" {
		l, f := w.snapshot()
		var focus any
		if f != "" {
			focus = f
		}
		return map[string]any{"workspaces": l, "focus": focus}, nil, true
	}
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		return nil, rpc.Errorf(rpc.CodeInvalidParams, "invalid params: %v", err), true
	}
	w.mu.Lock()
	x := w.list[p.ID]
	var terms []string
	if x != nil {
		terms = slices.Clone(x.Terms)
	}
	last := w.lastTerm[p.ID]
	w.mu.Unlock()
	if x == nil {
		return nil, rpc.Errorf(rpc.CodeNotFound, "no such workspace: %s", p.ID), true
	}
	if req.Method == "workspace.close_files" {
		// core がワークスペースごとになる（P14）まで、開いているファイルは無い。
		return struct{}{}, nil, true
	}
	w.mu.Lock()
	w.focus = p.ID
	w.mu.Unlock()
	rev := ss.srv.bumpRev()
	out := map[string]any{"rev": rev}
	if !slices.Contains(terms, last) {
		last = ""
		if len(terms) > 0 {
			last = terms[0]
		}
	}
	if last != "" {
		out["termId"] = last
	}
	return out, nil, true
}
