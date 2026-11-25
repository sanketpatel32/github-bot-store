package main

import (
	"fmt"
	"testing"
)

func TestMilestonesCoverKeyCounts(t *testing.T) {
	for _, n := range []int{1, 5, 10, 25, 50, 100} {
		if _, ok := milestones[n]; !ok {
			t.Errorf("milestone %d missing", n)
		}
	}
	// messages must contain the %s verb exactly once for the login
	for n, tmpl := range milestones {
		if fmt.Sprintf(tmpl, "user") == tmpl {
			t.Errorf("milestone %d template has no login verb", n)
		}
	}
}
