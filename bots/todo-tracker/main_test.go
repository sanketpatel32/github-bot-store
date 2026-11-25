package main

import "testing"

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
