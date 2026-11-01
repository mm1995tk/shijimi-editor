package tui

import (
	"github.com/mm1995tk/shijimi-editor/internal/core"
	"testing"
)

// Drawing the agent panel twice gives the same rows: fit already put the
// scroll and the selection where they go.
func TestAgentScreenDrawsTwiceTheSame(t *testing.T) {
	a, l := agentSelApp(t, core.LogEntry{Kind: core.LogInfo, Text: "one\ntwo"}, core.LogEntry{Kind: core.LogAgent, Text: "# h\n\ntext"})
	one := drawAgent(a.AgentScreen(l.agentW, l.bodyH, true), l.agentW, l.bodyH, true)
	scroll, sel := a.Ag.Scroll, a.Sg.Sel
	two := drawAgent(a.AgentScreen(l.agentW, l.bodyH, true), l.agentW, l.bodyH, true)
	if a.Ag.Scroll != scroll || a.Sg.Sel != sel {
		t.Fatal("building the screen changed the state")
	}
	if len(one) != l.bodyH || len(one) != len(two) {
		t.Fatalf("rows %d %d, want %d", len(one), len(two), l.bodyH)
	}
	for i := range one {
		if one[i] != two[i] {
			t.Fatalf("row %d differs", i)
		}
	}
}
