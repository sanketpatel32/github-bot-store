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

		// how long has each reviewer actually been waiting? updated_at is
		// wrong — new commits and edits reset it while the review request
		// keeps aging. The timeline's review_requested events carry the real
		// request timestamps (#134).
		timeline, err := client.GetAll(fmt.Sprintf("/repos/%s/issues/%d/timeline", repo, pr.Number))
		if err != nil {
			continue
		}
		requestedAt := map[string]time.Time{}
		for _, traw := range timeline {
			var t struct {
				Event     string    `json:"event"`
				CreatedAt time.Time `json:"created_at"`
				Reviewer  struct {
					Login string `json:"login"`
				} `json:"requested_reviewer"`
			}
			if json.Unmarshal(traw, &t) != nil {
				continue
			}
			// latest request wins — re-requests restart the clock
			if t.Event == "review_requested" && t.Reviewer.Login != "" {
				requestedAt[t.Reviewer.Login] = t.CreatedAt
			}
		}
		waiting := false
		for _, u := range rr.Users {
			at, ok := requestedAt[u.Login]
			if !ok {
				at = now // never formally requested via events — treat as fresh
			}
			if now.Sub(at) >= time.Duration(waitHours)*time.Hour {
				waiting = true
				break
			}
		}
		if !waiting {
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
				if strings.Contains(c.Body, marker) && mentionsUser(c.Body, u.Login) {
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

// mentionsUser reports whether body contains @login as a complete mention —
// substring matching makes a reminder for @bob suppress the one owed to
// @bobsmith (#134).
func mentionsUser(body, login string) bool {
	needle := "@" + login
	for idx := 0; ; {
		i := strings.Index(body[idx:], needle)
		if i == -1 {
			return false
		}
		pos := idx + i + len(needle)
		if pos >= len(body) {
			return true
		}
		c := body[pos]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' {
			return true
		}
		idx = pos
	}
}

func main() {
	cfg := ghbot.LoadConfig("review-reminder", "8117")
	if ghbot.HandleCLIArgs("review-reminder", cfg, "cron") {
		return
	}
	client := ghbot.NewClient(cfg.Token)
	repo := os.Getenv("REPO")
	if repo == "" {
		log.Fatal("REPO is not set (owner/repo)")
	}
	wait := envInt("REMIND_AFTER_HOURS", 24)
	ghbot.ServeCron(cfg, "review-reminder", ghbot.SweepInterval(24), func() { sweep(client, repo, wait) })
}
