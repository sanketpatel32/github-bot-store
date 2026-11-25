package main

import "testing"

func TestTitleReDefault(t *testing.T) {
	re := titleRe()
	for _, good := range []string{"feat: add x", "fix(api): handle nil", "feat!: breaking", "revert: undo"} {
		if !re.MatchString(good) {
			t.Errorf("should pass: %q", good)
		}
	}
	for _, bad := range []string{"Add feature", "feat add x", "wip: nope"} {
		if re.MatchString(bad) {
			t.Errorf("should fail: %q", bad)
		}
	}
}

func TestTitleReCustomTypes(t *testing.T) {
	t.Setenv("ALLOWED_TYPES", "feature,bugfix")
	re := titleRe()
	if !re.MatchString("feature: x") || !re.MatchString("bugfix: y") {
		t.Error("custom types rejected")
	}
	if re.MatchString("feat: x") {
		t.Error("default type should be rejected when overridden")
	}
}

func TestRequireScope(t *testing.T) {
	t.Setenv("REQUIRE_SCOPE", "true")
	re := titleRe()
	if re.MatchString("feat: x") {
		t.Error("scope should be required")
	}
	if !re.MatchString("feat(api): x") {
		t.Error("scoped title should pass")
	}
}
