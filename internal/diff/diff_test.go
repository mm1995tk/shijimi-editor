package diff

import (
	"reflect"
	"strings"
	"testing"
)

const sample = `diff --git a/main.go b/main.go
index 1111111..2222222 100644
--- a/main.go
+++ b/main.go
@@ -1,4 +1,5 @@ package main
 package main
-import "fmt"
+import (
+	"fmt"
+)

 func main() {}
diff --git a/new.txt b/new.txt
new file mode 100644
--- /dev/null
+++ b/new.txt
@@ -0,0 +1 @@
+hello
`

func TestParse(t *testing.T) {
	files := Parse(sample)
	if len(files) != 2 {
		t.Fatalf("files = %d", len(files))
	}
	f := files[0]
	if f.Path() != "main.go" || f.Added != 3 || f.Deleted != 1 {
		t.Fatalf("file0 = %+v", f)
	}
	h := f.Hunks[0]
	if h.Lines[0].OldNo != 1 || h.Lines[0].NewNo != 1 {
		t.Fatalf("line0 = %+v", h.Lines[0])
	}
	if l := h.Lines[3]; l.Kind != Add || l.NewNo != 3 {
		t.Fatalf("line3 = %+v", l)
	}
	if l := h.Lines[5]; l.Kind != Context || l.OldNo != 3 || l.NewNo != 5 {
		t.Fatalf("line5 = %+v", l)
	}
	if files[1].Status != "added" || files[1].Path() != "new.txt" {
		t.Fatalf("file1 = %+v", files[1])
	}
}

func TestLines(t *testing.T) {
	split := func(s string) []string {
		if s == "" {
			return nil
		}
		return strings.Split(s, ",")
	}
	cases := []struct {
		old, cur string
		want     []Block
	}{
		{"a,b,c", "a,b,c", nil},
		{"", "a,b", []Block{{0, 0, 0, 2}}},
		{"a,b", "", []Block{{0, 2, 0, 0}}},
		{"a,b,c", "a,x,c", []Block{{1, 2, 1, 2}}},
		{"a,b,c,d,e", "a,c,d,x,y,e", []Block{{1, 2, 1, 1}, {4, 4, 3, 5}}},
		{"a,b,c", "x,a,b,c,y", []Block{{0, 0, 0, 1}, {3, 3, 4, 5}}},
		{"a,a,a", "a,a", []Block{{2, 3, 2, 2}}},
	}
	for _, c := range cases {
		got := Lines(split(c.old), split(c.cur))
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Lines(%q, %q) = %v, want %v", c.old, c.cur, got, c.want)
		}
	}
}

func TestLinesRebuilds(t *testing.T) {
	old := strings.Split("a b c d e f g h i j", " ")
	cur := strings.Split("a c d x y f g i j k", " ")
	var out []string
	pos := 0
	for _, b := range Lines(old, cur) {
		out = append(out, old[pos:b.OldStart]...)
		out = append(out, cur[b.NewStart:b.NewEnd]...)
		pos = b.OldEnd
	}
	out = append(out, old[pos:]...)
	if !reflect.DeepEqual(out, cur) {
		t.Errorf("rebuilt %v, want %v", out, cur)
	}
}
