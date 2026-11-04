package core

import "testing"

func TestLayoutInput(t *testing.T) {
	a, f := newTestCore(t, t.TempDir(), "", 100, 30)
	if !a.SideShown() {
		t.Fatal("the test front starts with the sidebar, and the core was told")
	}
	a.Update(Layout{TermShown: true, AgentOpen: true, AgentWidthPref: 33, ExplorerShare: 40, ResizeMode: true})
	if a.SideShown() || !a.TermShown() || !a.AgentOpen() || a.AgentWidthPref() != 33 || a.ExplorerShare() != 40 || !a.InResizeMode() {
		t.Fatalf("layout = %+v", a.layout)
	}
	a.SetSide(true)
	if !a.SideShown() || !f.side {
		t.Fatalf("SetSide: core %v, port %v", a.SideShown(), f.side)
	}
}
