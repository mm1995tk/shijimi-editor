package ui

import "testing"

func TestUrlAt(t *testing.T) {
	line := []rune("see (https://example.com/a?b=1). ok")
	for col, want := range map[int]string{5: "https://example.com/a?b=1", 30: "", 0: ""} {
		got, ok := urlAt(line, col)
		if got != want || ok != (want != "") {
			t.Errorf("col %d: got %q,%v want %q", col, got, ok, want)
		}
	}
}

func TestOpenURLCommand(t *testing.T) {
	a := newTestApp(t, "go https://example.com/x now\n")
	var opened string
	old := openExternal
	openExternal = func(u string) error { opened = u; return nil }
	defer func() { openExternal = old }()

	a.execCommand("open-url https://a.example")
	if opened != "https://a.example" {
		t.Fatalf("opened %q", opened)
	}
	a.curText().buf.Cur.Col = 5
	a.execCommand("open-url")
	if opened != "https://example.com/x" {
		t.Fatalf("cursor url: opened %q", opened)
	}
	opened = ""
	a.curText().buf.Cur.Col = 0
	a.execCommand("open-url")
	if opened != "" || !a.msgErr {
		t.Fatalf("expected error, opened %q err %v", opened, a.msgErr)
	}
	a.execCommand("open-url ftp://x")
	if opened != "" || !a.msgErr {
		t.Fatal("non-http URL must be rejected")
	}
}

func TestOpenExternalMissingCommand(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := openExternal("https://example.com"); err == nil {
		t.Fatal("expected error when opener is missing")
	}
}
