// auto-merger — merges PRs labeled "automerge" once all checks pass.
// Evaluates both modern check runs and legacy commit statuses, and only
// merges the exact validated head SHA.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
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
	} `json:"pull_request"`
}

type prInfo struct {
	State     string `json:"state"`
	Mergeable *bool  `json:"mergeable"`
	Head      struct {
		SHA string `json:"sha"`
	} `json:"head"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

// mergeMethod reads MERGE_METHOD (squash|merge|rebase, default squash) —
// validated at use so a typo can't silently pick the wrong strategy (#28).
func mergeMethod() string {
	m := os.Getenv("MERGE_METHOD")
	switch m {
	case "", "squash":
		return "squash"
	case "merge", "rebase":
		return m
	default:
		log.Printf("MERGE_METHOD %q invalid — using squash", m)
		return "squash"
	}
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

// checksGreen evaluates check runs (modern) plus the legacy combined status.
// Anything still running or explicitly failing blocks the merge; success,
// skipped and neutral conclusions pass. A sha with no checks at all counts
// as green (repos without CI).
func checksGreen(client *ghbot.Client, repo, sha string) bool {
	// 1. check runs — paginated: the endpoint caps at 100 per page and a sha
	// with more runs must still have ALL of them evaluated (#106)
	type runList struct {
		TotalCount int `json:"total_count"`
		CheckRuns  []struct {
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
		} `json:"check_runs"`
	}
	var runs runList
	for page := 1; ; page++ {
		runsBody, err := client.Do("GET",
			fmt.Sprintf("/repos/%s/commits/%s/check-runs?per_page=100&page=%d", repo, sha, page), nil)
		if err != nil {
			log.Printf("check-runs fetch: %v", err)
			return false
		}
		var chunk runList
		if json.Unmarshal(runsBody, &chunk) != nil {
			return false
		}
		runs.TotalCount = chunk.TotalCount
		runs.CheckRuns = append(runs.CheckRuns, chunk.CheckRuns...)
		if len(runs.CheckRuns) >= chunk.TotalCount || len(chunk.CheckRuns) == 0 {
			break
		}
	}
	if len(runs.CheckRuns) < runs.TotalCount {
		log.Printf("warning: only saw %d of %d check runs for %.7s — refusing to merge blind",
			len(runs.CheckRuns), runs.TotalCount, sha)
		return false
	}
	for _, r := range runs.CheckRuns {
		if r.Status != "completed" {
			return false // queued or in progress
		}
		switch r.Conclusion {
		case "success", "skipped", "neutral":
		default:
			return false // failure, cancelled, timed_out, action_required
		}
	}

	// 2. legacy commit statuses (some CI still only posts these)
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
	if status.TotalCount > 0 && status.State != "success" {
		return false
	}
	return true
}

func tryMerge(client *ghbot.Client, repo string, number int) {
	// re-read fresh state immediately before deciding — the event that woke
	// us may be stale
	pr, err := fetchPR(client, repo, number)
	if err != nil {
		return
	}
	if !pr.automergeable() {
		return
	}
	if pr.Mergeable != nil && !*pr.Mergeable {
		log.Printf("PR #%d: automerge set but not mergeable (conflicts?)", number)
		return
	}
	headSHA := pr.Head.SHA
	if !checksGreen(client, repo, headSHA) {
		log.Printf("PR #%d: automerge set but checks not green yet", number)
		return
	}
	// pin the merge to the exact head we validated — if the branch moved
	// since, GitHub rejects the merge instead of merging unreviewed commits
	_, err = client.Do("PUT", fmt.Sprintf("/repos/%s/pulls/%d/merge", repo, number),
		map[string]string{"merge_method": mergeMethod(), "sha": headSHA})
	if err != nil {
		log.Printf("merge PR #%d: %v", number, err)
		return
	}
	log.Printf("merged PR #%d at %.10s", number, headSHA)
	ghbot.Notify("auto-merger", fmt.Sprintf("merged PR #%d in %s", number, repo))
}

func main() {
	cfg := ghbot.LoadConfig("auto-merger", "8110")
	if ghbot.HandleCLIArgs("auto-merger", cfg) {
		return
	}
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
