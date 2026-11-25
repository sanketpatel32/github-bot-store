package main

import (
	"strings"
	"testing"
)

func TestInsertEntryDedupesOnPRNumber(t *testing.T) {
	body := "### Features\n- original title (#42) @alice\n"
	// same PR, edited title — must NOT be re-inserted (#129)
	out := insertEntry(body, "Features", "- renamed title (#42) @alice", 42)
	if strings.Count(out, "#42)") != 1 {
		t.Errorf("edited title re-inserted the same PR:\n%s", out)
	}
	// different PR must insert even with identical title text
	out = insertEntry(out, "Features", "- original title (#43) @bob", 43)
	if !strings.Contains(out, "#43)") {
		t.Errorf("distinct PR missing after insert:\n%s", out)
	}
}
