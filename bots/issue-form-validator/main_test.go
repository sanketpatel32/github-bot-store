package main

import (
	"strings"
	"testing"
)

// bugFormBody mirrors what .github/ISSUE_TEMPLATE/bug_report.yml renders
// when every field is filled in.
const bugFormBody = `### Bot name

stale-bot

### What happened?

It closed an issue that was still active.

### Logs

` + "```" + `
2024-01-01 closed #42
` + "```" + `

### How are you running it?

GitHub Actions`

// featureFormBody mirrors feature_request.yml fully filled in.
const featureFormBody = `### Summary

Add a bot that pins releases.

### Details

It should pin the latest release discussion.

### Kind

New bot`

func TestFilledBugFormPasses(t *testing.T) {
	if problems := validateIssue(bugFormBody, nil); len(problems) != 0 {
		t.Errorf("complete bug form must pass, got %v", problems)
	}
}

func TestFilledFeatureFormPasses(t *testing.T) {
	if problems := validateIssue(featureFormBody, nil); len(problems) != 0 {
		t.Errorf("complete feature form must pass, got %v", problems)
	}
}

func TestEmptyRequiredFieldFails(t *testing.T) {
	body := strings.Replace(bugFormBody, "It closed an issue that was still active.", "", 1)
	if problems := validateIssue(body, nil); len(problems) == 0 {
		t.Error("empty what-happened section must be flagged")
	}
}

func TestFreeformIssuePassesByDefault(t *testing.T) {
	// API-created maintenance/roadmap issues have no form headings
	if problems := validateIssue("random prose with no headings at all", nil); len(problems) != 0 {
		t.Errorf("freeform issue should pass without explicit REQUIRED_SECTIONS, got %v", problems)
	}
}

func TestExplicitSectionsApplyToEverything(t *testing.T) {
	problems := validateIssue(featureFormBody, []string{"steps to reproduce"})
	if len(problems) == 0 {
		t.Error("explicit REQUIRED_SECTIONS must apply even to feature forms")
	}
}
