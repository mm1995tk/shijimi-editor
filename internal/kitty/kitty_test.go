package kitty

import (
	"bytes"
	"image"
	"image/jpeg"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestSupported(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"none", nil, false},
		{"xterm", map[string]string{"TERM": "xterm-256color"}, false},
		{"kitty term", map[string]string{"TERM": "xterm-kitty"}, true},
		{"kitty window", map[string]string{"KITTY_WINDOW_ID": "3"}, true},
		{"ghostty", map[string]string{"TERM_PROGRAM": "ghostty"}, true},
		{"wezterm", map[string]string{"TERM_PROGRAM": "WezTerm"}, true},
		{"herdr", map[string]string{"TERM_PROGRAM": "herdr", "TERM": "xterm-256color"}, false},
		{"herdr in ghostty", map[string]string{"TERM_PROGRAM": "herdr", "GHOSTTY_RESOURCES_DIR": "/x"}, false},
		{"herdr forced", map[string]string{"TERM_PROGRAM": "herdr", EnvGraphics: "kitty"}, true},
		{"force on", map[string]string{EnvGraphics: "kitty"}, true},
		{"force off", map[string]string{EnvGraphics: "halfblock", "TERM": "xterm-kitty"}, false},
	}
	for _, c := range cases {
		if got := Supported(env(c.env)); got != c.want {
			t.Errorf("%s: Supported = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestDiacriticsTable(t *testing.T) {
	if len(diacritics) != 297 {
		t.Fatalf("diacritics has %d entries, want 297", len(diacritics))
	}
	if diacritics[0] != 0x305 || diacritics[1] != 0x30d {
		t.Errorf("unexpected first entries %U %U", diacritics[0], diacritics[1])
	}
	for i := 1; i < len(diacritics); i++ {
		if diacritics[i] <= diacritics[i-1] {
			t.Fatalf("diacritics not ascending at %d", i)
		}
	}
}

func TestPlaceholders(t *testing.T) {
	lines := Placeholders(0x010203, 3, 2)
	if len(lines) != 2 {
		t.Fatalf("got %d lines", len(lines))
	}
	if !strings.HasPrefix(lines[0], "\x1b[38;2;1;2;3m") {
		t.Errorf("image id missing from foreground colour: %q", lines[0])
	}
	if n := strings.Count(lines[1], string(placeholder)); n != 3 {
		t.Errorf("line 1 has %d placeholders, want 3", n)
	}
	if !strings.Contains(lines[1], string(placeholder)+string(diacritics[1])+string(diacritics[2])) {
		t.Errorf("row 1 / column 2 cell not found: %q", lines[1])
	}
	// the TUI measures lines with this; it must be exactly the column count
	for i, l := range lines {
		if w := ansi.StringWidth(l); w != 3 {
			t.Errorf("line %d width = %d, want 3", i, w)
		}
	}
	if got := Placeholders(1, MaxCells+50, 1); ansi.StringWidth(got[0]) != MaxCells {
		t.Errorf("columns are not clamped to %d", MaxCells)
	}
}

func TestTransmitChunks(t *testing.T) {
	data := bytes.Repeat([]byte{0xab}, 10000)
	s := Transmit(7, data, 40, 10)
	if !strings.HasPrefix(s, "\x1b_Ga=T,U=1,f=100,i=7,c=40,r=10,C=1,q=2,m=1;") {
		t.Errorf("bad first chunk header: %q", s[:60])
	}
	chunks := strings.Split(strings.TrimSuffix(s, "\x1b\\"), "\x1b\\")
	if len(chunks) != 4 { // 10000 bytes = 13336 base64 chars = 4 chunks
		t.Fatalf("got %d chunks, want 4", len(chunks))
	}
	for i, c := range chunks {
		_, payload, _ := strings.Cut(c, ";")
		if len(payload) > chunkSize {
			t.Errorf("chunk %d payload is %d bytes", i, len(payload))
		}
		last := i == len(chunks)-1
		if last != strings.Contains(c[:20], "m=0") {
			t.Errorf("chunk %d: wrong m= flag in %q", i, c[:20])
		}
		if !utf8.ValidString(c) {
			t.Errorf("chunk %d is not valid UTF-8", i)
		}
	}
	if s := Transmit(7, []byte("x"), 1, 1); !strings.Contains(s, "m=0;") {
		t.Errorf("single chunk must end with m=0: %q", s)
	}
}

func TestDelete(t *testing.T) {
	if got := Delete(9); got != "\x1b_Ga=d,d=I,i=9,q=2\x1b\\" {
		t.Errorf("Delete = %q", got)
	}
}

func TestTransmitJPEG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 16, 8))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	s, err := TransmitJPEG(3, buf.Bytes(), 4, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(s, "\x1b_Ga=T,U=1,f=24,o=z,s=16,v=8,i=3,c=4,r=2,C=1,q=2,m=") {
		t.Errorf("bad header: %q", s[:70])
	}
	if _, err := TransmitJPEG(3, []byte("not a jpeg"), 4, 2); err == nil {
		t.Error("invalid JPEG must fail")
	}
}
