// merge-conflict-detector — checks open PRs when the default branch moves,
// and comments on any that are now unmergeable.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

type PushEvent struct {
	Ref  string `json:"ref"`
	Repo struct {
		FullName      string `json:"full_name"`
		DefaultBranch string `json:"default_branch"`
	} `json:"repository"`
}

func main() {
	cfg := ghbot.LoadConfig("merge-conflict-detector", "8109")
	client := ghbot.NewClient(cfg.Token)

	ghbot.Serve(cfg, "merge-conflict-detector", func(event string, payload []byte) {
		if event != "push" {
			return
		}
		var ev PushEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		if !strings.HasSuffix(ev.Ref, "/"+ev.Repo.DefaultBranch) {
			return
		}

		body, err := client.Do("GET",
			fmt.Sprintf("/repos/%s/pulls?state=open&per_page=50", ev.Repo.FullName), nil)
		if err != nil {
			log.Printf("list prs: %v", err)
			return
		}
		var prs []struct {
			Number int    `json:"number"`
			Title  string `json:"title"`
		}
		json.Unmarshal(body, &prs)

		for _, pr := range prs {
			// mergeable is computed async — fetch, retry once after a delay
			mergeable, conflicted := checkPR(client, ev.Repo.FullName, pr.Number)
			if mergeable == nil {
				time.Sleep(3 * time.Second)
				mergeable, conflicted = checkPR(client, ev.Repo.FullName, pr.Number)
			}
			if conflicted {
				client.Comment(ev.Repo.FullName, pr.Number, fmt.Sprintf(
					"Heads up — this PR now **conflicts with `%s`** after the latest push. "+
						"Please rebase or merge the base branch to resolve it.", ev.Repo.DefaultBranch))
				log.Printf("PR #%d conflicts", pr.Number)
			}
		}
	})
}

func checkPR(client *ghbot.Client, repo string, number int) (*bool, bool) {
	body, err := client.Do("GET", fmt.Sprintf("/repos/%s/pulls/%d", repo, number), nil)
	if err != nil {
		return nil, false
	}
	var pr struct {
		Mergeable *bool `json:"mergeable"`
	}
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, false
	}
	if pr.Mergeable == nil {
		return nil, false // still computing
	}
	return pr.Mergeable, !*pr.Mergeable
}
