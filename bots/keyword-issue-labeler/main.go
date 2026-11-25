// keyword-issue-labeler — auto-labels new issues from keywords found in the
// title and body. Configure with labels.json; ships with sensible defaults.
package main

import (
	"encoding/json"
	"log"
	"os"
	"strings"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

type IssueEvent struct {
	Action string `json:"action"`
	Issue  struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
	} `json:"issue"`
	Repo struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

// defaultRules maps label -> keywords (lowercase substring match).
var defaultRules = map[string][]string{
	"bug":         {"bug", "broken", "crash", "error", "fails", "failure", "regression", "panic", "exception"},
	"feature":     {"feature", "add support", "would be nice", "suggestion", "enhancement", "could we"},
	"docs":        {"docs", "documentation", "readme", "typo", "example"},
	"question":    {"question", "how do", "how to", "is it possible", "help"},
	"performance": {"slow", "performance", "memory", "leak", "latency"},
	"security":    {"security", "vulnerability", "cve", "xss", "injection"},
}

func loadRules() map[string][]string {
	data, err := os.ReadFile("labels.json")
	if err != nil {
		return defaultRules
	}
	var rules map[string][]string
	if json.Unmarshal(data, &rules) != nil {
		return defaultRules
	}
	return rules
}

func main() {
	cfg := ghbot.LoadConfig("keyword-issue-labeler", "8114")
	if ghbot.HandleCLIArgs("keyword-issue-labeler", cfg, "webhook") {
		return
	}
	client := ghbot.NewClient(cfg.Token)
	rules := loadRules()

	ghbot.Serve(cfg, "keyword-issue-labeler", func(event string, payload []byte) {
		if event != "issues" {
			return
		}
		var ev IssueEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		if ev.Action != "opened" && ev.Action != "edited" {
			return
		}

		text := strings.ToLower(ev.Issue.Title + "\n" + ev.Issue.Body)
		var matched []string
		for label, keywords := range rules {
			for _, kw := range keywords {
				if strings.Contains(text, kw) {
					matched = append(matched, label)
					break
				}
			}
		}
		if len(matched) == 0 {
			return
		}
		for _, label := range matched {
			client.EnsureLabel(ev.Repo.FullName, label, "ededed", "auto-applied by keyword-issue-labeler")
		}
		if err := client.AddLabels(ev.Repo.FullName, ev.Issue.Number, matched); err != nil {
			log.Printf("labels: %v", err)
			return
		}
		log.Printf("issue #%d labeled %v", ev.Issue.Number, matched)
	})
}
