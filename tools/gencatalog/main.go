// gencatalog — regenerates the website catalog and README bot table from
// manifest.json, the single source of truth. Run after adding a bot:
//
//	go run ./tools/gencatalog
//
// The BOTS array in docs/index.html and the table in README.md are
// GENERATED — edit manifest.json instead.
package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
)

//go:embed botpage.html
var botPage string

type bot struct {
	Name     string   `json:"name"`
	Emoji    string   `json:"emoji"`
	Type     string   `json:"type"`
	Port     int      `json:"port"`
	Accent   string   `json:"accent"`
	Desc     string   `json:"desc"`
	Category string   `json:"category"`
	Events   string   `json:"events"`
	Perms    string   `json:"perms"`
	Notes    []string `json:"notes"`
	// machine-readable generation metadata (#113): the exact GitHub event
	// names the bot consumes, minimal Actions permissions, and required
	// settings beyond GITHUB_TOKEN — the catalog generator emits correct,
	// complete workflows from these instead of guessing from prose.
	GhEvents     []string          `json:"gh_events"`
	ActionsPerms map[string]string `json:"actions_perms"`
	RequiredEnv  []string          `json:"required_env"`
}

func renderJS(bots []bot) string {
	var sb strings.Builder
	for _, b := range bots {
		notes := make([]string, len(b.Notes))
		for i, n := range b.Notes {
			notes[i] = fmt.Sprintf("%q", n)
		}
		fmt.Fprintf(&sb, "  { name: %q, emoji: %q, type: %q, port: %d, accent: %q,\n", b.Name, b.Emoji, b.Type, b.Port, b.Accent)
		fmt.Fprintf(&sb, "    desc: %q, category: %q,\n", b.Desc, b.Category)
		fmt.Fprintf(&sb, "    events: %q, perms: %q,\n", b.Events, b.Perms)
		fmt.Fprintf(&sb, "    notes: [%s],\n", strings.Join(notes, ", "))
		evs := make([]string, len(b.GhEvents))
		for i, e := range b.GhEvents {
			evs[i] = fmt.Sprintf("%q", e)
		}
		var permPairs []string
		for k, v := range b.ActionsPerms {
			permPairs = append(permPairs, fmt.Sprintf("%q: %q", k, v))
		}
		sort.Strings(permPairs)
		envs := make([]string, len(b.RequiredEnv))
		for i, e := range b.RequiredEnv {
			envs[i] = fmt.Sprintf("%q", e)
		}
		fmt.Fprintf(&sb, "    ghEvents: [%s], actionsPerms: {%s}, requiredEnv: [%s] },\n",
			strings.Join(evs, ", "), strings.Join(permPairs, ", "), strings.Join(envs, ", "))
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
	// static per-bot detail pages with SEO/social metadata (#84)
	os.MkdirAll("docs/bots", 0o755)
	for _, b := range bots {
		var notes strings.Builder
		for _, n := range b.Notes {
			notes.WriteString("<li>" + n + "</li>")
		}
		page := strings.ReplaceAll(botPage, "{{NAME}}", b.Name)
		page = strings.ReplaceAll(page, "{{EMOJI}}", b.Emoji)
		page = strings.ReplaceAll(page, "{{DESC}}", b.Desc)
		page = strings.ReplaceAll(page, "{{EVENTS}}", b.Events)
		page = strings.ReplaceAll(page, "{{PERMS}}", b.Perms)
		page = strings.ReplaceAll(page, "{{TYPE}}", b.Type)
		page = strings.ReplaceAll(page, "{{NOTES}}", notes.String())
		page = strings.ReplaceAll(page, "{{PORT}}", fmt.Sprint(b.Port))
		os.WriteFile("docs/bots/"+b.Name+".html", []byte(page), 0o644)
	}
	fmt.Printf("catalog regenerated: %d bots (+ static detail pages)\n", len(bots))
}
