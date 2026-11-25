// duplicate-issue-hunter — when a new issue opens, searches for similar
// open/closed issues by title keywords and comments with the best matches.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

type IssueEvent struct {
	Action string `json:"action"`
	Issue  struct {
		Number int                    `json:"number"`
		Title  string                 `json:"title"`
		User   struct{ Login string } `json:"user"`
	} `json:"issue"`
	Repo struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "is": true, "in": true, "on": true,
	"to": true, "of": true, "and": true, "for": true, "with": true, "when": true,
	"i": true, "it": true, "this": true, "that": true, "not": true, "cant": true, "doesnt": true,
}

var wordRe = regexp.MustCompile(`[a-z0-9]+`)

func keywords(title string) []string {
	words := wordRe.FindAllString(strings.ToLower(title), -1)
	seen := map[string]bool{}
	var out []string
	for _, w := range words {
		if len(w) > 2 && !stopWords[w] && !seen[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	return out
}

func main() {
	cfg := ghbot.LoadConfig("duplicate-issue-hunter", "8107")
	client := ghbot.NewClient(cfg.Token)

	ghbot.Serve(cfg, "duplicate-issue-hunter", func(event string, payload []byte) {
		if event != "issues" {
			return
		}
		var ev IssueEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		if ev.Action != "opened" {
			return
		}

		words := keywords(ev.Issue.Title)
		if len(words) == 0 {
			return
		}
		// search with the top keywords
		query := strings.Join(words[:min(4, len(words))], " ")
		path := fmt.Sprintf("/search/issues?q=repo:%s+type:issue+%s&per_page=5&sort=updated",
			ev.Repo.FullName, url.QueryEscape(query))
		body, err := client.Do("GET", path, nil)
		if err != nil {
			log.Printf("search: %v", err)
			return
		}
		var result struct {
			Items []struct {
				Number int    `json:"number"`
				Title  string `json:"title"`
				State  string `json:"state"`
				URL    string `json:"html_url"`
			} `json:"items"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return
		}

		// score overlap, skip self
		type match struct {
			number, score int
			title, state  string
		}
		var matches []match
		for _, item := range result.Items {
			if item.Number == ev.Issue.Number {
				continue
			}
			overlap := 0
			itemWords := keywords(item.Title)
			for _, w := range words {
				for _, iw := range itemWords {
					if w == iw {
						overlap++
					}
				}
			}
			if overlap >= 2 {
				matches = append(matches, match{item.Number, overlap, item.Title, item.State})
			}
		}
		if len(matches) == 0 {
			return
		}
		sort.Slice(matches, func(i, j int) bool { return matches[i].score > matches[j].score })
		if len(matches) > 3 {
			matches = matches[:3]
		}

		var b strings.Builder
		b.WriteString("This issue looks similar to some existing ones — worth a quick check before diving in:\n\n")
		for _, m := range matches {
			b.WriteString(fmt.Sprintf("- #%d (%s) — %s\n", m.number, m.state, m.title))
		}
		b.WriteString("\nIf one of them covers it, we can close this as a duplicate. If not, carry on!")
		if err := client.Comment(ev.Repo.FullName, ev.Issue.Number, b.String()); err != nil {
			log.Printf("comment: %v", err)
			return
		}
		log.Printf("flagged %d possible duplicates on #%d", len(matches), ev.Issue.Number)
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
