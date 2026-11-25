// gencatalog — regenerates the website catalog and README bot table from
// manifest.json, the single source of truth. Run after adding a bot:
//
//	go run ./tools/gencatalog
//
// The BOTS array in docs/index.html and the table in README.md are
// GENERATED — edit manifest.json instead.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

type bot struct {
	Name   string   `json:"name"`
	Emoji  string   `json:"emoji"`
	Type   string   `json:"type"`
	Port   int      `json:"port"`
	Accent string   `json:"accent"`
	Desc   string   `json:"desc"`
	Events string   `json:"events"`
	Perms  string   `json:"perms"`
	Notes  []string `json:"notes"`
}

func renderJS(bots []bot) string {
	var sb strings.Builder
	for _, b := range bots {
		notes := make([]string, len(b.Notes))
		for i, n := range b.Notes {
			notes[i] = fmt.Sprintf("%q", n)
		}
		fmt.Fprintf(&sb, "  { name: %q, emoji: %q, type: %q, port: %d, accent: %q,\n", b.Name, b.Emoji, b.Type, b.Port, b.Accent)
		fmt.Fprintf(&sb, "    desc: %q,\n", b.Desc)
		fmt.Fprintf(&sb, "    events: %q, perms: %q,\n", b.Events, b.Perms)
		fmt.Fprintf(&sb, "    notes: [%s] },\n", strings.Join(notes, ", "))
	}
	return sb.String()
}

func renderTable(bots []bot) string {
	var sb strings.Builder
	for _, b := range bots {
		fmt.Fprintf(&sb, "| [%s](bots/%s) | %s | %s |\n", b.Name, b.Name, b.Desc, b.Type)
	}
	return sb.String()
}

func main() {
	data, err := os.ReadFile("manifest.json")
	if err != nil {
		log.Fatal("manifest.json: ", err)
	}
	var bots []bot
	if err := json.Unmarshal(data, &bots); err != nil {
		log.Fatal("parse manifest: ", err)
	}

	// splice into docs/index.html between the BOTS markers
	html, err := os.ReadFile("docs/index.html")
	if err != nil {
		log.Fatal("index.html: ", err)
	}
	const begin = "const BOTS = ["
	s := string(html)
	start := strings.Index(s, begin)
	if start == -1 {
		log.Fatal("BOTS array not found in index.html")
	}
	arrStart := start + len(begin)
	end := strings.Index(s[arrStart:], "\n];")
	if end == -1 {
		log.Fatal("BOTS array end not found")
	}
	s = s[:arrStart] + "\n" + renderJS(bots) + s[arrStart+end:]
	os.WriteFile("docs/index.html", []byte(s), 0o644)

	// rebuild README table rows
	readme, err := os.ReadFile("README.md")
	if err != nil {
		log.Fatal("README.md: ", err)
	}
	lines := strings.Split(string(readme), "\n")
	var out []string
	inTable := false
	tableDone := false
	for _, line := range lines {
		if strings.HasPrefix(line, "| [") && !tableDone {
			if !inTable {
				inTable = true
				out = append(out, strings.TrimSuffix(renderTable(bots), "\n"))
			}
			continue // drop old rows
		}
		if inTable {
			inTable = false
			tableDone = true
		}
		out = append(out, line)
	}
	os.WriteFile("README.md", []byte(strings.Join(out, "\n")), 0o644)
	fmt.Printf("catalog regenerated: %d bots\n", len(bots))
}
