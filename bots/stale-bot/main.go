// stale-bot — marks issues/PRs with no activity for STALE_DAYS as stale,
// then closes them CLOSE_DAYS later if nothing happens. Any comment or
// update removes the stale label.
//
// Unlike most bots here this one is cron-style: it polls on an interval
// instead of reacting to webhooks. It still runs the healthz server.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

type issue struct {
	Number    int       `json:"number"`
	UpdatedAt time.Time `json:"updated_at"`
	Labels    []struct {
		Name string `json:"name"`
	} `json:"labels"`
	PullRequest *struct{} `json:"pull_request"`
}

func hasLabel(i issue, name string) bool {
	for _, l := range i.Labels {
		if l.Name == name {
			return true
		}
	}
	return false
}

func sweep(client *ghbot.Client, repo string, staleDays, closeDays int) {
	cutoff := time.Now().Add(-time.Duration(staleDays) * 24 * time.Hour).UTC().Format("2006-01-02T15:04:05Z")
	path := fmt.Sprintf("/repos/%s/issues?state=open&since=%s&per_page=100", repo, cutoff)
	body, err := client.Do("GET", path, nil)
	if err != nil {
		log.Printf("list issues: %v", err)
		return
	}
	var issues []issue
	if err := json.Unmarshal(body, &issues); err != nil {
		log.Printf("parse: %v", err)
		return
	}

	for _, i := range issues {
		kind := "issue"
		if i.PullRequest != nil {
			kind = "pull request"
		}
		idleDays := int(time.Since(i.UpdatedAt).Hours() / 24)

		if hasLabel(i, "stale") {
			if idleDays >= closeDays {
				client.Comment(repo, i.Number,
					"Closing due to inactivity. Happy to reopen if there's interest — just comment.")
				client.Do("PATCH", fmt.Sprintf("/repos/%s/issues/%d", repo, i.Number),
					map[string]string{"state": "closed"})
				log.Printf("closed %s #%d", kind, i.Number)
			}
			continue
		}

		if idleDays >= staleDays {
			client.EnsureLabel(repo, "stale", "ededed", "Inactive — will close without activity")
			client.AddLabels(repo, i.Number, []string{"stale"})
			client.Comment(repo, i.Number, fmt.Sprintf(
				"This %s has been inactive for %d days and is now marked stale. "+
					"It will close in %d days unless there's new activity.", kind, idleDays, closeDays))
			log.Printf("marked %s #%d stale", kind, i.Number)
		}
	}
}

func main() {
	cfg := ghbot.LoadConfig("stale-bot", "8103")
	client := ghbot.NewClient(cfg.Token)

	repo := os.Getenv("REPO") // owner/name — required for cron bots
	if repo == "" {
		log.Fatal("REPO (owner/name) env var is required")
	}
	staleDays := envInt("STALE_DAYS", 30)
	closeDays := envInt("CLOSE_AFTER_DAYS", 7)
	intervalHours := envInt("SWEEP_INTERVAL_HOURS", 12)

	go func() {
		for {
			log.Printf("sweeping %s (stale>%dd, close>%dd)", repo, staleDays, closeDays)
			sweep(client, repo, staleDays, closeDays)
			time.Sleep(time.Duration(intervalHours) * time.Hour)
		}
	}()

	// no webhooks needed — just the health endpoint
	ghbot.Serve(cfg, "stale-bot", func(string, []byte) {})
}
