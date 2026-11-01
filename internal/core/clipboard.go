package core

import (
	"os"
	"os/exec"
	"strings"
)

type clipTool struct {
	copy  []string
	paste []string
	env   string // required environment variable, if any
}

var clipTools = []clipTool{
	{copy: []string{"wl-copy"}, paste: []string{"wl-paste", "-n"}, env: "WAYLAND_DISPLAY"},
	{copy: []string{"xclip", "-selection", "clipboard"}, paste: []string{"xclip", "-o", "-selection", "clipboard"}, env: "DISPLAY"},
	{copy: []string{"xsel", "-ib"}, paste: []string{"xsel", "-ob"}, env: "DISPLAY"},
	{copy: []string{"pbcopy"}, paste: []string{"pbpaste"}},
}

func findClipTool() *clipTool {
	for i := range clipTools {
		t := &clipTools[i]
		if t.env != "" && os.Getenv(t.env) == "" {
			continue
		}
		if _, err := exec.LookPath(t.copy[0]); err == nil {
			return t
		}
	}
	return nil
}

// WriteSystemClipboard copies s to the OS clipboard when a helper tool is
// available. Failures are ignored; the in-app clipboard is always updated.
func WriteSystemClipboard(s string) {
	t := findClipTool()
	if t == nil {
		return
	}
	cmd := exec.Command(t.copy[0], t.copy[1:]...)
	cmd.Stdin = strings.NewReader(s)
	_ = cmd.Start()
	go cmd.Wait()
}

func ReadSystemClipboard() (string, bool) {
	t := findClipTool()
	if t == nil {
		return "", false
	}
	out, err := exec.Command(t.paste[0], t.paste[1:]...).Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}
