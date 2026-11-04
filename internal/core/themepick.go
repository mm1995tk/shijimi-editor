package core

import "github.com/mm1995tk/shijimi-editor/internal/msg"

// setTheme switches the active theme by name.
func (a *Core) setTheme(name string) bool {
	t, ok := lookupTheme(name)
	if !ok {
		return false
	}
	a.port.SetPalette(t)
	return true
}

// themePicker opens a list of the built-in themes.
func (a *Core) themePicker() []Effect {
	items := make([]PaletteItem, 0, len(Themes))
	names := themeNames()
	for _, n := range names {
		var d msg.Msg
		if n == CurTheme.Name {
			d = msg.T("theme.current")
		}
		items = append(items, PaletteItem{Label: msg.S(n), Detail: d})
	}
	return a.Pal.Pick(msg.T("theme.title"), items, func(i int) []Effect {
		a.setTheme(names[i])
		a.Info("cmd.themeis", CurTheme.Name)
		return nil
	})
}
