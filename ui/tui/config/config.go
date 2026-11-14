// Package config holds the settings only the TUI reads: [terminal] and
// [experimental]. A UI registers Section(...) with core/config's
// Options.Sections and reads the values with Of.
package config

import (
	"slices"
	"strings"

	"github.com/mm1995tk/shijimi-editor/core/config"
)

// Name is the name of the section in config.Config.Value.
const Name = "tui"

// Multiplexers are the values of [terminal] multiplexer.
var Multiplexers = []string{"auto", "herdr", "tmux", "zellij", "cmux", "gui", "none"}

// Terminal is the [terminal] section of config.toml.
type Terminal struct {
	// Multiplexer is the multiplexer whose panes replace the built-in terminal
	// while Experimental.Multiplexer is on: "auto" (the innermost one the
	// editor runs in; in the GUI's primary terminal, the GUI), "herdr", "tmux",
	// "zellij", "cmux", "gui" (the panes of the GUI, for the shj of its primary
	// terminal) or "none" (always the built-in terminal).
	Multiplexer string
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

// Settings are the TUI's settings of config.toml.
type Settings struct {
	Terminal     Terminal
	Experimental Experimental
}

// Default is the settings of a config that says nothing. experimental is the
// default of the [experimental] keys (see Experimental.Default).
func Default(experimental bool) *Settings {
	return &Settings{
		Terminal:     Terminal{Multiplexer: "auto"},
		Experimental: Experimental{Default: experimental, Multiplexer: experimental},
	}
}

// Section is what the TUI registers in config.Options.Sections. experimental
// is the default of the [experimental] keys: the build's release channel.
func Section(experimental bool) config.Section {
	return config.Section{
		Name: Name,
		Schema: map[string]config.Field{
			"experimental": config.Table(map[string]config.Field{
				"herdr":       config.Bool,
				"multiplexer": config.Bool,
			}),
			"terminal": config.Table(map[string]config.Field{
				"multiplexer": config.String.Check(checkMultiplexer),
			}),
		},
		New: func() config.Value { return Default(experimental) },
	}
}

// Of is the TUI's settings of c: the defaults of the stable channel when c did
// not read them (a nil Config, or one made by hand).
func Of(c *config.Config) *Settings {
	if s, ok := c.Value(Name).(*Settings); ok {
		return s
	}
	s := Default(false)
	if c != nil {
		c.SetValue(Name, s)
	}
	return s
}

// Merge reads the TUI's keys of a decoded config.toml or config.local.toml.
func (s *Settings) Merge(raw map[string]any, user bool) {
	if ex, ok := raw["experimental"].(map[string]any); ok {
		// "herdr" is the old name of "multiplexer"; the new one wins when both are set.
		if v, ok := ex["herdr"].(bool); ok {
			s.Experimental.Multiplexer = v
		}
		if v, ok := ex["multiplexer"].(bool); ok {
			s.Experimental.Multiplexer = v
		}
	}
	if t, ok := raw["terminal"].(map[string]any); ok {
		if v, ok := t["multiplexer"].(string); ok {
			v = strings.ToLower(strings.TrimSpace(v))
			if slices.Contains(Multiplexers, v) {
				s.Terminal.Multiplexer = v
			}
		}
	}
}

func checkMultiplexer(v any) (string, []any) {
	s, _ := v.(string)
	if !slices.Contains(Multiplexers, strings.ToLower(strings.TrimSpace(s))) {
		return "config.err.oneof", []any{strings.Join(Multiplexers, ", ")}
	}
	return "", nil
}
