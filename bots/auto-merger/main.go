// auto-merger — merges PRs labeled "automerge" once all checks pass.
// Reacts to check_suite/check_run/status events and PR labeling.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

type genericEvent struct {
	Action string `json:"action"`
	Repo   struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	PR *struct {
		Number int `json:"number"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	} `json:"pull_request"`
}

type prInfo struct {
	State  string `json:"state"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Head struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

func fetchPR(client *ghbot.Client, repo string, number int) (*prInfo, error) {
	body, err := client.Do("GET", fmt.Sprintf("/repos/%s/pulls/%d", repo, number), nil)
	if err != nil {
		return nil, err
	}
	var pr prInfo
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}

func (p *prInfo) automergeable() bool {
	if p.State != "open" {
		return false
	}
	for _, l := range p.Labels {
		if l.Name == "automerge" {
			return true
		}
	}
	return false
}

func checksGreen(client *ghbot.Client, repo, sha string) bool {
	statusBody, err := client.Do("GET",
		fmt.Sprintf("/repos/%s/commits/%s/status", repo, sha), nil)
	if err != nil {
		return false
	}
	var status struct {
		State      string `json:"state"`
		TotalCount int    `json:"total_count"`
	}
	json.Unmarshal(statusBody, &status)
	// no checks at all counts as green (repos without CI); failure/pending blocks
	return status.State == "success" || status.TotalCount == 0
}

func tryMerge(client *ghbot.Client, repo string, number int) {
	pr, err := fetchPR(client, repo, number)
	if err != nil || !pr.automergeable() {
		return
	}
	if !checksGreen(client, repo, pr.Head.SHA) {
		log.Printf("PR #%d: automerge set but checks not green yet", number)
		return
	}
	_, err = client.Do("PUT", fmt.Sprintf("/repos/%s/pulls/%d/merge", repo, number),
		map[string]string{"merge_method": "squash"})
	if err != nil {
		log.Printf("merge PR #%d: %v", number, err)
		return
	}
	log.Printf("merged PR #%d", number)
}

func main() {
	cfg := ghbot.LoadConfig("auto-merger", "8110")
	client := ghbot.NewClient(cfg.Token)

	ghbot.Serve(cfg, "auto-merger", func(event string, payload []byte) {
		var ev genericEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		repo := ev.Repo.FullName

		switch event {
		case "pull_request":
			if ev.PR == nil {
				return
			}
			if ev.Action == "labeled" || ev.Action == "opened" || ev.Action == "synchronize" {
				time.Sleep(2 * time.Second) // let statuses post
				tryMerge(client, repo, ev.PR.Number)
			}
		case "check_suite", "check_run", "status":
			// a check completed — scan open automerge PRs
			items, err := client.GetAll(fmt.Sprintf("/repos/%s/pulls?state=open", repo))
			if err != nil {
				return
			}
			for _, raw := range items {
				var pr struct {
					Number int `json:"number"`
				}
				if json.Unmarshal(raw, &pr) == nil {
					tryMerge(client, repo, pr.Number)
				}
			}
		}
	})
}
