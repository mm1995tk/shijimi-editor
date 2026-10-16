package diff

import "testing"

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
