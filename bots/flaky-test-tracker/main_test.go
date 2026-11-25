package main

import "testing"

func TestStateRoundtrip(t *testing.T) {
	st := loadState() // fresh env: no file -> empty maps, must not panic
	if st.LastSeen == nil || st.Flakes == nil {
		t.Error("empty state must initialize maps")
	}
	st.Flakes["check"] = 2
	st.save() // writes flaky-state.json in cwd — acceptable in test sandbox
	st2 := loadState()
	if st2.Flakes["check"] != 2 {
		t.Error("state did not roundtrip through disk")
	}
}
