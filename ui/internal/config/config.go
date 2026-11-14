// Package config holds the setting that the UIs share and the
// core does not read: the language of the UI text. (How things look that the
// core draws with, such as line numbers and the theme, are the core's own
// settings: core/config.) A UI registers Section with core/config's
// Options.Sections and reads the value with Of.
package config

import (
	"github.com/mm1995tk/shijimi-editor/core/config"
	"github.com/mm1995tk/shijimi-editor/ui/internal/i18n"
)

// Name is the name of Section in config.Config.Value.
const Name = "ui"

// Settings are the shared settings of config.toml.
type Settings struct {
	// UILanguage is the language of the UI text: "en" (default) or "ja".
	// It is set by ui-language in config.toml.
	UILanguage string
}

// Default is the settings of a config that says nothing.
func Default() *Settings {
	return &Settings{UILanguage: i18n.Default}
}

// Section is what the UI registers in config.Options.Sections.
var Section = config.Section{
	Name: Name,
	Schema: map[string]config.Field{
		"ui-language": config.String.Check(checkUILanguage),
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

// Merge reads the shared keys of a decoded config.toml or config.local.toml.
func (s *Settings) Merge(raw map[string]any, user bool) {
	if v, ok := raw["ui-language"].(string); ok && i18n.Valid(v) {
		s.UILanguage = v
	}
}

func checkUILanguage(v any) (string, []any) {
	if s, _ := v.(string); !i18n.Valid(s) {
		return "config.err.oneof", []any{i18n.English + ", " + i18n.Japanese}
	}
	return "", nil
}
