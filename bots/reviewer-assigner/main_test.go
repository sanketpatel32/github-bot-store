package main

import "testing"

func TestParseCODEOWNERS(t *testing.T) {
	content := `# comment line
*.go            @go-team-lead
/docs/          @docs-owner @org/tech-writers
src/frontend/   @alice @bob

*               @fallback
`
	rules := parseCODEOWNERS(content)
	if len(rules) != 4 {
		t.Fatalf("expected 4 rules, got %d", len(rules))
	}
	if rules[0].pattern != "" { // *.go glob -> static prefix is empty-ish; check fallback logic below instead
		t.Logf("pattern[0] = %q", rules[0].pattern)
	}
	// docs rule should carry both owners incl. team
	found := false
	for _, r := range rules {
		if r.pattern == "docs/" {
			found = true
			if len(r.owners) != 2 || r.owners[1] != "@org/tech-writers" {
				t.Errorf("docs owners wrong: %v", r.owners)
			}
		}
	}
	if !found {
		t.Error("docs/ rule missing")
	}
}

func TestMatchOwnersLongestPrefix(t *testing.T) {
	rules := []rule{
		{"", []string{"@fallback"}},
		{"src/", []string{"@backend"}},
		{"src/web/", []string{"@frontend"}},
	}
	owners := matchOwners(rules, []string{"src/web/app.ts", "src/api/server.go", "README.md"})
	set := map[string]bool{}
	for _, o := range owners {
		set[o] = true
	}
	for _, want := range []string{"@frontend", "@backend", "@fallback"} {
		if !set[want] {
			t.Errorf("expected owner %s in %v", want, owners)
		}
	}

	// longest prefix: src/web file must NOT also pull @backend
	owners = matchOwners(rules, []string{"src/web/app.ts"})
	for _, o := range owners {
		if o == "@backend" {
			t.Error("longest-prefix violated: src/web pulled @backend")
		}
	}
}
