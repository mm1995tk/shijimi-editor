package core

import "github.com/mm1995tk/shijimi-editor/internal/i18n"

// setTheme switches the active theme by name.
func (a *Core) setTheme(name string) bool {
	t, ok := lookupTheme(name)
	if !ok {
		return false
	}
	a.SetPalette(t)
	return true
}

// themePicker opens a list of the built-in themes.
func (a *Core) themePicker() []Effect {
	items := make([]PaletteItem, 0, len(Themes))
	for _, n := range themeNames() {
		d := ""
		if n == CurTheme.Name {
			d = i18n.T("theme.current")
		}
		items = append(items, PaletteItem{Label: n, Detail: d})
	}
	return a.Pal.Pick(i18n.T("theme.title"), items, func(i int) []Effect {
		a.setTheme(items[i].Label)
		a.Info(i18n.T("cmd.themeis"), CurTheme.Name)
		return nil
	})
}
