package core

// MinSideSectionH is the least height of a section of the diff mode sidebar:
// a header and two rows.
const MinSideSectionH = 3

// MuxFailTab is the termFailure.tab of failures seen in the multiplexer pane.
const MuxFailTab = -1

// DiffGeom is where the parts of a diff view sit on screen, in cells.
type DiffGeom struct {
	Top, H         int // first content row and number of content rows
	LeftW          int // width of the base pane (0 when unified)
	RightX, RightW int // the editable pane
	BarX           int // the change bar at the right edge
}
