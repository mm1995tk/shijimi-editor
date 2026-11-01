package core

import (
	"strings"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/buffer"
)

func TestUnifiedDiff(t *testing.T) {
	var old, cur []string
	for i := 1; i <= 20; i++ {
		old = append(old, "l"+string(rune('a'+i)))
	}
	cur = append(cur, old...)
	cur[10] = "CHANGED"
	got := UnifiedDiff(strings.Join(old, "\n")+"\n", strings.Join(cur, "\n")+"\n")
	var sb strings.Builder
	for _, l := range got {
		if l.Kind == '@' {
			sb.WriteString(l.Text + "\n")
			continue
		}
		sb.WriteString(string(l.Kind) + l.Text + "\n")
	}
	want := "@@ -8,7 +8,7 @@\n li\n lj\n lk\n-ll\n+CHANGED\n lm\n ln\n lo\n"
	if sb.String() != want {
		t.Errorf("diff =\n%s\nwant\n%s", sb.String(), want)
	}
	if d := UnifiedDiff("a\n", "a\n"); len(d) != 1 {
		t.Errorf("no-change diff = %v", d)
	}
	if d := UnifiedDiff("", "x\ny\n"); d[0].Text != "@@ -1,0 +1,2 @@" || len(d) != 3 {
		t.Errorf("new file diff = %v", d)
	}
}

func TestSelectText(t *testing.T) {
	b := buffer.New()
	b.Insert("héllo wörld\nfoo bar")
	SelectText(b, "wör", "ld")
	if got := b.SelectedText(); got != "wörld" {
		t.Errorf("selected %q", got)
	}
	SelectText(b, "bar", "")
	if got := b.SelectedText(); got != "bar" {
		t.Errorf("selected %q", got)
	}
}

func TestSelectionTracker(t *testing.T) {
	var tr SelectionTracker
	a := Selection{FilePath: "/x", Selection: IDERange{Start: IDEPos{1, 0}, End: IDEPos{1, 0}, IsEmpty: true}}
	if !tr.Observe(a) {
		t.Error("the first selection is pushed")
	}
	if tr.Observe(a) {
		t.Error("the same selection is not pushed twice")
	}
	b := a
	b.Selection.Start.Line = 2
	if !tr.Observe(b) || tr.Last().Selection.Start.Line != 2 {
		t.Errorf("a moved selection is pushed; last = %+v", tr.Last())
	}
}

func TestProposalsResolveCloseCancel(t *testing.T) {
	var q Proposals
	r1, r2, r3 := make(chan IDEReply, 1), make(chan IDEReply, 1), make(chan IDEReply, 1)
	p1 := NewProposal("one", "/a", "/a", "x\n", "y\n", r1)
	p2 := NewProposal("two", "/b", "/b", "x\n", "y\n", r2)
	p3 := NewProposal("three", "/c", "/c", "x\n", "y\n", r3)
	q.Add(p1)
	q.Add(p2)
	q.Add(p3)
	if q.First() != p1 || q.Len() != 3 {
		t.Fatalf("first = %v len = %d", q.First(), q.Len())
	}
	q.Resolve(p1, true)
	if got := (<-r1).Content[0].Text; got != "FILE_SAVED" {
		t.Errorf("accepted reply = %q", got)
	}
	q.Close(q.Named("two")[0])
	if got := (<-r2).Content[0].Text; got != "DIFF_REJECTED" {
		t.Errorf("closed reply = %q", got)
	}
	q.Cancel(r3)
	if q.Len() != 0 || q.First() != nil {
		t.Errorf("left %d", q.Len())
	}
}
