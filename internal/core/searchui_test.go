package core

import (
	"testing"
)

func TestSearchPanelDebounce(t *testing.T) {
	a := newTestApp(t, "needle\n")
	a.openSearch(FQuery)
	keys := func(s string) {
		for _, r := range s {
			a.handleSearchKey(textKey(string(r)))
		}
	}
	keys("nee")
	stale := SearchTick{seq: a.search.seq.Load()}
	keys("dle")
	if a.HandleEvent(stale) != nil {
		t.Fatal("a superseded tick must be dropped")
	}
	effs := a.HandleEvent(SearchTick{seq: a.search.seq.Load()})
	if len(effs) != 1 {
		t.Fatalf("the latest tick must start a search, got %#v", effs)
	}
	run, ok := effs[0].(RunSearch)
	if !ok {
		t.Fatalf("got %#v", effs[0])
	}
	done, ok := run.Run().(SearchDone)
	if !ok || len(done.res.Hits) != 1 {
		t.Fatalf("got %#v", done)
	}
	if a.HandleEvent(done) != nil || a.search.Busy || len(a.search.Res.Hits) != 1 {
		t.Fatal("the result was not taken")
	}
}
