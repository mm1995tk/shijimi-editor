package core

import (
	"github.com/mm1995tk/shijimi-editor/core/config"
	"github.com/mm1995tk/shijimi-editor/core/msg"
)

// Port is the one interface the core knows its UI by (docs/design/gui-protocol.md,
// 「core が知る相手は Port だけ」). The TUI and the adapter of `shj serve` meet it.
// Its methods mean the same in every UI and return nothing: the core goes on
// when it has called one, and an answer that matters comes back later as an
// Event (OpenURLResult).
type Port interface {
	// WriteClipboard puts text on the user's OS clipboard.
	WriteClipboard(text string)
	// OpenURL opens url in the user's browser. A UI that can tell whether it
	// worked answers with an OpenURLResult.
	OpenURL(url string)
	// Exit says the program may end.
	Exit()
	// ShowSide, ShowTerm and ShowAgent open or close the sidebar, the terminal
	// panel and the suggestion panel. The UI answers with a Layout.
	ShowSide(on bool)
	ShowTerm(on bool)
	ShowAgent(on bool)
	// DragBorder starts a drag of the border r with the pointer.
	DragBorder(r ResizeTarget)
	// StartResize starts moving the borders with the keys; ResizeKey takes the
	// keys while Layout.ResizeMode is on; ResetPanes puts the borders back.
	// The UI answers with a Layout.
	StartResize()
	ResizeKey(key string)
	ResetPanes()
	// TermFocus gives the keys to the terminal panel (on) or takes them
	// away. A UI that has a shell starts it when it has none; one that cannot
	// start it shows why and closes the panel.
	TermFocus(on bool)
	// TermKey sends a key press to the shell of the terminal panel.
	TermKey(k Key)
	// TermToggle shows the terminal and gives it the keys, or hides it when it
	// is shown. The core calls it only for a UI whose terminal is a pane outside
	// its own panels (Layout.TermInPane), which keeps that state itself; for the
	// others the core shows and hides the panel with ShowTerm and TermFocus.
	TermToggle()
	// TermRun types line into the shell, as if the user entered it.
	TermRun(line string)
	// TermPaste sends s to the shell without a trailing Enter.
	TermPaste(s string)
	// TermClose ends the shell of the terminal panel, and the pane a UI made for
	// it.
	TermClose()
	// CommandEnded says the shell in the multiplexer pane reported the exit
	// code of a command that ended ("shj command-end <code>"). What a UI sees
	// in its own terminal it tells the core with AddFailureTab, ClearFailure
	// and KeepFailuresOf.
	CommandEnded(code int)
	// SetPalette makes t the colors the UI draws with (the user picked a theme,
	// or the config names one). The core keeps which theme is chosen. A GUI
	// draws with its own and asks for the theme with theme.get.
	SetPalette(t *Theme)
	// ImageClosed says the image Id is not shown any more, so that a UI frees
	// what it holds for it (the TUI's terminal graphics).
	ImageClosed(id uint32)
	// Text returns m in the language the UI shows. The core builds every message
	// as a msg.Msg and leaves it to the UI to translate; it asks only where it has to
	// lay text out or match it (wrapping, the palette's search), which needs the
	// words themselves.
	Text(m msg.Msg) string
	// ConfigRead says the core read its configuration: at start (also when it
	// had mistakes, so that they are reported in the user's language) and at
	// :config-reload. The UI takes its own settings from cfg, such as the UI
	// language, and applies them.
	ConfigRead(cfg *config.Config)
}

// nopPort is the Port of a core whose UI has not set one: it does nothing.
type nopPort struct{}

func (nopPort) WriteClipboard(string)     {}
func (nopPort) OpenURL(string)            {}
func (nopPort) Exit()                     {}
func (nopPort) ShowSide(bool)             {}
func (nopPort) ShowTerm(bool)             {}
func (nopPort) ShowAgent(bool)            {}
func (nopPort) DragBorder(ResizeTarget)   {}
func (nopPort) StartResize()              {}
func (nopPort) ResizeKey(string)          {}
func (nopPort) ResetPanes()               {}
func (nopPort) TermFocus(bool)            {}
func (nopPort) TermToggle()               {}
func (nopPort) TermKey(Key)               {}
func (nopPort) TermRun(string)            {}
func (nopPort) TermPaste(string)          {}
func (nopPort) TermClose()                {}
func (nopPort) CommandEnded(int)          {}
func (nopPort) ImageClosed(uint32)        {}
func (nopPort) SetPalette(*Theme)         {}
func (nopPort) Text(m msg.Msg) string     { return m.String() }
func (nopPort) ConfigRead(*config.Config) {}

// portOrNop is p, or a Port that does nothing when p is nil.
func portOrNop(p Port) Port {
	if p == nil {
		return nopPort{}
	}
	return p
}

// SetPort sets the Port of a core made by hand: a test helper (New takes the Port already). A nil
// Port is one that does nothing.
func (a *Core) SetPort(p Port) { a.port = portOrNop(p) }
