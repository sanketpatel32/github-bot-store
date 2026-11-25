package main

import "testing"

func TestKeywordsUnicode(t *testing.T) {
	// multilingual titles (#25): non-latin scripts must yield keywords.
	// escapes used so this file stays ASCII-safe in any editor/terminal.
	words := keywords("Fehler beim Starten der Anwendung")
	if len(words) == 0 {
		t.Error("german title yielded no keywords")
	}
	words = keywords("\u043e\u0448\u0438\u0431\u043a\u0430 \u043f\u0440\u0438 \u0437\u0430\u043f\u0443\u0441\u043a\u0435")
	if len(words) == 0 {
		t.Error("cyrillic title yielded no keywords")
	}
	words = keywords("\u30a8\u30e9\u30fc\u304c\u767a\u751f\u3057\u307e\u3057\u305f")
	if len(words) == 0 {
		t.Error("japanese title yielded no keywords")
	}
}

func TestSimilarity(t *testing.T) {
	a := keywords("crash on startup")
	if similarity(a, a) < 90 {
		t.Errorf("identical titles should score near 100, got %d", similarity(a, a))
	}
	b := keywords("totally unrelated networking issue")
	if similarity(a, b) >= 40 {
		t.Error("unrelated titles should score below threshold")
	}
	c := keywords("crash on startup windows")
	if score := similarity(a, c); score < 50 {
		t.Errorf("near-duplicate scored only %d", score)
	}
}
