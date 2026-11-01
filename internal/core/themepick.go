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
	items := make([]paletteItem, 0, len(themes))
	for _, n := range themeNames() {
		d := ""
		if n == curTheme.Name {
			d = i18n.T("theme.current")
		}
		items = append(items, paletteItem{label: n, detail: d})
	}
	return a.pal.pick(i18n.T("theme.title"), items, func(i int) []Effect {
		a.setTheme(items[i].label)
		a.info(i18n.T("cmd.themeis"), curTheme.Name)
		return nil
	})
}
