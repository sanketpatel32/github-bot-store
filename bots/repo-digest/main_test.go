package main

import (
	"strings"
	"testing"
	"time"
)

func TestISOWeekPeriod(t *testing.T) {
	// any date must produce a YYYY-Www shaped key and consecutive weeks differ
	d1 := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	p1 := isoWeekPeriod(d1)
	if !strings.Contains(p1, "-W") {
		t.Errorf("bad period format: %s", p1)
	}
	p2 := isoWeekPeriod(d1.Add(7 * 24 * time.Hour))
	if p1 == p2 {
		t.Error("consecutive weeks must produce different periods")
	}
}
