package ctl

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed skill/SKILL.md
var skillMD []byte

// DefaultSkillDir returns where `skill install` writes by default.
func DefaultSkillDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "skills", "dageki"), nil
}

// InstallSkill writes SKILL.md into dir and returns the file path.
func InstallSkill(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, skillMD, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func runSkill(args []string, stdout, stderr func(string)) int {
	if len(args) < 1 || len(args) > 2 || args[0] != "install" {
		stderr("usage: dgk skill install [書き出し先ディレクトリ]")
		return 2
	}
	var dir string
	if len(args) == 2 {
		dir = args[1]
	} else {
		d, err := DefaultSkillDir()
		if err != nil {
			stderr(err.Error())
			return 1
		}
		dir = d
	}
	path, err := InstallSkill(dir)
	if err != nil {
		stderr(err.Error())
		return 1
	}
	stdout(fmt.Sprintf("書き出しました: %s", path))
	return 0
}
