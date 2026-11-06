package config

import (
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/mm1995tk/shijimi-editor/core/config"
)

// kisuiSchema is the keys of config.toml that kisui-terminal reads, not shj:
// [server] (kisui serve) and [[gui.machines]] (the window). kisui-terminal
// shares the file with shj, so shj knows them too: it checks them (a mistake
// is reported, as for its own keys) and does not warn that nobody reads
// them. What they mean is in the README of kisui-terminal
// (https://github.com/mm1995tk/kisui-terminal).
var kisuiSchema = map[string]config.Field{
	"server": config.Table(map[string]config.Field{
		"idle_stop_days": config.Int.Check(config.NonNegative),
		"start_dir":      config.String.Check(checkStartDir),
	}).UserOnly(),
	"gui": config.Table(map[string]config.Field{
		"machines": config.Tables([]string{"ssh"}, map[string]config.Field{"ssh": config.String, "name": config.String}),
	}).UserOnly(),
}

// expandStartDir turns a [server] start_dir into an absolute path: ~ and ~/x
// are the home directory. ok is false when it is neither that nor an absolute
// path.
func expandStartDir(v string) (dir string, ok bool) {
	switch {
	case v == "~" || strings.HasPrefix(v, "~/"):
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		return filepath.Join(home, v[1:]), true
	case filepath.IsAbs(v):
		return filepath.Clean(v), true
	}
	return "", false
}

func checkStartDir(v any) (string, []any) {
	s, _ := v.(string)
	dir, ok := expandStartDir(s)
	if !ok {
		return "config.err.abspath", nil
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return "config.err.notdir", []any{dir}
	}
	return "", nil
}

// withKisui adds kisuiSchema to the UI's own keys.
func withKisui(schema map[string]config.Field) map[string]config.Field {
	maps.Copy(schema, kisuiSchema)
	return schema
}
