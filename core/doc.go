// Package core is the part of the editor that does not depend on a terminal:
// no bubbletea, no lipgloss, no terminal cell widths. ui/tui draws it
// to a terminal. `kisui serve` does not hold a core: the GUI shows terminals, and the
// editor runs in one of them as the TUI. See docs/design/gui-protocol.md.
package core
