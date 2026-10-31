// Package config loads shijimi settings. Language and language-server
// definitions use a format compatible with Helix's languages.toml, so an
// existing Helix `languages.toml` works when copied to `~/.config/shijimi-editor/`.
package config

import (
	"context"
	"encoding/json"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/mm1995tk/shijimi-editor/internal/channel"
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
	// Invalid lists entries that could not be read (a value that is not a
	// string, such as a Helix sub-table), as "keys.normal.g".
	Invalid []string
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
	Experimental Experimental
	Languages    map[string]*Language
	Servers      map[string]*Server
	Sources      []string // files that were loaded, for display
}

// nixEvalTimeout bounds one `nix eval`.
const nixEvalTimeout = 15 * time.Second

// Load merges built-in defaults, the user's `~/.config/shijimi-editor/`
// (`~/.config/shijimi-editor-nightly/` for a nightly build, see userDir), and
// the project's `.shijimi-editor/` directory (later ones win). Helix's own config
// directories are not read. In each directory
// `config.nix` / `languages.nix` are evaluated with `nix eval --json` and
// applied after the TOML file of the same name.
func Load(root string) *Config {
	c := &Config{
		Browser:      Browser{ListenHost: "127.0.0.1"},
		Experimental: Experimental{Default: channel.IsNightly(), Multiplexer: channel.IsNightly()},
		Terminal:     Terminal{Multiplexer: "auto"},
		Editor:       Editor{UILanguage: i18n.Default, Keymap: "helix", LineNumber: "absolute", FileIcons: true, AutoSave: true, AutoSaveDelay: time.Second, WatchFiles: true, MouseHover: true, IndentGuides: true, MermaidCommand: "mmdc -q -i {in} -o {out} -b transparent -s 2"},
		Languages:    map[string]*Language{},
		Servers:      map[string]*Server{},
	}
	c.mergeLanguages(defaultLanguages, "")

	// user marks the user's own directory, the only one [agent] and [keys] are read from.
	type configDir struct {
		path string
		user bool
	}
	var dirs []configDir
	if d, ok := userConfigDir(); ok {
		dirs = append(dirs, configDir{userDir(d), true})
	}
	dirs = append(dirs, configDir{filepath.Join(root, ".shijimi-editor"), false})
	for _, dir := range dirs {
		d, user := dir.path, dir.user
		if data, err := os.ReadFile(filepath.Join(d, "config.toml")); err == nil {
			c.mergeEditor(string(data))
			c.mergeBrowser(string(data))
			c.mergeExperimental(string(data))
			if user {
				c.mergeAgent(string(data))
				c.mergeKeys(string(data))
			}
			c.Sources = append(c.Sources, filepath.Join(d, "config.toml"))
		}
		if raw, ok := evalNix(filepath.Join(d, "config.nix")); ok {
			c.mergeEditorRaw(raw)
			c.mergeBrowserRaw(raw)
			c.mergeExperimentalRaw(raw)
			if user {
				c.mergeAgentRaw(raw)
				c.mergeKeysRaw(raw)
			}
			c.Sources = append(c.Sources, filepath.Join(d, "config.nix"))
		}
		if data, err := os.ReadFile(filepath.Join(d, "languages.toml")); err == nil {
			c.mergeLanguages(string(data), filepath.Join(d, "languages.toml"))
		}
		if raw, ok := evalNix(filepath.Join(d, "languages.nix")); ok {
			c.mergeLanguagesRaw(raw, filepath.Join(d, "languages.nix"))
		}
	}
	return c
}

// UserDir returns the user's own configuration directory, such as
// ~/.config/shijimi-editor, for messages that tell where to write a setting.
func UserDir() (string, bool) {
	d, ok := userConfigDir()
	if !ok {
		return "", false
	}
	return userDir(d), true
}

// userDir returns the user's own configuration directory under the config
// home: `shijimi-editor`, or `shijimi-editor-nightly` for a nightly build. A
// nightly build that has no `shijimi-editor-nightly` directory reads
// `shijimi-editor` instead, so that trying the nightly needs no copying of the
// settings; creating the directory makes the two builds independent.
func userDir(configHome string) string {
	d := filepath.Join(configHome, "shijimi-editor"+channel.Suffix())
	if channel.Suffix() != "" {
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
// decoder produces, so both formats share one merge path. ok is false when
// the file is missing, nix is unavailable, or evaluation fails.
func evalNix(path string) (map[string]any, bool) {
	if _, err := os.Stat(path); err != nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), nixEvalTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "nix", "--extra-experimental-features", "nix-command",
		"eval", "--json", "--file", path).Output()
	if err != nil {
		return nil, false
	}
	var raw map[string]any
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, false
	}
	for k, v := range raw {
		raw[k] = normalizeJSON(v)
	}
	// [[language]] is an array of tables in TOML.
	if list, ok := raw["language"].([]any); ok {
		tables := make([]map[string]any, 0, len(list))
		for _, x := range list {
			if m, ok := x.(map[string]any); ok {
				tables = append(tables, m)
			}
		}
		raw["language"] = tables
	}
	return raw, true
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

func (c *Config) mergeEditor(data string) {
	var raw map[string]any
	if _, err := toml.Decode(data, &raw); err != nil {
		return
	}
	c.mergeEditorRaw(raw)
}

func (c *Config) mergeBrowser(data string) {
	var raw map[string]any
	if _, err := toml.Decode(data, &raw); err != nil {
		return
	}
	c.mergeBrowserRaw(raw)
}

func (c *Config) mergeBrowserRaw(raw map[string]any) {
	if br, ok := raw["browser"].(map[string]any); ok {
		if v, ok := br["listen-host"].(string); ok && v != "" {
			c.Browser.ListenHost = v
		}
	}
}

func (c *Config) mergeExperimental(data string) {
	var raw map[string]any
	if _, err := toml.Decode(data, &raw); err != nil {
		return
	}
	c.mergeExperimentalRaw(raw)
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
	if t, ok := raw["terminal"].(map[string]any); ok {
		if v, ok := t["multiplexer"].(string); ok {
			v = strings.ToLower(strings.TrimSpace(v))
			if slices.Contains(Multiplexers, v) {
				c.Terminal.Multiplexer = v
			}
		}
	}
}

func (c *Config) mergeKeys(data string) {
	var raw map[string]any
	if _, err := toml.Decode(data, &raw); err != nil {
		return
	}
	c.mergeKeysRaw(raw)
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
				c.Keys.Invalid = append(c.Keys.Invalid, "keys."+mode+"."+k)
				continue
			}
			if *dst == nil {
				*dst = map[string]string{}
			}
			(*dst)[k] = strings.TrimSpace(s)
		}
	}
	sort.Strings(c.Keys.Invalid)
}

func (c *Config) mergeAgent(data string) {
	var raw map[string]any
	if _, err := toml.Decode(data, &raw); err != nil {
		return
	}
	c.mergeAgentRaw(raw)
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

func (c *Config) mergeLanguages(data, source string) {
	var raw map[string]any
	if _, err := toml.Decode(data, &raw); err != nil {
		return
	}
	c.mergeLanguagesRaw(raw, source)
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
	langs, _ := raw["language"].([]map[string]any)
	for _, m := range langs {
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
