// Package config holds the settings that the TUI and the GUI share: how
// things look. The core does not read them. A UI registers Section with
// core/config's Options.Sections and reads the values with Of.
package config

import (
	"github.com/mm1995tk/shijimi-editor/internal/config"
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// Name is the name of Section in config.Config.Value.
const Name = "ui"

// Settings are the shared settings of config.toml.
type Settings struct {
	LineNumber string // "absolute" or "relative"
	FileIcons  bool   // Nerd Font icons in the file tree (default true)
	Theme      string // theme name; "" = default (github-dark)

	// UILanguage is the language of the UI text: "en" (default) or "ja".
	// It is set by ui-language in config.toml.
	UILanguage string

	// MouseHover shows the LSP hover when the pointer rests on a word in the
	// editor. Default true.
	MouseHover bool

	// IndentGuides draws a faint vertical line at each indent level in the
	// editor ([editor.indent-guides] render). Default true.
	IndentGuides bool
}

// Default is the settings of a config that says nothing.
func Default() *Settings {
	return &Settings{
		LineNumber:   "absolute",
		FileIcons:    true,
		UILanguage:   i18n.Default,
		MouseHover:   true,
		IndentGuides: true,
	}
}

// Section is what the UI registers in config.Options.Sections.
var Section = config.Section{
	Name: Name,
	Schema: map[string]config.Field{
		"theme":       config.String,
		"ui-language": config.String.Check(checkUILanguage),
		"editor": config.Table(map[string]config.Field{
			"line-number":   config.String,
			"file-icons":    config.Bool,
			"mouse-hover":   config.Bool,
			"theme":         config.String,
			"indent-guides": config.Table(map[string]config.Field{"render": config.Bool}),
		}),
	},
	New: func() config.Value { return Default() },
}

// Of is the shared settings of c: the defaults when c did not read them
// (a nil Config, or one made by hand).
func Of(c *config.Config) *Settings {
	if s, ok := c.Value(Name).(*Settings); ok {
		return s
	}
	s := Default()
	if c != nil {
		c.SetValue(Name, s)
	}
	return s
}

// Merge reads the shared keys of a decoded config.toml / config.nix.
func (s *Settings) Merge(raw map[string]any, user bool) {
	ed, _ := raw["editor"].(map[string]any)
	if v, ok := ed["line-number"].(string); ok {
		s.LineNumber = v
	}
	if v, ok := ed["file-icons"].(bool); ok {
		s.FileIcons = v
	}
	if v, ok := ed["mouse-hover"].(bool); ok {
		s.MouseHover = v
	}
	if ig, ok := ed["indent-guides"].(map[string]any); ok {
		if v, ok := ig["render"].(bool); ok {
			s.IndentGuides = v
		}
	}
	if v, ok := raw["theme"].(string); ok {
		s.Theme = v
	}
	if v, ok := raw["ui-language"].(string); ok && i18n.Valid(v) {
		s.UILanguage = v
	}
	if v, ok := ed["theme"].(string); ok {
		s.Theme = v
	}
}

func checkUILanguage(v any) (string, []any) {
	if s, _ := v.(string); !i18n.Valid(s) {
		return "config.err.oneof", []any{i18n.English + ", " + i18n.Japanese}
	}
	return "", nil
}
