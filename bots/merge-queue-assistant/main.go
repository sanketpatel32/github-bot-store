// merge-queue-assistant — a poor person's merge queue: label PRs
// queue/merge and the bot processes them one at a time in FIFO order.
// The head of the queue gets queue/next; when it merges or closes, the
// following PR is promoted and told to update with main. Prevents the
// "everyone merges green-but-stale PRs and main breaks" problem.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

type PREvent struct {
	Action string `json:"action"`
	Number int    `json:"number"`
	PR     struct {
		Merged bool `json:"merged"`
	} `json:"pull_request"`
	Repo struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func reconcile(client *ghbot.Client, repo string) {
	items, err := client.GetAll(fmt.Sprintf("/repos/%s/pulls?state=open", repo))
	if err != nil {
		log.Printf("list: %v", err)
		return
	}
	type queuedPR struct {
		number    int
		createdAt time.Time
		labels    map[string]bool
	}
	var queue []queuedPR
	for _, raw := range items {
		var pr struct {
			Number    int       `json:"number"`
			CreatedAt time.Time `json:"created_at"`
			Labels    []struct {
				Name string `json:"name"`
			} `json:"labels"`
		}
		if json.Unmarshal(raw, &pr) != nil {
			continue
		}
		lset := map[string]bool{}
		for _, l := range pr.Labels {
			lset[l.Name] = true
		}
		if lset["queue/merge"] {
			queue = append(queue, queuedPR{pr.Number, pr.CreatedAt, lset})
		}
	}
	// FIFO: oldest first
	sort.Slice(queue, func(i, j int) bool { return queue[i].createdAt.Before(queue[j].createdAt) })

	client.EnsureLabel(repo, "queue/merge", "a371f7", "in the merge queue")
	client.EnsureLabel(repo, "queue/next", "3fb950", "head of the merge queue — clear to merge")

	for i, pr := range queue {
		isHead := i == 0
		hasNext := pr.labels["queue/next"]
		if isHead && !hasNext {
			client.AddLabels(repo, pr.number, []string{"queue/next"})
			client.Comment(repo, pr.number,
				"You're at the head of the merge queue. Update with latest `main` and merge when green — the queue holds everyone else back until you land.")
			log.Printf("PR #%d promoted to queue head", pr.number)
		}
		if !isHead && hasNext {
			client.Do("DELETE", fmt.Sprintf("/repos/%s/issues/%d/labels/queue/next", repo, pr.number), nil)
			log.Printf("PR #%d stripped of stale queue/next", pr.number)
		}
	}
	if len(queue) > 1 {
		log.Printf("queue depth: %d", len(queue))
	}
}

func main() {
	cfg := ghbot.LoadConfig("merge-queue-assistant", "8130")
	client := ghbot.NewClient(cfg.Token)

	ghbot.Serve(cfg, "merge-queue-assistant", func(event string, payload []byte) {
		if event != "pull_request" {
			return
		}
		var ev PREvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		// any PR lifecycle change can reshuffle the queue
		switch ev.Action {
		case "labeled", "unlabeled", "closed", "opened", "synchronize":
		default:
			return
		}
		reconcile(client, ev.Repo.FullName)
	})
}
