package main

import "testing"

func TestIsBreaking(t *testing.T) {
	cases := []struct {
		title, body string
		want        bool
	}{
		{"feat: add x", "", false},
		{"feat!: drop api", "", true},
		{"fix(api)!: change return", "", true},
		{"feat: x", "Some text\nBREAKING CHANGE: removes endpoint", true},
		{"feat: x", "breaking-change: yep", true},
		{"docs: typo", "nothing here", false},
	}
	for _, c := range cases {
		if got := isBreaking(c.title, c.body); got != c.want {
			t.Errorf("isBreaking(%q,%q)=%v want %v", c.title, c.body, got, c.want)
		}
	}
}
