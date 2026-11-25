package main

import "testing"

func TestValidateIssue(t *testing.T) {
	valid := `## What happened
The bot crashes when the payload is empty and nothing works.

## Steps to reproduce
1. send empty payload
2. observe crash

## Expected
No crash, a 400 error.
`
	problems := validateIssue(valid, []string{"what happened", "steps to reproduce", "expected"})
	if len(problems) != 0 {
		t.Errorf("valid issue flagged: %v", problems)
	}

	empty := `## What happened

## Expected
n/a`
	problems = validateIssue(empty, []string{"what happened", "steps to reproduce", "expected"})
	if len(problems) < 3 {
		t.Errorf("expected 3+ problems, got %v", problems)
	}

	none := ""
	problems = validateIssue(none, []string{"what happened"})
	if len(problems) != 1 {
		t.Errorf("empty body should yield 1 missing-section problem, got %v", problems)
	}
}
