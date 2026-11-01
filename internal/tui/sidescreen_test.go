package tui

import (
	"os"
	"path/filepath"
	"testing"
)

// Drawing the agent panel twice gives the same rows: fit already put the
// scroll and the selection where they go.
func TestAgentScreenDrawsTwiceTheSame(t *testing.T) {
	a, l := agentSelApp(t, logEntry{kind: logInfo, text: "one\ntwo"}, logEntry{kind: logAgent, text: "# h\n\ntext"})
	one := drawAgent(a.AgentScreen(l.agentW, l.bodyH, true), l.agentW, l.bodyH, true)
	scroll, sel := a.ag.scroll, a.sg.sel
	two := drawAgent(a.AgentScreen(l.agentW, l.bodyH, true), l.agentW, l.bodyH, true)
	if a.ag.scroll != scroll || a.sg.sel != sel {
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
