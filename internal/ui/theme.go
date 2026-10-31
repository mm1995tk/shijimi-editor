package ui

import (
	"maps"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

// syntaxColors are the token colors a theme provides.
type syntaxColors struct {
	Text, Comment, Preproc, Keyword, Namespace, Type, Name, Function string
	Const, String, Escape, Number                                    string
	Control                                                          string // if, return, ... (semantic tokens); "" = Keyword
}

// Theme is a complete color scheme for the UI and syntax highlighting.
type Theme struct {
	Name string

	EditorBg, EditorFg, SideBg, SideFg, TabBarBg, TabInact, TabInactFg string
	Accent, LineNo, LineNoCur, CurLine, Selection, Match, Border       string
	PaletteBg, ListSel, ListSelUnf, Dirty, Error                       string
	IndentGuide                                                        string // editor indent guides; "" = LineNo

	Strong      string // text on selections and active tabs
	SubSel      string // secondary text on a selected list row
	SideTitle   string
	Dim         string
	Heading     string
	PanelTitle  string
	PanelCmd    string
	PaletteHi   string
	AccentFg    string // text on the status bar
	StatusErrBg string
	Cursor      string
	CursorFg    string // text under the cursor / inside mode badges

	ModeNormal, ModeInsert, ModeSelect string
	GitAdded, GitDeleted, Renamed      string
	DiagError, DiagWarn, DiagInfo      string

	HoverCodeBg, HoverInline, HoverHead, HoverRule string

	AddBg, DelBg, AddSign, DelSign, HunkFg, HunkBg, NoteBg string
	// diff mode: changed parts of a line, rewrites on the change bar, and the
	// part of the bar that is on screen
	AddWordBg, DelWordBg, ModSign, BarView string

	Syntax syntaxColors

	// Icons overrides file tree icons by icon kind (see icons.go). Kinds not
	// listed keep the default icon.
	Icons map[string]iconOverride
}

var darkPlus = Theme{
	Name:     "dark-plus",
	EditorBg: "#1e1e1e", EditorFg: "#d4d4d4", SideBg: "#252526", SideFg: "#cccccc",
	TabBarBg: "#252526", TabInact: "#2d2d2d", TabInactFg: "#969696",
	Accent: "#007acc", LineNo: "#858585", LineNoCur: "#c6c6c6", CurLine: "#2a2d2e", IndentGuide: "#404040",
	Selection: "#264f78", Match: "#613214", Border: "#3c3c3c",
	PaletteBg: "#252526", ListSel: "#04395e", ListSelUnf: "#37373d",
	Dirty: "#e2c08d", Error: "#f48771",
	Strong: "#ffffff", SubSel: "#bbbbbb", SideTitle: "#bbbbbb", Dim: "#6e6e6e",
	Heading: "#9cdcfe", PanelTitle: "#e7e7e7", PanelCmd: "#4ec9b0", PaletteHi: "#2aaaff",
	AccentFg: "#ffffff", StatusErrBg: "#c72e0f", Cursor: "#aeafad", CursorFg: "#1e1e1e",
	ModeNormal: "#569cd6", ModeInsert: "#73c991", ModeSelect: "#e2c08d",
	GitAdded: "#73c991", GitDeleted: "#c74e39", Renamed: "#d2a8ff",
	DiagError: "#f14c4c", DiagWarn: "#cca700", DiagInfo: "#3794ff",
	HoverCodeBg: "#151515", HoverInline: "#ce9178", HoverHead: "#569cd6", HoverRule: "#5a5a5a",
	AddBg: "#203a27", DelBg: "#3d2125", AddSign: "#3fb950", DelSign: "#f85149",
	HunkFg: "#79c0ff", HunkBg: "#1d2733", NoteBg: "#2b2b31",
	AddWordBg: "#2f6b3c", DelWordBg: "#7a2e35", ModSign: "#3794ff", BarView: "#45494e",
	Syntax: syntaxColors{
		Text: "#d4d4d4", Comment: "#6a9955", Preproc: "#c586c0", Keyword: "#569cd6",
		Namespace: "#c586c0", Type: "#4ec9b0", Name: "#9cdcfe", Function: "#dcdcaa",
		Const: "#4fc1ff", String: "#ce9178", Escape: "#d7ba7d", Number: "#b5cea8", Control: "#c586c0",
	},
}

var githubDark = Theme{
	Name:     "github-dark",
	EditorBg: "#0d1117", EditorFg: "#c9d1d9", SideBg: "#010409", SideFg: "#c9d1d9",
	TabBarBg: "#010409", TabInact: "#161b22", TabInactFg: "#8b949e",
	Accent: "#1f6feb", LineNo: "#6e7681", LineNoCur: "#c9d1d9", CurLine: "#161b22", IndentGuide: "#30363d",
	Selection: "#264466", Match: "#5a4a10", Border: "#30363d",
	PaletteBg: "#161b22", ListSel: "#1f3a5f", ListSelUnf: "#21262d",
	Dirty: "#e3b341", Error: "#ff7b72",
	Strong: "#f0f6fc", SubSel: "#b1bac4", SideTitle: "#8b949e", Dim: "#6e7681",
	Heading: "#79c0ff", PanelTitle: "#f0f6fc", PanelCmd: "#7ee787", PaletteHi: "#58a6ff",
	AccentFg: "#ffffff", StatusErrBg: "#da3633", Cursor: "#58a6ff", CursorFg: "#0d1117",
	ModeNormal: "#58a6ff", ModeInsert: "#3fb950", ModeSelect: "#e3b341",
	GitAdded: "#3fb950", GitDeleted: "#f85149", Renamed: "#d2a8ff",
	DiagError: "#f85149", DiagWarn: "#d29922", DiagInfo: "#58a6ff",
	HoverCodeBg: "#010409", HoverInline: "#a5d6ff", HoverHead: "#79c0ff", HoverRule: "#484f58",
	AddBg: "#1a3b2a", DelBg: "#2d1214", AddSign: "#3fb950", DelSign: "#f85149",
	HunkFg: "#79c0ff", HunkBg: "#121d2f", NoteBg: "#161b22",
	AddWordBg: "#287140", DelWordBg: "#6e2027", ModSign: "#58a6ff", BarView: "#30363d",
	Syntax: syntaxColors{
		Text: "#c9d1d9", Comment: "#8b949e", Preproc: "#ff7b72", Keyword: "#ff7b72",
		Namespace: "#ff7b72", Type: "#ffa657", Name: "#c9d1d9", Function: "#d2a8ff",
		Const: "#79c0ff", String: "#a5d6ff", Escape: "#79c0ff", Number: "#79c0ff",
	},
}

var lightPlus = Theme{
	Name:     "light-plus",
	EditorBg: "#ffffff", EditorFg: "#000000", SideBg: "#f3f3f3", SideFg: "#616161",
	TabBarBg: "#f3f3f3", TabInact: "#ececec", TabInactFg: "#8e8e90",
	Accent: "#007acc", LineNo: "#237893", LineNoCur: "#0b216f", CurLine: "#f2f2f2", IndentGuide: "#d3d3d3",
	Selection: "#add6ff", Match: "#f8c9ab", Border: "#e5e5e5",
	PaletteBg: "#f3f3f3", ListSel: "#0060c0", ListSelUnf: "#e4e6f1",
	Dirty: "#bf8803", Error: "#e51400",
	Strong: "#000000", SubSel: "#e0e0e0", SideTitle: "#616161", Dim: "#a0a0a0",
	Heading: "#0451a5", PanelTitle: "#424242", PanelCmd: "#267f99", PaletteHi: "#0066bf",
	AccentFg: "#ffffff", StatusErrBg: "#c72e0f", Cursor: "#000000", CursorFg: "#ffffff",
	ModeNormal: "#0451a5", ModeInsert: "#2e8b57", ModeSelect: "#bf8803",
	GitAdded: "#2e8b57", GitDeleted: "#ad0707", Renamed: "#6f42c1",
	DiagError: "#e51400", DiagWarn: "#bf8803", DiagInfo: "#1a85ff",
	HoverCodeBg: "#ececec", HoverInline: "#a31515", HoverHead: "#0000ff", HoverRule: "#b0b0b0",
	AddBg: "#dafbe1", DelBg: "#ffebe9", AddSign: "#1a7f37", DelSign: "#cf222e",
	HunkFg: "#0550ae", HunkBg: "#ddf4ff", NoteBg: "#fff8c5",
	AddWordBg: "#aceebb", DelWordBg: "#ffcecb", ModSign: "#0969da", BarView: "#d0d7de",
	Syntax: syntaxColors{
		Text: "#000000", Comment: "#008000", Preproc: "#af00db", Keyword: "#0000ff",
		Namespace: "#af00db", Type: "#267f99", Name: "#001080", Function: "#795e26",
		Const: "#0070c1", String: "#a31515", Escape: "#ee0000", Number: "#098658", Control: "#af00db",
	},
	// The default icon colors are tuned for dark backgrounds; darken the ones
	// that wash out on white.
	Icons: map[string]iconOverride{
		"folder":      {Color: "#3b7bb5"},
		"folder-open": {Color: "#3b7bb5"},
		"default":     {Color: "#6e6e6e"},
		"text":        {Color: "#6e6e6e"},
		"json":        {Color: "#9a8a00"},
		"js":          {Color: "#9a8a00"},
		"license":     {Color: "#9a8a00"},
		"shell":       {Color: "#3f8f1f"},
		"python":      {Color: "#c88a00"},
		"config":      {Color: "#6b7785"},
		"lock":        {Color: "#6b7785"},
		"rust":        {Color: "#b5694a"},
		"archive":     {Color: "#a07a40"},
		"nix":         {Color: "#4f8fc4"},
	},
}

var themes = []*Theme{&darkPlus, &githubDark, &lightPlus}

// defaultTheme is used when no theme (or an unknown one) is configured.
var defaultTheme = &githubDark

// themeAliases maps other names (notably Helix theme names) to built-ins.
var themeAliases = map[string]string{
	"dark":         "dark-plus",
	"vscode":       "dark-plus",
	"github":       "github-dark",
	"github-light": "light-plus",
	"light":        "light-plus",
	"default":      "github-dark",
}

// themeNames lists the built-in theme names, sorted.
func themeNames() []string {
	names := make([]string, 0, len(themes))
	for _, t := range themes {
		names = append(names, t.Name)
	}
	sort.Strings(names)
	return names
}

// lookupTheme finds a theme by name. Names are case-insensitive and `_` equals
// `-`. Helix variants such as "github_dark_dimmed" resolve to the built-in
// whose name (or alias) is the longest prefix.
func lookupTheme(name string) (*Theme, bool) {
	n := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(name)), "_", "-")
	keys := map[string]string{}
	for _, t := range themes {
		keys[t.Name] = t.Name
	}
	maps.Copy(keys, themeAliases)
	best := ""
	for k := range keys {
		if (n == k || strings.HasPrefix(n, k+"-")) && len(k) > len(best) {
			best = k
		}
	}
	for _, t := range themes {
		if best != "" && t.Name == keys[best] {
			return t, true
		}
	}
	return nil, false
}

var (
	curTheme *Theme
	// themeGen increments on every applyTheme so caches can invalidate.
	themeGen int
)

func init() { applyTheme(defaultTheme) }

// applyTheme makes t the active theme: it rewrites the palette variables,
// rebuilds precomputed styles, and invalidates color caches.
func applyTheme(t *Theme) {
	curTheme = t
	themeGen++

	cEditorBg, cEditorFg, cSideBg, cSideFg = t.EditorBg, t.EditorFg, t.SideBg, t.SideFg
	cTabBarBg, cTabInact, cTabInactFg = t.TabBarBg, t.TabInact, t.TabInactFg
	cAccent, cLineNo, cLineNoCur, cCurLine = t.Accent, t.LineNo, t.LineNoCur, t.CurLine
	cSelection, cMatch, cBorder = t.Selection, t.Match, t.Border
	if cIndentGuide = t.IndentGuide; cIndentGuide == "" {
		cIndentGuide = t.LineNo
	}
	cPaletteBg, cListSel, cListSelUnf = t.PaletteBg, t.ListSel, t.ListSelUnf
	cDirty, cError = t.Dirty, t.Error

	cAddBg, cDelBg, cAddSign, cDelSign = t.AddBg, t.DelBg, t.AddSign, t.DelSign
	cHunkFg, cHunkBg, cNoteBg = t.HunkFg, t.HunkBg, t.NoteBg
	cAddWordBg, cDelWordBg, cModSign, cBarView = t.AddWordBg, t.DelWordBg, t.ModSign, t.BarView

	cHoverBg, cHoverFg, cHoverBorder = t.PaletteBg, t.EditorFg, t.Accent
	cHoverRule = t.HoverRule

	modeStyles = map[editMode]lipgloss.Style{
		modeNormal: bgfg(t.ModeNormal, t.CursorFg).Bold(true),
		modeInsert: bgfg(t.ModeInsert, t.CursorFg).Bold(true),
		modeSelect: bgfg(t.ModeSelect, t.CursorFg).Bold(true),
	}

	s := t.Syntax
	chromaStyle = chroma.MustNewStyle(t.Name, chroma.StyleEntries{
		chroma.Text:                s.Text,
		chroma.Comment:             s.Comment,
		chroma.CommentPreproc:      s.Preproc,
		chroma.Keyword:             s.Keyword,
		chroma.KeywordNamespace:    s.Namespace,
		chroma.KeywordType:         s.Type,
		chroma.Name:                s.Name,
		chroma.NameBuiltin:         s.Type,
		chroma.NameFunction:        s.Function,
		chroma.NameClass:           s.Type,
		chroma.NameConstant:        s.Const,
		chroma.NameTag:             s.Keyword,
		chroma.NameAttribute:       s.Name,
		chroma.NameDecorator:       s.Function,
		chroma.LiteralString:       s.String,
		chroma.LiteralStringEscape: s.Escape,
		chroma.LiteralNumber:       s.Number,
		chroma.Operator:            s.Text,
		chroma.Punctuation:         s.Text,
		chroma.GenericHeading:      "bold " + s.Keyword,
		chroma.GenericSubheading:   "bold " + s.Keyword,
		chroma.GenericInserted:     s.Number,
		chroma.GenericDeleted:      s.String,
		chroma.GenericEmph:         "italic",
		chroma.GenericStrong:       "bold",
	})

	cellStyles = map[cellStyle]lipgloss.Style{}
	tokenColors = map[chroma.TokenType]string{}
	rebuildStyles(t)
}

// setTheme switches the active theme by name.
func (a *App) setTheme(name string) bool {
	t, ok := lookupTheme(name)
	if !ok {
		return false
	}
	applyTheme(t)
	return true
}

// themePicker opens a list of the built-in themes.
func (a *App) themePicker() tea.Cmd {
	items := make([]paletteItem, 0, len(themes))
	for _, n := range themeNames() {
		d := ""
		if n == curTheme.Name {
			d = i18n.T("theme.current")
		}
		items = append(items, paletteItem{label: n, detail: d})
	}
	return a.pal.pick(i18n.T("theme.title"), items, func(i int) tea.Cmd {
		a.setTheme(items[i].label)
		a.info(i18n.T("cmd.themeis"), curTheme.Name)
		return nil
	})
}

func themeOrDefault(name string) *Theme {
	if t, ok := lookupTheme(name); ok {
		return t
	}
	return defaultTheme
}
