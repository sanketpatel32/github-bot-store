package main

import (
	"strings"
	"testing"
)

func TestParsePatch(t *testing.T) {
	patch := `@@ -10,6 +10,8 @@ func example() {
 context line
+// TODO: handle nil input properly
+	x := compute()
-// TODO: old hack no longer needed
+// FIXME validate config before use
--- header lines ignored
+++ b/file.go`
	added, removed := parsePatch(patch)
	if len(added) != 2 {
		t.Errorf("expected 2 added, got %v", added)
	}
	if len(removed) != 1 || removed[0] != "old hack no longer needed" {
		t.Errorf("removed wrong: %v", removed)
	}
	if added[0] != "handle nil input properly" {
		t.Errorf("added[0] = %q", added[0])
	}
}

func TestParsePatchTruncated(t *testing.T) {
	a, r := parsePatch("")
	if len(a) != 0 || len(r) != 0 {
		t.Error("empty patch must yield nothing")
	}
}

func TestMarkerRoundtrip(t *testing.T) {
	h := todoHash("main.go", "fix this")
	if m := marker(h); m != "<!-- todo-tracker:"+h+" -->" {
		t.Errorf("marker format wrong: %s", m)
	}
	// same input, same hash — dedupe stability
	if todoHash("main.go", "fix this") != h {
		t.Error("hash not stable")
	}
	if todoHash("other.go", "fix this") == h {
		t.Error("hash should differ by file")
	}
}

func TestScannable(t *testing.T) {
	for _, yes := range []string{"main.go", "src/app.ts", "x.py", ".github/workflows/ci.yml"} {
		if !scannable(yes) {
			t.Errorf("%s should be scannable", yes)
		}
	}
	for _, no := range []string{"docs/bots/todo-tracker.html", "README.md", "go.sum", "manifest.json"} {
		if scannable(no) {
			t.Errorf("%s should NOT be scannable — false positive source", no)
		}
	}
}

func TestParsePatchSkipsProseAndEmptyMarkers(t *testing.T) {
	patch := `+// TODO: implement retry backoff — this one is real
+// TODO/FIXME as words, which the scanner mentions in prose
+// the word TODO appears mid-sentence here
+// TODO
+	//FIXME
+# TODO: another real one`
	added, _ := parsePatch(patch)
	if len(added) != 2 {
		t.Fatalf("want exactly 2 real TODOs, got %d: %v", len(added), added)
	}
	for _, a := range added {
		if strings.Contains(a, "prose") || strings.Contains(a, "mid-sentence") {
			t.Errorf("prose leaked through: %q", a)
		}
	}
}
