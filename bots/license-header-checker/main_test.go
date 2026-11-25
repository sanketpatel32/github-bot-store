package main

import "testing"

func TestSourceExt(t *testing.T) {
	for _, yes := range []string{"main.go", "src/app.ts", "x.py", "lib.rs"} {
		if !sourceExt(yes) {
			t.Errorf("%s should be a source file", yes)
		}
	}
	for _, no := range []string{"README.md", "image.png", "data.json", "Makefile"} {
		if sourceExt(no) {
			t.Errorf("%s should not be a source file", no)
		}
	}
}
