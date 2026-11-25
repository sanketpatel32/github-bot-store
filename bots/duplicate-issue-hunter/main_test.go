package main

import "testing"

func TestKeywordsUnicode(t *testing.T) {
	// multilingual titles (#25): non-latin scripts must yield keywords
	words := keywords("Fehler beim Starten der Anwendung")
	if len(words) == 0 {
		t.Error("german title yielded no keywords")
	}
	words = keywords("  ")
	if len(words) == 0 {
		t.Error("cyrillic title yielded no keywords")
	}
	words = keywords("")
	if len(words) == 0 {
		t.Error("japanese title yielded no keywords")
	}
}

func TestSimilarity(t *testing.T) {
	a := keywords("crash on startup")
	if similarity(a, a) < 90 {
		t.Error("identical titles should score near 100")
	}
	b := keywords("totally unrelated networking issue")
	if similarity(a, b) >= 40 {
		t.Error("unrelated titles should score below threshold")
	}
	c := keywords("crash on startup windows")
	score := similarity(a, c)
	if score < 50 {
		t.Errorf("near-duplicate scored only %d", score)
	}
}
