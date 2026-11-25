package main

import "testing"

func TestSizeLabelCustomBounds(t *testing.T) {
	def := []int{20, 100, 300, 800}
	if sizeLabel(5, def) != "size/XS" || sizeLabel(50, def) != "size/S" ||
		sizeLabel(200, def) != "size/M" || sizeLabel(500, def) != "size/L" ||
		sizeLabel(5000, def) != "size/XL" {
		t.Error("default tiers wrong")
	}
	custom := []int{10, 50, 150, 400}
	if sizeLabel(5, custom) != "size/XS" || sizeLabel(300, custom) != "size/L" || sizeLabel(401, custom) != "size/XL" {
		t.Error("custom tiers wrong")
	}
}

func TestIsGenerated(t *testing.T) {
	patterns := generatedPatterns()
	for _, gen := range []string{"web/package-lock.json", "api/go.sum", "x.pb.go", "dist/bundle.min.js"} {
		if !isGenerated(gen, patterns) {
			t.Errorf("%s should be generated", gen)
		}
	}
	for _, real := range []string{"main.go", "src/app.ts", "README.md"} {
		if isGenerated(real, patterns) {
			t.Errorf("%s should NOT be generated", real)
		}
	}
}
