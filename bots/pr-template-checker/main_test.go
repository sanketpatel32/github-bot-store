package main

import "testing"

func TestValidate(t *testing.T) {
	required := []string{"summary", "testing"}

	cases := []struct {
		name        string
		body        string
		wantMissing int
		wantEmpty   int
	}{
		{"fully populated", "## Summary\nAdds the login page.\n\n## Testing\nRan unit tests locally.", 0, 0},
		{"missing testing", "## Summary\nAdds the login page.", 1, 0},
		{"empty heading content", "## Summary\n\n## Testing\nran the tests", 0, 1},
		{"placeholder not accepted", "## Summary\nyour description here\n\n## Testing\nunit tests pass", 0, 1},
		{"na not accepted", "## Summary\nn/a\n\n## Testing\nunit tests pass", 0, 1},
		{"html comments stripped", "<!--## Summary-->\n## Testing\nran tests ok", 1, 0},
		{"keyword in prose does not count", "this is a summary of changes in prose form", 2, 0},
		{"case insensitive headings", "## SUMMARY\nreal content here\n## Testing\nreal tests here", 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			missing, empty := validate(tc.body, required, nil)
			if len(missing) != tc.wantMissing {
				t.Errorf("missing = %v, want %d items", missing, tc.wantMissing)
			}
			if len(empty) != tc.wantEmpty {
				t.Errorf("empty = %v, want %d items", empty, tc.wantEmpty)
			}
		})
	}
}

func TestOptionalSections(t *testing.T) {
	// optional present-but-empty counts as empty; absent is fine
	_, empty := validate("## Summary\nreal content\n## Testing\nreal tests\n## Screenshots\n", nil, []string{"screenshots"})
	if len(empty) != 1 {
		t.Errorf("present-but-empty optional should count, got %v", empty)
	}
	_, empty = validate("## Summary\nreal content\n## Testing\nreal tests", nil, []string{"screenshots"})
	if len(empty) != 0 {
		t.Errorf("absent optional should not count, got %v", empty)
	}
}
