package core

// ProposalWidth is the width of the proposal box on a screen w columns wide.
func ProposalWidth(w int) int { return Clamp(w*4/5, 30, 140) }
