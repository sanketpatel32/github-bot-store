package main

import "testing"

func TestBumpSize(t *testing.T) {
	cases := []struct {
		title string
		want  string
	}{
		{"Bump lodash from 4.17.20 to 4.17.21", "deps/patch"},
		{"Bump react from 17.0.2 to 18.0.0", "deps/major"},
		{"Bump go from 1.21.0 to 1.23.4", "deps/minor"},
		{"bump serde from 1.0.0 to 1.1.0", "deps/minor"},
		{"feat: add feature", ""},
		{"Bump x to 2.0.0", ""}, // no from-version, can't classify
	}
	for _, c := range cases {
		if got := bumpSize(c.title); got != c.want {
			t.Errorf("bumpSize(%q)=%q want %q", c.title, got, c.want)
		}
	}
}
