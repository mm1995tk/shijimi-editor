package tui

import (
	"github.com/mm1995tk/shijimi-editor/internal/core"
)

// frontend.go is App's side of core.FrontEnd: the terminal under the editor.
// The fields behind them are App's; Core asks through these calls.

// ReleasesReported reports whether the terminal sends key releases.
func (a *App) ReleasesReported() bool { return a.keyReleases }

// SetReleasesReported records whether the terminal sends key releases.
func (a *App) SetReleasesReported(on bool) { a.keyReleases = on }

// MuxReset makes the multiplexer be looked for again (the config changed).
func (a *App) MuxReset() { a.mux.resolved = false }

// SetPalette makes t the colors the front end draws with (applyTheme, in
// theme_apply.go, rewrites the palette variables and the styles built from
// them). Core keeps which theme is chosen; how it looks is the front end's.
func (a *App) SetPalette(t *core.Theme) { applyTheme(t) }
