// pr-aging-bot — cron bot that labels open PRs by how long they've been
// waiting: aging/fresh, aging/week, aging/stale, aging/ancient. Makes the
// review backlog visible at a glance.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

var ageLabels = []struct {
	name  string
	color string
	days  int
}{
	{"aging/fresh", "3fb950", 3},
	{"aging/week", "d29922", 7},
	{"aging/stale", "db6d28", 14},
	{"aging/ancient", "f85149", 30},
}

func labelForAge(age time.Duration) string {
	days := int(age.Hours() / 24)
	switch {
	case days >= 30:
		return "aging/ancient"
	case days >= 14:
		return "aging/stale"
	case days >= 7:
		return "aging/week"
	default:
		return "aging/fresh"
	}
}

func sweep(client *ghbot.Client, repo string) {
	for _, l := range ageLabels {
		client.EnsureLabel(repo, l.name, l.color, fmt.Sprintf("PR open for %d+ days", l.days))
	}
	client.EnsureLabel(repo, "aging/fresh", "3fb950", "PR open less than a week")

	items, err := client.GetAll(fmt.Sprintf("/repos/%s/pulls?state=open", repo))
	if err != nil {
		log.Printf("list pulls: %v", err)
		return
	}
	now := time.Now()
	for _, raw := range items {
		var pr struct {
			Number    int       `json:"number"`
			CreatedAt time.Time `json:"created_at"`
			Draft     bool      `json:"draft"`
			Labels    []struct {
				Name string `json:"name"`
			} `json:"labels"`
		}
		if json.Unmarshal(raw, &pr) != nil {
			continue
		}
		if pr.Draft && os.Getenv("SKIP_DRAFTS") != "false" {
			continue
		}
		want := labelForAge(now.Sub(pr.CreatedAt))
		var current string
		for _, l := range pr.Labels {
			if strings.HasPrefix(l.Name, "aging/") {
				current = l.Name
			}
		}
		if current == want {
			continue
		}
		if current != "" {
			client.Do("DELETE", fmt.Sprintf("/repos/%s/issues/%d/labels/%s", repo, pr.Number, current), nil)
		}
		if err := client.AddLabels(repo, pr.Number, []string{want}); err != nil {
			log.Printf("label PR #%d: %v", pr.Number, err)
			continue
		}
		log.Printf("PR #%d: %s -> %s", pr.Number, current, want)
	}
}

func main() {
	cfg := ghbot.LoadConfig("pr-aging-bot", "8116")
	client := ghbot.NewClient(cfg.Token)
	repo := os.Getenv("REPO")
	if repo == "" {
		log.Fatal("REPO is not set (owner/repo)")
	}
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		sweep(client, repo)
		return
	}
	sweep(client, repo)
	log.Printf("sweep complete — health server keeps running")
	ghbot.ServeHealth(cfg, "pr-aging-bot")
}
