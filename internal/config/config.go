// Package config loads gh-editor settings. Language and language-server
// definitions use Helix's languages.toml format, so an existing
// `.helix/languages.toml` in a project works as-is.
package config

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
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
}

// Editor holds editor settings (subset of Helix's config.toml [editor]).
type Editor struct {
	Keymap     string // "helix" (default) or "vscode"
	LineNumber string // "absolute" or "relative"
	FileIcons  bool   // Nerd Font icons in the file tree (default true)
}

// Config is the merged configuration.
type Config struct {
	Editor    Editor
	Languages map[string]*Language
	Servers   map[string]*Server
	Sources   []string // files that were loaded, for display
}

// Load merges built-in defaults, the user's Helix config, and the project's
// `.helix/` and `.gh-editor/` directories (later ones win).
func Load(root string) *Config {
	c := &Config{
		Editor:    Editor{Keymap: "helix", LineNumber: "absolute", FileIcons: true},
		Languages: map[string]*Language{},
		Servers:   map[string]*Server{},
	}
	c.mergeLanguages(defaultLanguages, "")

	var dirs []string
	if d, err := os.UserConfigDir(); err == nil {
		dirs = append(dirs, filepath.Join(d, "helix"), filepath.Join(d, "gh-editor"))
	}
	dirs = append(dirs, filepath.Join(root, ".helix"), filepath.Join(root, ".gh-editor"))
	for _, d := range dirs {
		if data, err := os.ReadFile(filepath.Join(d, "config.toml")); err == nil {
			c.mergeEditor(string(data))
			c.Sources = append(c.Sources, filepath.Join(d, "config.toml"))
		}
		if data, err := os.ReadFile(filepath.Join(d, "languages.toml")); err == nil {
			c.mergeLanguages(string(data), filepath.Join(d, "languages.toml"))
		}
	}
	return c
}

func (c *Config) mergeEditor(data string) {
	var raw map[string]any
	if _, err := toml.Decode(data, &raw); err != nil {
		return
	}
	if v, ok := raw["keymap"].(string); ok {
		c.Editor.Keymap = v
	}
	ed, _ := raw["editor"].(map[string]any)
	if v, ok := ed["line-number"].(string); ok {
		c.Editor.LineNumber = v
	}
	if v, ok := ed["file-icons"].(bool); ok {
		c.Editor.FileIcons = v
	}
	if v, ok := ed["keymap"].(string); ok {
		c.Editor.Keymap = v
	}
}

func (c *Config) mergeLanguages(data, source string) {
	var raw map[string]any
	if _, err := toml.Decode(data, &raw); err != nil {
		return
	}
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
				s.Config = cfg
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
		for _, ft := range c.Languages[n].FileTypes {
			if ft == base {
				return c.Languages[n]
			}
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
