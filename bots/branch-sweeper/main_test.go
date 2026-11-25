package main

import "testing"

func TestProtectedByPattern(t *testing.T) {
	patterns := []string{"main", "master", "release/*", ""}
	cases := map[string]bool{
		"main":           true,
		"master":         true,
		"release/1.0":    true,
		"release/hotfix": true,
		"feat/login":     false,
		"fix-typo":       false,
		"mainline":       false, // prefix must not match exact-name pattern
		"releases/1.0":   false, // plural doesn't match release/*
	}
	for branch, want := range cases {
		if got := protectedByPattern(branch, patterns); got != want {
			t.Errorf("protectedByPattern(%q) = %v, want %v", branch, got, want)
		}
	}
}
