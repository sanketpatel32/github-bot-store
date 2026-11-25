// review-reminder — cron bot that nudges requested reviewers who haven't
// responded within a configurable window. One reminder per reviewer per PR,
// tracked via a managed comment marker.
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

const marker = "<!-- review-reminder -->"

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func sweep(client *ghbot.Client, repo string, waitHours int) {
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
		}
		if json.Unmarshal(raw, &pr) != nil || pr.Draft {
			continue
		}

		body, err := client.Do("GET",
			fmt.Sprintf("/repos/%s/pulls/%d/requested_reviewers", repo, pr.Number), nil)
		if err != nil {
			continue
		}
		var rr struct {
			Users []struct {
				Login string `json:"login"`
			} `json:"users"`
		}
		if json.Unmarshal(body, &rr) != nil || len(rr.Users) == 0 {
			continue
		}

		// has the PR been waiting long enough? use last update as baseline
		var prDetail struct {
			UpdatedAt time.Time `json:"updated_at"`
		}
		detail, err := client.Do("GET", fmt.Sprintf("/repos/%s/pulls/%d", repo, pr.Number), nil)
		if err != nil || json.Unmarshal(detail, &prDetail) != nil {
			continue
		}
		if now.Sub(prDetail.UpdatedAt) < time.Duration(waitHours)*time.Hour {
			continue
		}

		// already reminded? look for our managed comment mentioning them
		comments, err := client.GetAll(fmt.Sprintf("/repos/%s/issues/%d/comments", repo, pr.Number))
		if err != nil {
			continue
		}
		var pending []string
		for _, u := range rr.Users {
			reminded := false
			for _, craw := range comments {
				var c struct {
					Body string `json:"body"`
				}
				if json.Unmarshal(craw, &c) != nil {
					continue
				}
				if strings.Contains(c.Body, marker) && strings.Contains(c.Body, "@"+u.Login) {
					reminded = true
					break
				}
			}
			if !reminded {
				pending = append(pending, u.Login)
			}
		}
		if len(pending) == 0 {
			continue
		}
		mentions := make([]string, len(pending))
		for i, p := range pending {
			mentions[i] = "@" + p
		}
		client.Comment(repo, pr.Number, fmt.Sprintf(
			"%s\n%s — this PR has been waiting %d+ hours for review. Friendly nudge. 🙂",
			marker, strings.Join(mentions, " "), waitHours))
		log.Printf("PR #%d: reminded %v", pr.Number, pending)
	}
}

// sweepInterval reads SWEEP_INTERVAL_HOURS (default 24).
func sweepInterval() time.Duration {
	h := 24
	if v := os.Getenv("SWEEP_INTERVAL_HOURS"); v != "" {
		fmt.Sscanf(v, "%d", &h)
	}
	return time.Duration(h) * time.Hour
}

func main() {
	cfg := ghbot.LoadConfig("review-reminder", "8117")
	if ghbot.HandleCLIArgs("review-reminder", cfg) {
		return
	}
	client := ghbot.NewClient(cfg.Token)
	repo := os.Getenv("REPO")
	if repo == "" {
		log.Fatal("REPO is not set (owner/repo)")
	}
	wait := envInt("REMIND_AFTER_HOURS", 24)
	ghbot.ServeCron(cfg, "review-reminder", sweepInterval(), func() { sweep(client, repo, wait) })
}
