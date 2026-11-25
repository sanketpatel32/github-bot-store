// merge-conflict-detector — when any base branch moves, checks PRs targeting
// that exact branch and maintains ONE warning comment per PR: posted on the
// clean→conflicted transition, edited on repeat checks, marked resolved when
// the conflict clears.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

const marker = "<!-- merge-conflict-detector -->"

type PushEvent struct {
	Ref  string `json:"ref"` // refs/heads/<branch>
	Repo struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func branchFromRef(ref string) string {
	return strings.TrimPrefix(ref, "refs/heads/")
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
		branch := branchFromRef(ev.Ref)
		if branch == ev.Ref {
			return // not a branch push (tag, etc.)
		}

		// only PRs targeting THIS branch — a develop push shouldn't warn
		// main-targeted PRs, and non-default targets are covered too
		rawPRs, err := client.GetAll(fmt.Sprintf(
			"/repos/%s/pulls?state=open&base=%s", ev.Repo.FullName, branch))
		if err != nil {
			log.Printf("list prs: %v", err)
			return
		}

		for _, raw := range rawPRs {
			var pr struct {
				Number int `json:"number"`
			}
			if json.Unmarshal(raw, &pr) != nil {
				continue
			}
			checkOne(client, ev.Repo.FullName, pr.Number, branch)
		}
	})
}

func checkOne(client *ghbot.Client, repo string, number int, branch string) {
	// mergeable is computed async — bounded retries, then give up quietly
	var mergeable *bool
	for attempt := 0; attempt < 3; attempt++ {
		mergeable = fetchMergeable(client, repo, number)
		if mergeable != nil {
			break
		}
		time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
	}
	if mergeable == nil {
		log.Printf("PR #%d mergeability unknown after retries — skipping", number)
		return
	}

	conflicted := !*mergeable
	existing := findWarningComment(client, repo, number)

	switch {
	case conflicted && existing == 0:
		// clean -> conflicted transition: post the one warning
		body := marker + "\n" + fmt.Sprintf(
			"⚠️ This PR now **conflicts with `%s`** after the latest push. "+
				"Please rebase or merge the base branch to resolve it.", branch)
		if err := client.Comment(repo, number, body); err != nil {
			log.Printf("comment: %v", err)
			return
		}
		log.Printf("PR #%d newly conflicted", number)
	case conflicted && existing != 0:
		// still conflicted — nothing to do, the warning stands
		log.Printf("PR #%d still conflicted (no repeat spam)", number)
	case !conflicted && existing != 0:
		// conflict resolved — mark the same comment resolved
		client.Do("PATCH", fmt.Sprintf("/repos/%s/issues/comments/%d", repo, existing),
			map[string]string{"body": marker + "\n✅ Conflicts resolved — this PR merges cleanly again."})
		log.Printf("PR #%d conflict resolved", number)
	}
}

func fetchMergeable(client *ghbot.Client, repo string, number int) *bool {
	body, err := client.Do("GET", fmt.Sprintf("/repos/%s/pulls/%d", repo, number), nil)
	if err != nil {
		return nil
	}
	var pr struct {
		Mergeable *bool `json:"mergeable"`
	}
	if json.Unmarshal(body, &pr) != nil {
		return nil
	}
	return pr.Mergeable
}

func findWarningComment(client *ghbot.Client, repo string, number int) int64 {
	body, err := client.Do("GET",
		fmt.Sprintf("/repos/%s/issues/%d/comments?per_page=100", repo, number), nil)
	if err != nil {
		return 0
	}
	var comments []struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
	}
	if json.Unmarshal(body, &comments) != nil {
		return 0
	}
	for _, c := range comments {
		if strings.Contains(c.Body, marker) {
			return c.ID
		}
	}
	return 0
}
