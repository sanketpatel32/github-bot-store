package main

import "testing"

func TestGlobMatchBasics(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"*.go", "main.go", true},
		{"*.go", "src/main.go", true}, // no slash = basename anywhere
		{"*.go", "README.md", false},
		{"/src/", "src/deep/file.go", true}, // dir subtree
		{"/src/deep/", "src/file.go", false},
		{"docs/**/*.md", "docs/a/b/c.md", true},
		{"docs/**/*.md", "docs/a/b/c.txt", false},
		{"/main.go", "main.go", true},
		{"/main.go", "cmd/main.go", false}, // anchored to root
	}
	for _, c := range cases {
		if got := globMatch(c.pattern, c.path); got != c.want {
			t.Errorf("globMatch(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}

func TestLastMatchWins(t *testing.T) {
	rules := parseCODEOWNERS("* @default\n*.go @go\n")
	owners := matchOwners(rules, []string{"main.go"})
	if len(owners) != 1 || owners[0] != "@go" {
		t.Errorf("later *.go must override earlier *, got %v", owners)
	}
}

func TestLastMatchWinsRegardlessOfSpecificity(t *testing.T) {
	// the counterintuitive core of CODEOWNERS: order decides, not length
	rules := parseCODEOWNERS("/src/deep/ @deep\n/src/ @last\n")
	owners := matchOwners(rules, []string{"src/deep/file.go"})
	if len(owners) != 1 || owners[0] != "@last" {
		t.Errorf("later /src/ must win over earlier /src/deep/, got %v", owners)
	}
}

func TestInlineCommentsNotOwners(t *testing.T) {
	rules := parseCODEOWNERS("*.go @go # the go team\n")
	if len(rules) != 1 || len(rules[0].owners) != 1 || rules[0].owners[0] != "@go" {
		t.Errorf("inline comment tokens must not become owners: %+v", rules)
	}
}
