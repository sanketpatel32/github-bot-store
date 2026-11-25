// reviewer-assigner — assigns reviewers to PRs based on which paths changed.
// Configure with a simple JSON mapping of path prefixes to GitHub logins.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

type PREvent struct {
	Action string `json:"action"`
	Number int    `json:"number"`
	PR     struct {
		User struct{ Login string } `json:"user"`
	} `json:"pull_request"`
	Repo struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

// reviewers.json example:
// {"src/frontend/": ["alice"], "src/backend/": ["bob", "carol"], "": ["fallback-dev"]}
func loadConfig() map[string][]string {
	data, err := os.ReadFile("reviewers.json")
	if err != nil {
		log.Printf("no reviewers.json found (%v) — using empty config", err)
		return map[string][]string{}
	}
	var cfg map[string][]string
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("bad reviewers.json: %v", err)
		return map[string][]string{}
	}
	return cfg
}

func main() {
	cfg := ghbot.LoadConfig("reviewer-assigner", "8106")
	client := ghbot.NewClient(cfg.Token)
	rules := loadConfig()

	ghbot.Serve(cfg, "reviewer-assigner", func(event string, payload []byte) {
		if event != "pull_request" {
			return
		}
		var ev PREvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		if ev.Action != "opened" && ev.Action != "synchronize" {
			return
		}

		// fetch changed files
		body, err := client.Do("GET",
			fmt.Sprintf("/repos/%s/pulls/%d/files?per_page=100", ev.Repo.FullName, ev.Number), nil)
		if err != nil {
			log.Printf("files: %v", err)
			return
		}
		var files []struct {
			Filename string `json:"filename"`
		}
		if err := json.Unmarshal(body, &files); err != nil {
			return
		}

		// match prefixes — longest prefix wins per file
		assignees := map[string]bool{}
		for _, f := range files {
			bestLen := -1
			var bestReviewers []string
			for prefix, reviewers := range rules {
				if strings.HasPrefix(f.Filename, prefix) && len(prefix) > bestLen {
					bestLen = len(prefix)
					bestReviewers = reviewers
				}
			}
			for _, r := range bestReviewers {
				if r != ev.PR.User.Login { // never assign the author
					assignees[r] = true
				}
			}
		}

		if len(assignees) == 0 {
			log.Printf("PR #%d: no reviewer matches", ev.Number)
			return
		}
		list := make([]string, 0, len(assignees))
		for a := range assignees {
			list = append(list, a)
		}
		_, err = client.Do("POST",
			fmt.Sprintf("/repos/%s/pulls/%d/requested_reviewers", ev.Repo.FullName, ev.Number),
			map[string]any{"reviewers": list})
		if err != nil {
			log.Printf("assign: %v", err)
			return
		}
		log.Printf("PR #%d: requested reviewers %v", ev.Number, list)
	})
}
