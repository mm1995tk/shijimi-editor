package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// muxProcessNames maps the process name of a multiplexer to its name in
// [terminal] multiplexer. A name matches when the process name starts with it
// ("tmux: server" is how Linux shows the tmux server).
var muxProcessNames = []struct{ prefix, kind string }{
	{"herdr", "herdr"},
	{"tmux", "tmux"},
	{"zellij", "zellij"},
	{"cmux", "cmux"},
}

func muxKindOf(comm string) string {
	comm = filepath.Base(strings.TrimSpace(comm))
	for _, n := range muxProcessNames {
		if strings.HasPrefix(comm, n.prefix) {
			return n.kind
		}
	}
	return ""
}

// detectMultiplexer walks up the parent processes of pid and returns the
// first multiplexer found, which is the innermost one, or "" when there is
// none. Environment variables cannot tell which of several nested
// multiplexers is the inner one. parents returns the parent pid and the name of
// a process; nil means the system's (/proc, or ps).
func detectMultiplexer(pid int, parents func(pid int) (int, string, bool)) string {
	if parents == nil {
		parents = systemParent
	}
	seen := map[int]bool{}
	for pid > 1 && !seen[pid] {
		seen[pid] = true
		ppid, _, ok := parents(pid)
		if !ok {
			return ""
		}
		_, comm, ok := parents(ppid)
		if !ok {
			return ""
		}
		if k := muxKindOf(comm); k != "" {
			return k
		}
		pid = ppid
	}
	return ""
}

// systemParent returns the parent pid and the name of process pid.
func systemParent(pid int) (int, string, bool) {
	if b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat"); err == nil {
		// pid (comm) state ppid ...; comm may contain spaces and parentheses.
		s := string(b)
		l, r := strings.IndexByte(s, '('), strings.LastIndexByte(s, ')')
		if l >= 0 && r > l {
			if f := strings.Fields(s[r+1:]); len(f) >= 2 {
				if ppid, err := strconv.Atoi(f[1]); err == nil {
					return ppid, s[l+1 : r], true
				}
			}
		}
	}
	// macOS and others: ps -o ppid=,comm= -p pid
	out, err := exec.Command("ps", "-o", "ppid=,comm=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, "", false
	}
	line := strings.TrimSpace(string(out))
	sp := strings.IndexAny(line, " \t")
	if sp < 0 {
		return 0, "", false
	}
	ppid, err := strconv.Atoi(line[:sp])
	if err != nil {
		return 0, "", false
	}
	return ppid, strings.TrimSpace(line[sp:]), true
}
