package server

import "os"

// shellCwd はプロセス pid の cwd を読む。
func shellCwd(pid int) (string, error) {
	return os.Readlink("/proc/" + itoa(pid) + "/cwd")
}
