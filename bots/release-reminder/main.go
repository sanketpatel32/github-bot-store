// release-reminder — cron bot that opens (or updates) a tracking issue when
// unreleased commits pile up on the default branch. Nudges maintainers to
// cut a release before the diff gets scary.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

const marker = "<!-- release-reminder -->"

func sweep(client *ghbot.Client, repo string, threshold int) {
	// latest release (or tag) date
	var since time.Time
	body, err := client.Do("GET", fmt.Sprintf("/repos/%s/releases/latest", repo), nil)
	if err == nil {
		var rel struct {
			PublishedAt time.Time `json:"published_at"`
		}
		if json.Unmarshal(body, &rel) == nil {
			since = rel.PublishedAt
		}
	}
	if since.IsZero() {
		// no releases at all — fall back to first commit or just report total
		log.Printf("no releases found — nothing to compare against yet")
		return
	}

	commits, err := client.GetAll(fmt.Sprintf("/repos/%s/commits?since=%s",
		repo, since.UTC().Format(time.RFC3339)))
	if err != nil {
		log.Printf("commits: %v", err)
		return
	}
	count := len(commits)
	days := int(time.Since(since).Hours() / 24)
	log.Printf("%d commits since last release (%d days ago)", count, days)

	title := fmt.Sprintf("Release reminder: %d unreleased commits (%d days)", count, days)
	bodyText := fmt.Sprintf(`%s
The last release was **%d days ago** and **%d commits** have landed since.

Consider cutting a release. Configured threshold: %d commits (THRESHOLD_COMMITS).

_This issue updates itself; close it when you release._`,
		marker, days, count, threshold)

	// find existing managed issue
	existing := 0
	search, err := client.Do("GET", fmt.Sprintf(
		"/search/issues?q=repo:%s+type:issue+state:open+in:body+%s&per_page=1", repo, marker), nil)
	if err == nil {
		var result struct {
			Items []struct {
				Number int `json:"number"`
			} `json:"items"`
		}
		if json.Unmarshal(search, &result) == nil && len(result.Items) > 0 {
			existing = result.Items[0].Number
		}
	}

	if count < threshold {
		// below threshold — close any open reminder
		if existing != 0 {
			client.Comment(repo, existing, "Threshold no longer met — closing.")
			client.Do("PATCH", fmt.Sprintf("/repos/%s/issues/%d", repo, existing),
				map[string]string{"state": "closed"})
		}
		return
	}

	if existing != 0 {
		client.Do("PATCH", fmt.Sprintf("/repos/%s/issues/%d", repo, existing),
			map[string]string{"title": title, "body": bodyText})
		log.Printf("updated reminder issue #%d", existing)
	} else {
		_, err := client.Do("POST", fmt.Sprintf("/repos/%s/issues", repo),
			map[string]any{"title": title, "body": bodyText, "labels": []string{"release"}})
		if err != nil {
			log.Printf("create: %v", err)
			return
		}
		log.Printf("opened release reminder issue")
	}
}

func main() {
	cfg := ghbot.LoadConfig("release-reminder", "8124")
	if ghbot.HandleCLIArgs("release-reminder", cfg) {
		return
	}
	client := ghbot.NewClient(cfg.Token)
	repo := os.Getenv("REPO")
	if repo == "" {
		log.Fatal("REPO is not set (owner/repo)")
	}
	threshold := 20
	if v := os.Getenv("THRESHOLD_COMMITS"); v != "" {
		fmt.Sscanf(v, "%d", &threshold)
	}
	ghbot.ServeCron(cfg, "release-reminder", ghbot.SweepInterval(24), func() { sweep(client, repo, threshold) })
}
