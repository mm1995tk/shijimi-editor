package msg

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestZeroAndLiteral(t *testing.T) {
	if !(Msg{}).IsZero() || !S("").IsZero() || S("a").IsZero() || T("k").IsZero() {
		t.Error("IsZero")
	}
	if got := S("a").Add("b", S("c"), T("k", 1)).String(); got != "abck(1)" {
		t.Errorf("Add = %q", got)
	}
	// text added to text stays one arg, so a streamed reply does not nest
	m := Msg{}
	for range 100 {
		m = m.Add("x")
	}
	if len(m.Args) != 1 {
		t.Errorf("args = %d, want 1", len(m.Args))
	}
}

func TestErrorWrapsItsErrorArgs(t *testing.T) {
	base := errors.New("boom")
	err := Err("k", "x", base)
	if !errors.Is(err, base) {
		t.Error("errors.Is does not see through the message")
	}
	if got := Of(err); got.Key != "k" {
		t.Errorf("Of = %+v", got)
	}
	if got := Of(base); got.Key != "" || len(got.Args) != 1 {
		t.Errorf("Of(plain error) = %+v", got)
	}
	if Of(nil).IsZero() != true {
		t.Error("Of(nil)")
	}
}

func TestJSONForTheGUI(t *testing.T) {
	b, err := json.Marshal(struct {
		A Msg `json:"a"`
		B Msg `json:"b,omitzero"`
		C Msg `json:"c"`
		D Msg `json:"d"`
	}{
		A: T("k", 3, "x", T("inner"), Err("e", 1), errors.New("boom"), time.Second),
		C: S("plain"),
		D: Cat("n=", T("k")),
	})
	want := `{"a":{"key":"k","args":[3,"x",{"key":"inner","args":[]},{"key":"e","args":[1]},"boom","1s"]},"c":"plain","d":{"args":["n=",{"key":"k","args":[]}]}}`
	if err != nil || string(b) != want {
		t.Errorf("json = %s, %v\nwant %s", b, err, want)
	}
	var m Msg
	if err := json.Unmarshal([]byte(`"hi"`), &m); err != nil || m.String() != "hi" {
		t.Errorf("unmarshal = %+v, %v", m, err)
	}
	var back struct{ A Msg }
	if err := json.Unmarshal([]byte(`{"A":{"key":"k","args":["x",{"key":"i","args":[]}]}}`), &back); err != nil ||
		back.A.Key != "k" || back.A.Args[0] != "x" || back.A.Args[1].(Msg).Key != "i" {
		t.Errorf("unmarshal object = %+v, %v", back, err)
	}
}
