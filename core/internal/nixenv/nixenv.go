// Package nixenv reads the development environment of a folder's flake.nix
// with `nix print-dev-env --json`, for a workspace that has no .envrc: nix has
// no `direnv allow`, so whether to read a folder is asked once and the answer
// is kept here.
package nixenv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mm1995tk/shijimi-editor/core/msg"
)

// Timeout bounds one `nix print-dev-env`, which may build the shell.
const Timeout = 5 * time.Minute

// Wanted reports whether root is a folder to read a flake's environment from:
// it has a flake.nix and nix is installed (found on the process's own PATH).
// The caller has already ruled out the folders direnv serves.
func Wanted(root string) bool {
	if st, err := os.Stat(filepath.Join(root, "flake.nix")); err != nil || st.IsDir() {
		return false
	}
	_, err := exec.LookPath("nix")
	return err == nil
}

// Load runs `nix print-dev-env --json` in root and returns the environment (in
// os.Environ form) the commands of the workspace run with: base with the
// variables the dev shell exports. base is the process's environment if it is
// nil.
func Load(ctx context.Context, root string, base []string) ([]string, error) {
	if base == nil {
		base = os.Environ()
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "nix", "--extra-experimental-features", "nix-command flakes", "print-dev-env", "--json")
	cmd.Dir = root
	cmd.Env = base
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, msg.Err("nixenv.timeout", int(Timeout.Minutes()))
	}
	if err != nil {
		return nil, errors.New(lastLine(stderr.String(), err.Error()))
	}
	return Apply(base, stdout.Bytes())
}

// skip are the variables a dev shell sets for building, not for working in:
// taking them would point HOME or the temporary folder at a build sandbox that
// does not exist.
var skip = map[string]bool{
	"HOME": true, "USER": true, "LOGNAME": true, "SHELL": true, "TERM": true,
	"PWD": true, "OLDPWD": true, "SHLVL": true, "_": true,
	"TMP": true, "TMPDIR": true, "TEMP": true, "TEMPDIR": true,
	"NIX_BUILD_TOP": true, "NIX_LOG_FD": true, "NIX_STORE": true,
	"builder": true, "out": true, "outputs": true,
}

// Apply returns base with the exported variables in out, the JSON
// `nix print-dev-env --json` prints, except those in skip.
func Apply(base []string, out []byte) ([]string, error) {
	var d struct {
		Variables map[string]struct {
			Type  string          `json:"type"`
			Value json.RawMessage `json:"value"`
		} `json:"variables"`
	}
	if err := json.Unmarshal(out, &d); err != nil {
		return nil, err
	}
	set := map[string]string{}
	for k, v := range d.Variables {
		var s string
		if v.Type != "exported" || skip[k] || json.Unmarshal(v.Value, &s) != nil {
			continue
		}
		set[k] = s
	}
	env := make([]string, 0, len(base)+len(set))
	for _, kv := range base {
		k, _, _ := strings.Cut(kv, "=")
		if _, ok := set[k]; !ok {
			env = append(env, kv)
		}
	}
	for k, v := range set {
		env = append(env, k+"="+v)
	}
	return env, nil
}

func lastLine(s, def string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if l := strings.TrimSpace(lines[len(lines)-1]); l != "" {
		return l
	}
	return def
}

// ---- the answers ----

// Answers are the folders the user said yes or no to, kept in a file in dir.
type Answers struct{ path string }

var fileMu sync.Mutex // one file, many workspaces

// NewAnswers is the answers kept in dir. An empty dir keeps none: every
// folder is asked again.
func NewAnswers(dir string) Answers {
	if dir == "" {
		return Answers{}
	}
	return Answers{filepath.Join(dir, "nix-devshell.json")}
}

type file struct {
	Version int             `json:"version"`
	Folders map[string]bool `json:"folders"` // folder -> may its flake be read
}

func (a Answers) read() file {
	f := file{Version: 1, Folders: map[string]bool{}}
	if a.path == "" {
		return f
	}
	if data, err := os.ReadFile(a.path); err == nil {
		_ = json.Unmarshal(data, &f)
	}
	if f.Folders == nil {
		f.Folders = map[string]bool{}
	}
	return f
}

// Get is the answer for root: allowed, and whether it was answered at all.
func (a Answers) Get(root string) (allowed, answered bool) {
	fileMu.Lock()
	defer fileMu.Unlock()
	allowed, answered = a.read().Folders[root]
	return
}

// Set keeps the answer for root. Without a file it is lost.
func (a Answers) Set(root string, allowed bool) error {
	if a.path == "" {
		return nil
	}
	fileMu.Lock()
	defer fileMu.Unlock()
	f := a.read()
	f.Folders[root] = allowed
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(a.path), 0o755); err != nil {
		return err
	}
	tmp := a.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, a.path)
}
