package msg

import (
	"encoding/json"
	"errors"
	"testing"
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

func TestJSONIsTheTranslatedText(t *testing.T) {
	t.Cleanup(func() { Marshal = nil })
	Marshal = func(m Msg) string { return "[" + m.Key + "]" }
	b, err := json.Marshal(struct {
		A Msg `json:"a"`
		B Msg `json:"b,omitzero"`
	}{A: T("k")})
	if err != nil || string(b) != `{"a":"[k]"}` {
		t.Errorf("json = %s, %v", b, err)
	}
	var m Msg
	if err := json.Unmarshal([]byte(`"hi"`), &m); err != nil || m.String() != "hi" {
		t.Errorf("unmarshal = %+v, %v", m, err)
	}
}
