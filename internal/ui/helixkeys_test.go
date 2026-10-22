package ui

import (
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/config"
)

func TestNormalizeHelixKey(t *testing.T) {
	for in, want := range map[string]string{
		"C-s": "ctrl+s", "A-x": "alt+x", "S-tab": "shift+tab", "ret": "enter",
		"C-A-x": "ctrl+alt+x", "S-a": "A", "A-S-c": "alt+C", "pageup": "pgup",
		"F5": "f5", "minus": "-", "h": "h", "G": "G", "space": "space", "C-space": "ctrl+space",
	} {
		got, ok := normalizeHelixKey(in)
		if !ok || got != want {
			t.Errorf("normalizeHelixKey(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "C-", "X-a", "foo", "F13"} {
		if got, ok := normalizeHelixKey(in); ok {
			t.Errorf("normalizeHelixKey(%q) = %q, want invalid", in, got)
		}
	}
}

func TestHelixKeysRemap(t *testing.T) {
	a := newTestApp(t, "one two\nthree\n")
	a.cfg.Keys = config.Keys{
		Normal: map[string]string{"h": "move_char_right", "C-x": "no_op", "q": "bogus", "Z": ":write"},
		Insert: map[string]string{"A-j": "normal_mode"},
	}
	a.loadKeymap()
	if !a.msgErr || a.msg == "" {
		t.Fatalf("an unknown command should be reported, got %q", a.msg)
	}
	b := a.curText().buf
	keys(a, "h")
	if b.Cur.Col != 1 {
		t.Fatalf("h should move right, col = %d", b.Cur.Col)
	}
	keys(a, "l") // untouched default
	if b.Cur.Col != 2 {
		t.Fatalf("l col = %d", b.Cur.Col)
	}
	keys(a, "q") // unknown command: the key stays unbound
	keys(a, "i")
	if a.mode != modeInsert {
		t.Fatal("i should enter insert mode")
	}
	a.Update(altKey("j"))
	if a.mode != modeNormal {
		t.Fatal("A-j in insert mode should return to normal mode")
	}
}

func TestHelixSelectKeysInheritNormal(t *testing.T) {
	a := newTestApp(t, "one\ntwo\nthree\n")
	a.cfg.Keys = config.Keys{
		Normal: map[string]string{"j": "move_char_right"},
		Select: map[string]string{"j": "extend_line_below"},
	}
	a.loadKeymap()
	b := a.curText().buf
	keys(a, "j")
	if b.Cur.Col != 1 || b.Cur.Row != 0 {
		t.Fatalf("normal j = %v", b.Cur)
	}
	keys(a, "v", "j")
	if _, e, ok := b.Selection(); !ok || e.Row < 1 {
		t.Fatalf("select j should extend by line, cur = %v", b.Cur)
	}
}

func TestHelixCommandsAllBound(t *testing.T) {
	for k, name := range helixDefaultKeys {
		if _, ok := helixCommands[name]; !ok {
			t.Errorf("default key %q -> unknown command %q", k, name)
		}
	}
}
