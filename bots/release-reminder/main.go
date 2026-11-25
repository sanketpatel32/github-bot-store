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
	// accurate unreleased counting: compare the release TAG against the
	// default branch. Counting commits by published_at misses work that
	// landed before publish time but never made it into the release, and
	// offers no help at all before the first release (#143).
	var count, days int
	body, err := client.Do("GET", fmt.Sprintf("/repos/%s/releases/latest", repo), nil)
	if err == nil {
		var rel struct {
			TagName     string    `json:"tag_name"`
			PublishedAt time.Time `json:"published_at"`
		}
		if json.Unmarshal(body, &rel) == nil && rel.TagName != "" {
			cmp, err := client.Do("GET", fmt.Sprintf("/repos/%s/compare/%s...HEAD", repo, rel.TagName), nil)
			if err == nil {
				var c struct {
					AheadBy int `json:"ahead_by"`
				}
				if json.Unmarshal(cmp, &c) == nil {
					count = c.AheadBy
				}
			}
			days = int(time.Since(rel.PublishedAt).Hours() / 24)
		}
	}
	if count == 0 && days == 0 {
		// no releases at all — every commit is unreleased; remind the
		// maintainer their FIRST release is due too (#143)
		commits, err := client.GetAll(fmt.Sprintf("/repos/%s/commits", repo))
		if err != nil {
			log.Printf("commits: %v", err)
			return
		}
		count = len(commits)
		// age measured from the repo's first commit
		if len(commits) > 0 {
			var first struct {
				Commit struct {
					Committer struct {
						Date time.Time `json:"date"`
					} `json:"committer"`
				} `json:"commit"`
			}
			if json.Unmarshal(commits[len(commits)-1], &first) == nil {
				days = int(time.Since(first.Commit.Committer.Date).Hours() / 24)
			}
		}
	}
	log.Printf("%d unreleased commits (%d days)", count, days)

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
	if ghbot.HandleCLIArgs("release-reminder", cfg, "cron") {
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
