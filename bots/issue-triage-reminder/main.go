// issue-triage-reminder — cron bot that posts a digest comment or applies a
// needs-triage label to issues that have no labels/assignee/milestone after
// a grace period, so nothing slips through untriaged.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

func sweep(client *ghbot.Client, repo string, graceHours int) {
	client.EnsureLabel(repo, "needs-triage", "d29922", "Issue awaiting maintainer triage")
	items, err := client.GetAll(fmt.Sprintf("/repos/%s/issues?state=open", repo))
	if err != nil {
		log.Printf("list issues: %v", err)
		return
	}
	now := time.Now()
	for _, raw := range items {
		var issue struct {
			Number    int       `json:"number"`
			CreatedAt time.Time `json:"created_at"`
			Labels    []struct {
				Name string `json:"name"`
			} `json:"labels"`
			Assignees []struct{} `json:"assignees"`
			Milestone any        `json:"milestone"`
			PR        any        `json:"pull_request"`
		}
		if json.Unmarshal(raw, &issue) != nil || issue.PR != nil {
			continue
		}
		if now.Sub(issue.CreatedAt) < time.Duration(graceHours)*time.Hour {
			continue
		}
		hasLabel := false
		alreadyMarked := false
		for _, l := range issue.Labels {
			if l.Name == "needs-triage" {
				alreadyMarked = true
			} else {
				hasLabel = true
			}
		}
		triaged := hasLabel || len(issue.Assignees) > 0 || issue.Milestone != nil
		if triaged {
			// clean up stale marker if someone triaged manually
			if alreadyMarked {
				client.Do("DELETE", fmt.Sprintf("/repos/%s/issues/%d/labels/needs-triage", repo, issue.Number), nil)
				log.Printf("issue #%d: triaged — removed needs-triage", issue.Number)
			}
			continue
		}
		if alreadyMarked {
			continue
		}
		if err := client.AddLabels(repo, issue.Number, []string{"needs-triage"}); err != nil {
			log.Printf("label #%d: %v", issue.Number, err)
			continue
		}
		log.Printf("issue #%d: marked needs-triage (untriaged for %d+ hours)", issue.Number, graceHours)
	}

	// count for digest log line — visibility in Actions output
	log.Printf("triage sweep complete")
}

func main() {
	cfg := ghbot.LoadConfig("issue-triage-reminder", "8118")
	if ghbot.HandleCLIArgs("issue-triage-reminder", cfg, "cron") {
		return
	}
	client := ghbot.NewClient(cfg.Token)
	repo := os.Getenv("REPO")
	if repo == "" {
		log.Fatal("REPO is not set (owner/repo)")
	}
	grace := 24
	if v := os.Getenv("GRACE_HOURS"); v != "" {
		fmt.Sscanf(v, "%d", &grace)
	}
	ghbot.ServeCron(cfg, "issue-triage-reminder", ghbot.SweepInterval(24), func() { sweep(client, repo, grace) })
}
