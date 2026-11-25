package main

import (
	"testing"
	"time"
)

func TestLabelForAge(t *testing.T) {
	cases := []struct {
		days int
		want string
	}{
		{0, "aging/fresh"}, {6, "aging/fresh"}, {7, "aging/week"},
		{13, "aging/week"}, {14, "aging/stale"}, {29, "aging/stale"},
		{30, "aging/ancient"}, {90, "aging/ancient"},
	}
	for _, c := range cases {
		if got := labelForAge(time.Duration(c.days) * 24 * time.Hour); got != c.want {
			t.Errorf("age %dd: got %s want %s", c.days, got, c.want)
		}
	}
}
