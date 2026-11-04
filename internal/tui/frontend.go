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

// TermBlur takes the keys away from the terminal panel.
func (a *App) TermBlur() { a.term.blur() }

// TermFocus gives the keys to the terminal panel.
func (a *App) TermFocus() []core.Effect { return a.termEffects(a.term.focus()) }

// TermCloseAll closes every shell of the terminal panel.
func (a *App) TermCloseAll() { a.term.closeAll() }

// TermKey sends k to the terminal panel's shell.
func (a *App) TermKey(k core.Key) { a.term.key(k) }

// TermRun runs line in the terminal panel.
func (a *App) TermRun(line string) []core.Effect { return a.termEffects(a.term.run(line)) }

// TermPaste pastes s into the terminal panel.
func (a *App) TermPaste(s string) []core.Effect { return a.termEffects(a.term.paste(s)) }

// TermShell is the shell of the terminal panel, or nil before one starts.
func (a *App) TermShell() core.Shell {
	if a.term.sh == nil {
		return nil
	}
	return a.term.sh
}

// MuxReset makes the multiplexer be looked for again (the config changed).
func (a *App) MuxReset() { a.mux.resolved = false }

// SetPalette makes t the colors the front end draws with (applyTheme, in
// theme_apply.go, rewrites the palette variables and the styles built from
// them). Core keeps which theme is chosen; how it looks is the front end's.
func (a *App) SetPalette(t *core.Theme) { applyTheme(t) }
