package main

import "testing"

func TestSourceExtNotUsedHere(t *testing.T) {
	// placeholder sanity test — mapping logic is integration-level;
	// pure logic tests live in shared/ghbot
	if loadMapping() != nil {
		t.Error("no path-labels.json in test env — should be nil")
	}
}
