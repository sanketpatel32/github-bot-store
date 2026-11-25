// ci-failure-commenter — when a check fails on a commit that belongs to a PR,
// comments which checks failed with links to the logs. Collapses repeat
// failures into one updated comment instead of spamming.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

const marker = "<!-- ci-failure-commenter -->"

type CheckEvent struct {
	Action   string `json:"action"`
	CheckRun *struct {
		HeadSHA      string `json:"head_sha"`
		Conclusion   string `json:"conclusion"`
		Name         string `json:"name"`
		HTMLURL      string `json:"html_url"`
		PullRequests []struct {
			Number int `json:"number"`
		} `json:"pull_requests"`
	} `json:"check_run"`
	CheckSuite *struct {
		HeadSHA      string `json:"head_sha"`
		Conclusion   string `json:"conclusion"`
		PullRequests []struct {
			Number int `json:"number"`
		} `json:"pull_requests"`
	} `json:"check_suite"`
	Repo struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func main() {
	cfg := ghbot.LoadConfig("ci-failure-commenter", "8115")
	if ghbot.HandleCLIArgs("ci-failure-commenter", cfg) {
		return
	}
	client := ghbot.NewClient(cfg.Token)

	ghbot.Serve(cfg, "ci-failure-commenter", func(event string, payload []byte) {
		var ev CheckEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}

		var sha string
		var prNumbers []int
		switch {
		case event == "check_run" && ev.CheckRun != nil:
			if ev.Action != "completed" {
				return
			}
			sha = ev.CheckRun.HeadSHA
			for _, pr := range ev.CheckRun.PullRequests {
				prNumbers = append(prNumbers, pr.Number)
			}
		case event == "check_suite" && ev.CheckSuite != nil:
			if ev.Action != "completed" {
				return
			}
			sha = ev.CheckSuite.HeadSHA
			for _, pr := range ev.CheckSuite.PullRequests {
				prNumbers = append(prNumbers, pr.Number)
			}
		default:
			return
		}
		if len(prNumbers) == 0 {
			return
		}

		// collect failing checks for this sha
		runsBody, err := client.Do("GET",
			fmt.Sprintf("/repos/%s/commits/%s/check-runs?per_page=50", ev.Repo.FullName, sha), nil)
		if err != nil {
			log.Printf("check-runs: %v", err)
			return
		}
		var runs struct {
			CheckRuns []struct {
				Name       string `json:"name"`
				Conclusion string `json:"conclusion"`
				HTMLURL    string `json:"html_url"`
			} `json:"check_runs"`
		}
		if json.Unmarshal(runsBody, &runs) != nil {
			return
		}
		var failed []string
		for _, r := range runs.CheckRuns {
			if r.Conclusion == "failure" || r.Conclusion == "timed_out" || r.Conclusion == "action_required" {
				failed = append(failed, fmt.Sprintf("- ❌ [%s](%s) — %s", r.Name, r.HTMLURL, r.Conclusion))
			}
		}
		for _, number := range prNumbers {
			// never publish failure feedback for an outdated head — delayed
			// events for the previous commit must not overwrite the feedback
			// for the push a contributor is looking at now (#137)
			prBody, err := client.Do("GET", fmt.Sprintf("/repos/%s/pulls/%d", ev.Repo.FullName, number), nil)
			if err != nil {
				continue
			}
			var prDetail struct {
				Head struct {
					SHA string `json:"sha"`
				} `json:"head"`
			}
			if json.Unmarshal(prBody, &prDetail) != nil || prDetail.Head.SHA != sha {
				log.Printf("PR #%d: event sha %.7s is not current head — skipping", number, sha)
				continue
			}

			var body string
			if len(failed) == 0 {
				// checks green again — a comment still screaming failure on a
				// repaired PR is worse than no comment (#137)
				body = marker + "\n✅ CI is green on the latest push."
			} else {
				body = marker + "\n**CI is failing** on the latest push:\n\n" + strings.Join(failed, "\n") +
					"\n\n_Click a check to see the logs._"
			}
			// green resolution only edits an existing warning — posting a
			// fresh "CI is green" comment on a PR that never had a failure
			// would be noise on every successful push
			upsertComment(client, ev.Repo.FullName, number, body, len(failed) > 0)
		}
	})
}

// upsertComment edits the bot's existing comment if present, else posts a new one.
func upsertComment(client *ghbot.Client, repo string, number int, body string, postIfMissing bool) {
	commentsBody, err := client.Do("GET",
		fmt.Sprintf("/repos/%s/issues/%d/comments?per_page=100", repo, number), nil)
	if err == nil {
		var comments []struct {
			ID   int64  `json:"id"`
			Body string `json:"body"`
		}
		if json.Unmarshal(commentsBody, &comments) == nil {
			for _, c := range comments {
				if strings.Contains(c.Body, marker) {
					if _, err := client.Do("PATCH",
						fmt.Sprintf("/repos/%s/issues/comments/%d", repo, c.ID),
						map[string]string{"body": body}); err == nil {
						log.Printf("updated ci comment on #%d", number)
						return
					}
				}
			}
		}
	}
	if !postIfMissing {
		return
	}
	if err := client.Comment(repo, number, body); err != nil {
		log.Printf("comment: %v", err)
		return
	}
	log.Printf("posted ci failure comment on #%d", number)
}
