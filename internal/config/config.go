// Package config loads shijimi settings. Language and language-server
// definitions use a format compatible with Helix's languages.toml, so an
// existing Helix `languages.toml` works when copied to `~/.config/shijimi-editor/`.
package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// Server describes how to start a language server.
type Server struct {
	Name    string
	Command string
	Args    []string
	Env     map[string]string
	Config  any // sent as initializationOptions and workspace/configuration
}

// Language describes one language and the servers used for it.
type Language struct {
	Name       string
	LanguageID string
	FileTypes  []string // extensions or file names
	Globs      []string
	Roots      []string
	Servers    []string
	IndentUnit string
	// Formatter, when set, is what :fmt runs instead of the language server.
	Formatter *Formatter
}

// Formatter is an external command that formats a buffer over stdin/stdout
// (Helix's `formatter = { command, args }`). "%{buffer_name}" in Args is
// replaced with the file path relative to the project root.
type Formatter struct {
	Command string
	Args    []string
}

// Editor holds editor settings (subset of Helix's config.toml [editor]).
type Editor struct {
	Keymap     string // "helix" (default) or "vscode"
	LineNumber string // "absolute" or "relative"
	FileIcons  bool   // Nerd Font icons in the file tree (default true)
	Theme      string // theme name; "" = default (github-dark)

	// UILanguage is the language of the UI text: "en" (default) or "ja".
	// It is set by ui-language in config.toml.
	UILanguage string

	// AutoSave mirrors Helix's [editor.auto-save.after-delay]; unlike Helix
	// it is enabled by default with a 1 second timeout.
	AutoSave      bool
	AutoSaveDelay time.Duration
	// OnSwitch is [editor.auto-save] on-switch: what a workspace does with its
	// open files when `shj serve` switches it to another folder, "save" or
	// "discard". Auto-save being off, it has no default: Load reports a missing
	// one as a mistake. It is "" while auto-save is on and the key is absent.
	OnSwitch string

	// WatchFiles reloads open files that another process (an AI agent, git)
	// changed on disk. Default true.
	WatchFiles bool

	// MouseHover shows the LSP hover when the pointer rests on a word in the
	// editor. Default true.
	MouseHover bool

	// IndentGuides draws a faint vertical line at each indent level in the
	// editor ([editor.indent-guides] render). Default true.
	IndentGuides bool

	// MermaidCommand renders a mermaid diagram to a PNG for the Markdown
	// preview. {in} and {out} are replaced by file paths; run through `sh -c`.
	// Empty disables diagram rendering (the source is shown instead).
	MermaidCommand string
}

// Search holds default globs for project-wide search (`:grep`).
type Search struct {
	Exclude []string
	Include []string
}

// Browser holds the settings of `:browser forward` (port forwarding).
type Browser struct {
	// ListenHost is the address `:browser forward` listens on. Set it to e.g.
	// "0.0.0.0" only on a trusted network.
	ListenHost string
}

// Agent is the ACP agent (Agent Client Protocol) the suggestion panel runs.
// It is read only from the user's own config directory, never from a
// project: a cloned repository must not choose a program to run.
type Agent struct {
	Command string
	Args    []string
	Env     map[string]string
}

// Keys holds the key bindings of config.toml's [keys.normal], [keys.insert]
// and [keys.select]: Helix-style key names mapped to a command name
// ("move_char_left") or a ":" command (":write"). Like Agent, it is read only
// from the user's own config directory: a ":sh ..." binding in a cloned
// repository's config would run a program the user never chose.
type Keys struct {
	Normal map[string]string
	Insert map[string]string
	Select map[string]string
}

// Experimental is the [experimental] section of config.toml: switches for
// features that are being tried out before they reach the stable channel.
// Each key defaults to Default.
type Experimental struct {
	// Default is the value of every key the configuration files do not set:
	// true on the nightly channel, false on the stable channel. A nightly user
	// sees the features without turning them on; a stable user does not see
	// them without asking.
	Default bool
	// Multiplexer makes the terminal commands (Ctrl+`, :sh, ...) use a pane of
	// the terminal multiplexer the editor runs in instead of the built-in
	// terminal panel; Terminal.Multiplexer chooses which one. The old key
	// "herdr" is read as the same switch.
	Multiplexer bool
}

// Terminal is the [terminal] section of config.toml.
type Terminal struct {
	// Multiplexer is the multiplexer whose panes replace the built-in terminal
	// while Experimental.Multiplexer is on: "auto" (the innermost one the
	// editor runs in), "herdr", "tmux", "zellij", "cmux", or "none" (always
	// the built-in terminal).
	Multiplexer string
}

// Serve is the [server] section of config.toml, read by `shj serve`.
type Serve struct {
	// IdleStopDays is how many days `shj serve` waits with no GUI connected
	// before it stops. 0 means it never stops.
	IdleStopDays int
	// StartDir is the folder a new workspace's shell starts in, as an absolute
	// path (a leading ~ is expanded). "" means the home directory.
	StartDir string
}

// Multiplexers are the values of [terminal] multiplexer.
var Multiplexers = []string{"auto", "herdr", "tmux", "zellij", "cmux", "none"}

// Config is the merged configuration.
type Config struct {
	Editor       Editor
	Keys         Keys
	Search       Search
	Browser      Browser
	Agent        Agent
	Terminal     Terminal
	Serve        Serve
	Experimental Experimental
	Languages    map[string]*Language
	Servers      map[string]*Server
	Sources      []string // files that were loaded, for display
}

// nixEvalTimeout bounds one `nix eval`.
const nixEvalTimeout = 15 * time.Second

// Load merges built-in defaults, the user's `~/.config/shijimi-editor/`
// (`~/.config/shijimi-editor-nightly/` for a nightly build, see Options and userDir), and
// the project's `.shijimi-editor/` directory (later ones win). Helix's own config
// directories are not read. In each directory
// `config.nix` / `languages.nix` are evaluated with `nix eval --json` and
// applied after the TOML file of the same name.
//
// A mistake in a file (TOML syntax, a key that is not read, a value of the
// wrong type, a Nix file that does not evaluate) is an *Error that lists every
// mistake with its file, line and reason. The Config returned with it is
// still usable for what it needs before it reports (the UI language), but the
// caller must not run with it.
func Load(root string, o Options) (*Config, error) {
	c := &Config{
		Browser:      Browser{ListenHost: "127.0.0.1"},
		Experimental: Experimental{Default: o.Experimental, Multiplexer: o.Experimental},
		Terminal:     Terminal{Multiplexer: "auto"},
		Serve:        Serve{IdleStopDays: 7},
		Editor:       Editor{UILanguage: i18n.Default, Keymap: "helix", LineNumber: "absolute", FileIcons: true, AutoSave: true, AutoSaveDelay: time.Second, WatchFiles: true, MouseHover: true, IndentGuides: true, MermaidCommand: "mmdc -q -i {in} -o {out} -b transparent -s 2"},
		Languages:    map[string]*Language{},
		Servers:      map[string]*Server{},
	}
	var probs []Problem
	if err := c.mergeLanguages(defaultLanguages, ""); err != nil {
		// the built-in table is part of the program; a mistake in it is a bug
		panic(err)
	}

	// user marks the user's own directory, the only one [agent] and [keys] are read from.
	type configDir struct {
		path string
		user bool
	}
	// the files of a directory, in the order they are applied: a Nix file after the TOML of the same name
	files := []struct {
		name   string
		schema map[string]field
		read   func(path string, schema map[string]field, user bool) (map[string]any, []Problem, bool)
		config bool // config.*, otherwise languages.*
	}{
		{"config.toml", configSchema, readTOML, true},
		{"config.nix", configSchema, readNix, true},
		{"languages.toml", languagesSchema, readTOML, false},
		{"languages.nix", languagesSchema, readNix, false},
	}
	// autoSaveOff is the file that last turned auto-save off.
	var autoSaveOff string
	var dirs []configDir
	if d, ok := userConfigDir(); ok {
		dirs = append(dirs, configDir{userDir(d, o), true})
	}
	dirs = append(dirs, configDir{filepath.Join(root, ".shijimi-editor"), false})
	for _, dir := range dirs {
		for _, f := range files {
			path := filepath.Join(dir.path, f.name)
			raw, ps, ok := f.read(path, f.schema, dir.user)
			probs = append(probs, ps...)
			if !ok {
				continue
			}
			if f.config {
				if autoSaveEnable(raw) == triFalse {
					autoSaveOff = path
				} else if autoSaveEnable(raw) == triTrue {
					autoSaveOff = ""
				}
				c.mergeConfigRaw(raw, dir.user)
				c.Sources = append(c.Sources, path)
			} else {
				c.mergeLanguagesRaw(raw, path)
			}
		}
	}
	if p, ok := c.missingOnSwitch(autoSaveOff); ok {
		probs = append(probs, p)
	}
	if len(probs) > 0 {
		return c, &Error{Problems: probs}
	}
	return c, nil
}

// missingOnSwitch reports the missing on-switch: with auto-save off, `shj serve`
// has no answer to what a workspace does with the open files when it switches
// folders, and it does not choose one (it would lose the edits or write files
// the user did not mean to). off is the file that turned auto-save off.
func (c *Config) missingOnSwitch(off string) (Problem, bool) {
	if c.Editor.AutoSave || c.Editor.OnSwitch != "" {
		return Problem{}, false
	}
	p := Problem{File: off, Key: "editor.auto-save.on-switch", code: "config.err.onswitch"}
	p.Line = lineOf(off, []string{"editor", "auto-save", "after-delay", "enable"})
	return p, true
}

// lineOf is the line of the key at path in a TOML file; 0 when unknown (a Nix
// file has no lines).
func lineOf(file string, path []string) int {
	if filepath.Ext(file) != ".toml" {
		return 0
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return 0
	}
	return indexLines(string(data)).find(path)
}

// mergeConfigRaw applies a decoded config.toml / config.nix. user is whether
// it is from the user's own directory.
func (c *Config) mergeConfigRaw(raw map[string]any, user bool) {
	c.mergeEditorRaw(raw)
	c.mergeBrowserRaw(raw)
	c.mergeExperimentalRaw(raw)
	c.mergeTerminalRaw(raw)
	if user {
		c.mergeAgentRaw(raw)
		c.mergeServeRaw(raw)
		c.mergeKeysRaw(raw)
	}
}

// tomlPrefix is the place the decoder puts at the start of its messages.
var tomlPrefix = regexp.MustCompile(`^toml: (line \d+ ?)?(\(last key "[^"]*"\))?:? ?`)

// readTOML reads and checks a TOML file. found is false when the file does not
// exist or cannot be decoded; the problems are returned either way.
func readTOML(path string, schema map[string]field, user bool) (raw map[string]any, probs []Problem, found bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, false
		}
		return nil, []Problem{{File: path, code: "config.err.read", args: []any{err.Error()}}}, false
	}
	if _, err := toml.Decode(string(data), &raw); err != nil {
		p := Problem{File: path, code: "config.err.syntax", args: []any{err.Error()}}
		if pe, ok := errors.AsType[toml.ParseError](err); ok {
			p.Line = pe.Position.Line
			msg := pe.Message
			if msg == "" {
				msg = tomlPrefix.ReplaceAllString(err.Error(), "")
			}
			// The decoder counts the newline that ended the line as part of the
			// next one when it was not expected there.
			if p.Line > 1 && strings.Contains(msg, `'\n'`) {
				p.Line--
			}
			p.args = []any{msg}
		}
		return nil, []Problem{p}, false
	}
	return raw, validate(raw, schema, path, string(data), user), true
}

// readNix evaluates a Nix file and checks the result. found is false when the
// file does not exist or does not evaluate.
func readNix(path string, schema map[string]field, user bool) (raw map[string]any, probs []Problem, found bool) {
	raw, exists, err := evalNix(path)
	if !exists {
		return nil, nil, false
	}
	if errors.Is(err, errNixTop) {
		return nil, []Problem{{File: path, code: "config.err.nixtop"}}, false
	}
	if err != nil {
		return nil, []Problem{{File: path, code: "config.err.nix", args: []any{err.Error()}}}, false
	}
	return raw, validate(raw, schema, path, "", user), true
}

// UserDir returns the user's own configuration directory, such as
// ~/.config/shijimi-editor, for messages that tell where to write a setting.
func UserDir(o Options) (string, bool) {
	d, ok := userConfigDir()
	if !ok {
		return "", false
	}
	return userDir(d, o), true
}

// userDir returns the user's own configuration directory under the config
// home: `shijimi-editor`, or `shijimi-editor-nightly` for a nightly build. A
// nightly build that has no `shijimi-editor-nightly` directory reads
// `shijimi-editor` instead, so that trying the nightly needs no copying of the
// settings; creating the directory makes the two builds independent.
func userDir(configHome string, o Options) string {
	d := filepath.Join(configHome, "shijimi-editor"+o.DirSuffix)
	if o.DirSuffix != "" {
		if st, err := os.Stat(d); err != nil || !st.IsDir() {
			return filepath.Join(configHome, "shijimi-editor")
		}
	}
	return d
}

// userConfigDir returns $XDG_CONFIG_HOME, or ~/.config when it is unset or not
// an absolute path, on every OS. Unlike os.UserConfigDir it does not use ~/Library/Application
// Support on macOS, so it matches Helix and the files the home-manager module
// writes with xdg.configFile.
func userConfigDir() (string, bool) {
	if d := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(d) {
		return d, true
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	return filepath.Join(home, ".config"), true
}

// evalNix evaluates a Nix file to JSON and returns it in the shape the TOML
// decoder produces, so both formats share one merge path. exists is false when
// there is no such file; err is why a file that exists could not be
// evaluated (nix is not installed, evaluation fails, the result is not an
// attribute set).
// errNixTop is the result of a Nix file that is not an attribute set.
var errNixTop = errors.New("not an attribute set")

func evalNix(path string) (raw map[string]any, exists bool, err error) {
	if _, err := os.Stat(path); err != nil {
		return nil, false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), nixEvalTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "nix", "--extra-experimental-features", "nix-command",
		"eval", "--json", "--file", path).Output()
	if err != nil {
		if ee, ok := errors.AsType[*exec.ExitError](err); ok {
			return nil, true, errors.New(lastLines(string(ee.Stderr)))
		}
		if ctx.Err() != nil {
			return nil, true, fmt.Errorf("nix eval did not finish in %s", nixEvalTimeout)
		}
		return nil, true, err
	}
	if err := json.Unmarshal(out, &raw); err != nil || raw == nil {
		return nil, true, errNixTop
	}
	for k, v := range raw {
		raw[k] = normalizeJSON(v)
	}
	return raw, true, nil
}

// lastLines is the end of a nix error message, where the reason is.
func lastLines(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > 3 {
		lines = lines[len(lines)-3:]
	}
	return strings.TrimSpace(strings.Join(lines, " "))
}

// normalizeJSON turns integral JSON numbers into int64, as the TOML decoder does.
func normalizeJSON(v any) any {
	switch v := v.(type) {
	case map[string]any:
		for k, x := range v {
			v[k] = normalizeJSON(x)
		}
	case []any:
		for i, x := range v {
			v[i] = normalizeJSON(x)
		}
	case float64:
		if v == float64(int64(v)) {
			return int64(v)
		}
	}
	return v
}

func (c *Config) mergeBrowserRaw(raw map[string]any) {
	if br, ok := raw["browser"].(map[string]any); ok {
		if v, ok := br["listen-host"].(string); ok && v != "" {
			c.Browser.ListenHost = v
		}
	}
}

func (c *Config) mergeExperimentalRaw(raw map[string]any) {
	if ex, ok := raw["experimental"].(map[string]any); ok {
		// "herdr" is the old name of "multiplexer"; the new one wins when both are set.
		if v, ok := ex["herdr"].(bool); ok {
			c.Experimental.Multiplexer = v
		}
		if v, ok := ex["multiplexer"].(bool); ok {
			c.Experimental.Multiplexer = v
		}
	}
}

func (c *Config) mergeTerminalRaw(raw map[string]any) {
	if t, ok := raw["terminal"].(map[string]any); ok {
		if v, ok := t["multiplexer"].(string); ok {
			v = strings.ToLower(strings.TrimSpace(v))
			if slices.Contains(Multiplexers, v) {
				c.Terminal.Multiplexer = v
			}
		}
	}
}

func (c *Config) mergeServeRaw(raw map[string]any) {
	if t, ok := raw["server"].(map[string]any); ok {
		if v, ok := t["start_dir"].(string); ok {
			if dir, ok := ExpandStartDir(v); ok {
				c.Serve.StartDir = dir
			}
		}
		switch v := t["idle_stop_days"].(type) {
		case int64:
			if v >= 0 {
				c.Serve.IdleStopDays = int(v)
			}
		case float64:
			if v >= 0 && v == float64(int(v)) {
				c.Serve.IdleStopDays = int(v)
			}
		}
	}
}

func (c *Config) mergeKeysRaw(raw map[string]any) {
	keys, ok := raw["keys"].(map[string]any)
	if !ok {
		return
	}
	for _, mode := range []string{"normal", "insert", "select"} {
		tbl, ok := keys[mode].(map[string]any)
		if !ok {
			continue
		}
		dst := &c.Keys.Normal
		switch mode {
		case "insert":
			dst = &c.Keys.Insert
		case "select":
			dst = &c.Keys.Select
		}
		for k, v := range tbl {
			s, ok := v.(string)
			if !ok {
				continue
			}
			if *dst == nil {
				*dst = map[string]string{}
			}
			(*dst)[k] = strings.TrimSpace(s)
		}
	}
}

func (c *Config) mergeAgentRaw(raw map[string]any) {
	ag, ok := raw["agent"].(map[string]any)
	if !ok {
		return
	}
	if v, ok := ag["command"].(string); ok {
		c.Agent.Command = v
	}
	if v, ok := ag["args"]; ok {
		c.Agent.Args = stringList(v)
	}
	if env, ok := ag["environment"].(map[string]any); ok {
		c.Agent.Env = map[string]string{}
		for k, v := range env {
			if s, ok := v.(string); ok {
				c.Agent.Env[k] = s
			}
		}
	}
}

func (c *Config) mergeEditorRaw(raw map[string]any) {
	if v, ok := raw["keymap"].(string); ok {
		c.Editor.Keymap = v
	}
	if sr, ok := raw["search"].(map[string]any); ok {
		if v, ok := sr["exclude"]; ok {
			c.Search.Exclude = stringList(v)
		}
		if v, ok := sr["include"]; ok {
			c.Search.Include = stringList(v)
		}
	}
	ed, _ := raw["editor"].(map[string]any)
	if v, ok := ed["line-number"].(string); ok {
		c.Editor.LineNumber = v
	}
	if v, ok := ed["file-icons"].(bool); ok {
		c.Editor.FileIcons = v
	}
	if v, ok := ed["mermaid-command"].(string); ok {
		c.Editor.MermaidCommand = v
	}
	if v, ok := ed["watch-files"].(bool); ok {
		c.Editor.WatchFiles = v
	}
	if v, ok := ed["mouse-hover"].(bool); ok {
		c.Editor.MouseHover = v
	}
	if ig, ok := ed["indent-guides"].(map[string]any); ok {
		if v, ok := ig["render"].(bool); ok {
			c.Editor.IndentGuides = v
		}
	}
	if v, ok := ed["keymap"].(string); ok {
		c.Editor.Keymap = v
	}
	if as, ok := ed["auto-save"].(map[string]any); ok {
		if v, ok := as["on-switch"].(string); ok && (v == "save" || v == "discard") {
			c.Editor.OnSwitch = v
		}
		if ad, ok := as["after-delay"].(map[string]any); ok {
			if v, ok := ad["enable"].(bool); ok {
				c.Editor.AutoSave = v
			}
			if v, ok := ad["timeout"].(int64); ok && v > 0 {
				c.Editor.AutoSaveDelay = time.Duration(v) * time.Millisecond
			}
		}
	}
	if v, ok := raw["theme"].(string); ok {
		c.Editor.Theme = v
	}
	if v, ok := raw["ui-language"].(string); ok && i18n.Valid(v) {
		c.Editor.UILanguage = v
	}
	if v, ok := ed["theme"].(string); ok {
		c.Editor.Theme = v
	}
}

func stringList(v any) []string {
	list, _ := v.([]any)
	var out []string
	for _, x := range list {
		if s, ok := x.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// mergeLanguages applies a languages.toml given as text.
func (c *Config) mergeLanguages(data, source string) error {
	var raw map[string]any
	if _, err := toml.Decode(data, &raw); err != nil {
		return err
	}
	c.mergeLanguagesRaw(raw, source)
	return nil
}

// mergeConfig merges a language server's config key by key; values in over
// win. Nested tables are merged recursively, and anything else (including
// arrays) is replaced.
func mergeConfig(base, over any) any {
	b, ok1 := base.(map[string]any)
	o, ok2 := over.(map[string]any)
	if !ok1 || !ok2 {
		return over
	}
	out := make(map[string]any, len(b)+len(o))
	maps.Copy(out, b)
	for k, v := range o {
		if old, ok := out[k]; ok {
			out[k] = mergeConfig(old, v)
		} else {
			out[k] = v
		}
	}
	return out
}

func (c *Config) mergeLanguagesRaw(raw map[string]any, source string) {
	if source != "" {
		c.Sources = append(c.Sources, source)
	}
	if servers, ok := raw["language-server"].(map[string]any); ok {
		for name, v := range servers {
			m, _ := v.(map[string]any)
			s := c.Servers[name]
			if s == nil {
				s = &Server{Name: name}
				c.Servers[name] = s
			}
			if cmd, ok := m["command"].(string); ok {
				s.Command = cmd
			}
			if args, ok := m["args"]; ok {
				s.Args = strings_(args)
			}
			if env, ok := m["environment"].(map[string]any); ok {
				s.Env = map[string]string{}
				for k, v := range env {
					if sv, ok := v.(string); ok {
						s.Env[k] = sv
					}
				}
			}
			if cfg, ok := m["config"]; ok {
				s.Config = mergeConfig(s.Config, cfg)
			}
		}
	}
	langs, _ := asList(raw["language"])
	for _, item := range langs {
		m, _ := item.(map[string]any)
		name, _ := m["name"].(string)
		if name == "" {
			continue
		}
		l := c.Languages[name]
		if l == nil {
			l = &Language{Name: name}
			c.Languages[name] = l
		}
		if v, ok := m["language-id"].(string); ok {
			l.LanguageID = v
		}
		if v, ok := m["file-types"].([]any); ok {
			l.FileTypes, l.Globs = nil, nil
			for _, ft := range v {
				switch ft := ft.(type) {
				case string:
					l.FileTypes = append(l.FileTypes, ft)
				case map[string]any:
					if g, ok := ft["glob"].(string); ok {
						l.Globs = append(l.Globs, g)
					}
				}
			}
		}
		if v, ok := m["roots"]; ok {
			l.Roots = strings_(v)
		}
		if v, ok := m["language-servers"].([]any); ok {
			l.Servers = nil
			for _, s := range v {
				switch s := s.(type) {
				case string:
					l.Servers = append(l.Servers, s)
				case map[string]any:
					if n, ok := s["name"].(string); ok {
						l.Servers = append(l.Servers, n)
					}
				}
			}
		}
		if f, ok := m["formatter"].(map[string]any); ok {
			if cmd, _ := f["command"].(string); cmd != "" {
				l.Formatter = &Formatter{Command: cmd, Args: strings_(f["args"])}
			}
		}
		if ind, ok := m["indent"].(map[string]any); ok {
			if u, ok := ind["unit"].(string); ok {
				l.IndentUnit = u
			}
		}
	}
}

func strings_(v any) []string {
	var out []string
	switch v := v.(type) {
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
	case []string:
		out = v
	}
	return out
}

// LanguageFor returns the language for a file path, or nil.
func (c *Config) LanguageFor(path string) *Language {
	base := filepath.Base(path)
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	names := make([]string, 0, len(c.Languages))
	for n := range c.Languages {
		names = append(names, n)
	}
	sort.Strings(names)
	// exact file names beat extensions
	for _, n := range names {
		if slices.Contains(c.Languages[n].FileTypes, base) {
			return c.Languages[n]
		}
	}
	for _, n := range names {
		l := c.Languages[n]
		for _, ft := range l.FileTypes {
			if ft == ext && ext != "" {
				return l
			}
		}
		for _, g := range l.Globs {
			if ok, _ := filepath.Match(g, base); ok {
				return l
			}
			if ok, _ := filepath.Match(g, path); ok {
				return l
			}
		}
	}
	return nil
}

// ID is the LSP languageId.
func (l *Language) ID() string {
	if l.LanguageID != "" {
		return l.LanguageID
	}
	return l.Name
}

// FindRoot walks up from file looking for one of the language's root
// markers, stopping at workspace. It returns workspace when none is found.
func (l *Language) FindRoot(file, workspace string) string {
	dir := filepath.Dir(file)
	for {
		for _, m := range l.Roots {
			if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
				return dir
			}
		}
		if dir == workspace || dir == filepath.Dir(dir) {
			return workspace
		}
		dir = filepath.Dir(dir)
	}
}

// defaultLanguages mirrors a small part of Helix's built-in languages.toml.
const defaultLanguages = `
[language-server.gopls]
command = "gopls"
config = { semanticTokens = true }

[language-server.rust-analyzer]
command = "rust-analyzer"

[language-server.typescript-language-server]
command = "typescript-language-server"
args = ["--stdio"]

[language-server.pylsp]
command = "pylsp"

[language-server.clangd]
command = "clangd"

[language-server.nil]
command = "nil"

[language-server.haskell-language-server]
command = "haskell-language-server-wrapper"
args = ["--lsp"]

[language-server.lua-language-server]
command = "lua-language-server"

[language-server.bash-language-server]
command = "bash-language-server"
args = ["start"]

[language-server.zls]
command = "zls"

[language-server.taplo]
command = "taplo"
args = ["lsp", "stdio"]

[language-server.yaml-language-server]
command = "yaml-language-server"
args = ["--stdio"]

[language-server.vscode-json-language-server]
command = "vscode-json-language-server"
args = ["--stdio"]

[[language]]
name = "go"
file-types = ["go"]
roots = ["go.work", "go.mod"]
language-servers = ["gopls"]

[[language]]
name = "rust"
file-types = ["rs"]
roots = ["Cargo.toml", "Cargo.lock"]
language-servers = ["rust-analyzer"]

[[language]]
name = "typescript"
file-types = ["ts", "mts", "cts"]
roots = ["package.json", "tsconfig.json"]
language-servers = ["typescript-language-server"]

[[language]]
name = "tsx"
language-id = "typescriptreact"
file-types = ["tsx"]
roots = ["package.json", "tsconfig.json"]
language-servers = ["typescript-language-server"]

[[language]]
name = "javascript"
file-types = ["js", "mjs", "cjs"]
roots = ["package.json"]
language-servers = ["typescript-language-server"]

[[language]]
name = "jsx"
language-id = "javascriptreact"
file-types = ["jsx"]
roots = ["package.json"]
language-servers = ["typescript-language-server"]

[[language]]
name = "python"
file-types = ["py", "pyi"]
roots = ["pyproject.toml", "setup.py", "requirements.txt"]
language-servers = ["pylsp"]

[[language]]
name = "c"
file-types = ["c", "h"]
roots = ["compile_commands.json", ".clangd"]
language-servers = ["clangd"]

[[language]]
name = "cpp"
file-types = ["cpp", "cc", "cxx", "hpp", "hh", "hxx"]
roots = ["compile_commands.json", ".clangd"]
language-servers = ["clangd"]

[[language]]
name = "nix"
file-types = ["nix"]
language-servers = ["nil"]

[[language]]
name = "haskell"
file-types = ["hs", "hs-boot"]
roots = ["stack.yaml", "cabal.project", "package.yaml", "hie.yaml"]
language-servers = ["haskell-language-server"]

[[language]]
name = "lua"
file-types = ["lua"]
roots = [".luarc.json", ".stylua.toml"]
language-servers = ["lua-language-server"]

[[language]]
name = "bash"
file-types = ["sh", "bash", "zsh"]
language-servers = ["bash-language-server"]

[[language]]
name = "zig"
file-types = ["zig"]
roots = ["build.zig"]
language-servers = ["zls"]

[[language]]
name = "toml"
file-types = ["toml"]
language-servers = ["taplo"]

[[language]]
name = "yaml"
file-types = ["yaml", "yml"]
language-servers = ["yaml-language-server"]

[[language]]
name = "json"
file-types = ["json", "jsonc"]
language-servers = ["vscode-json-language-server"]
`

type tri int

const (
	triUnset tri = iota
	triTrue
	triFalse
)

// autoSaveEnable is what a decoded config says for [editor.auto-save.after-delay] enable.
func autoSaveEnable(raw map[string]any) tri {
	ed, _ := raw["editor"].(map[string]any)
	as, _ := ed["auto-save"].(map[string]any)
	ad, _ := as["after-delay"].(map[string]any)
	switch v, _ := ad["enable"].(bool); {
	case ad["enable"] == nil:
		return triUnset
	case v:
		return triTrue
	}
	return triFalse
}

// ExpandStartDir turns a [server] start_dir into an absolute path: ~ and ~/x
// are the home directory. ok is false when it is neither that nor an absolute
// path.
func ExpandStartDir(v string) (dir string, ok bool) {
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
