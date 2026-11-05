// Package direnv reads the environment direnv makes for a folder, so that the
// commands of a workspace (language servers, formatters) can be found and
// started in it. `shj serve` is started over ssh and has no `nix develop` or
// direnv PATH of its own.
package direnv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/mm1995tk/shijimi-editor/core/msg"
)

// Timeout bounds one `direnv export json`. A `use flake` may build the shell
// the first time, so it is long.
const Timeout = 5 * time.Minute

// ErrBlocked is returned when the folder's .envrc is not allowed
// (`direnv allow` is needed).
var ErrBlocked = errors.New("direnv: .envrc is blocked")

// Wanted reports whether root has an .envrc and direnv is installed (found on
// the process's own PATH).
func Wanted(root string) bool {
	if st, err := os.Stat(filepath.Join(root, ".envrc")); err != nil || st.IsDir() {
		return false
	}
	_, err := exec.LookPath("direnv")
	return err == nil
}

// Load runs `direnv export json` in root and returns the environment (in
// os.Environ form) the commands of the workspace run with: base with the
// changes direnv made. base is the process's environment if it is nil. It
// returns ErrBlocked when direnv refused the .envrc.
func Load(ctx context.Context, root string, base []string) ([]string, error) {
	if base == nil {
		base = os.Environ()
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "direnv", "export", "json")
	cmd.Dir = root
	cmd.Env = base
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	if strings.Contains(stderr.String(), "is blocked") {
		return nil, ErrBlocked
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, msg.Err("direnv.timeout", int(Timeout.Minutes()))
	}
	if err != nil {
		return nil, errors.New(firstLine(stderr.String(), err.Error()))
	}
	return Apply(base, stdout.Bytes())
}

// Apply returns base with the changes in out, the JSON `direnv export json`
// prints: an object of values, where null removes a variable. Empty out is no
// change.
func Apply(base []string, out []byte) ([]string, error) {
	if len(bytes.TrimSpace(out)) == 0 {
		return base, nil
	}
	var changes map[string]*string
	if err := json.Unmarshal(out, &changes); err != nil {
		return nil, err
	}
	env := make([]string, 0, len(base)+len(changes))
	for _, kv := range base {
		k, _, _ := strings.Cut(kv, "=")
		if _, ok := changes[k]; !ok {
			env = append(env, kv)
		}
	}
	for k, v := range changes {
		if v != nil {
			env = append(env, k+"="+*v)
		}
	}
	return env, nil
}

// Getenv is the value of key in env (os.Environ form).
func Getenv(env []string, key string) string {
	for _, e := range slices.Backward(env) {
		if k, v, ok := strings.Cut(e, "="); ok && k == key {
			return v
		}
	}
	return ""
}

// LookPath is exec.LookPath with the PATH of env. A nil env is the process's.
func LookPath(env []string, name string) (string, error) {
	if env == nil || strings.ContainsRune(name, filepath.Separator) {
		return exec.LookPath(name)
	}
	for _, dir := range filepath.SplitList(Getenv(env, "PATH")) {
		if dir == "" {
			dir = "."
		}
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() && (runtime.GOOS == "windows" || st.Mode()&0o111 != 0) {
			return p, nil
		}
	}
	return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
}

func firstLine(s, def string) string {
	for l := range strings.SplitSeq(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return def
}
