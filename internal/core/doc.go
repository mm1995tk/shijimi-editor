// Package core is the part of the editor that does not depend on a terminal:
// no bubbletea, no lipgloss, no terminal cell widths. internal/tui draws it
// to a terminal; the GUI will reach it through the protocol.
// See docs/design/gui-protocol.md.
package core
