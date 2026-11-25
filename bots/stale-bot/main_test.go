package main

import (
	"os"
	"testing"
)

func TestLoadOptions(t *testing.T) {
	os.Clearenv()
	opts := loadOptions()
	if !opts.exemptLabels["pinned"] {
		t.Error("pinned must always be exempt")
	}
	if !opts.exemptDraftPRs {
		t.Error("draft PRs exempt by default")
	}
	if opts.exemptAssigned || opts.exemptMilestone || opts.dryRun {
		t.Error("optional exemptions off by default")
	}

	os.Setenv("EXEMPT_LABELS", "critical, wontfix")
	os.Setenv("EXEMPT_ASSIGNED", "true")
	os.Setenv("DRY_RUN", "true")
	opts = loadOptions()
	for _, l := range []string{"critical", "wontfix", "pinned"} {
		if !opts.exemptLabels[l] {
			t.Errorf("expected %s exempt", l)
		}
	}
	if !opts.exemptAssigned || !opts.dryRun {
		t.Error("env-driven options not applied")
	}
	os.Clearenv()
}
