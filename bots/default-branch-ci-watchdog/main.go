// default-branch-ci-watchdog tracks failures in a GitHub Actions workflow on
// the default branch. It keeps one open issue per failing workflow.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

const name = "default-branch-ci-watchdog"
const label = "bot:default-branch-ci"

var repoName = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
var workflowName = regexp.MustCompile(`^(?:[A-Za-z0-9_.-]+\.ya?ml|[0-9]+)$`)

type workflowRun struct {
	ID         int64  `json:"id"`
	Attempt    int    `json:"run_attempt"`
	Conclusion string `json:"conclusion"`
	URL        string `json:"html_url"`
}

type alertIssue struct {
	Number      int             `json:"number"`
	Title       string          `json:"title"`
	Body        string          `json:"body"`
	PullRequest json.RawMessage `json:"pull_request"`
}

func sweep(client *ghbot.Client, repo, workflow string) error {
	base := "/repos/" + repo
	body, err := client.Do("GET", base, nil)
	if err != nil {
		return fmt.Errorf("read repository: %w", err)
	}
	var info struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.Unmarshal(body, &info); err != nil || info.DefaultBranch == "" {
		return fmt.Errorf("missing default branch for %s", repo)
	}

	query := url.Values{
		"branch": {info.DefaultBranch}, "status": {"completed"}, "event": {"push"}, "per_page": {"1"},
	}
	body, err = client.Do("GET", base+"/actions/workflows/"+workflow+"/runs?"+query.Encode(), nil)
	if err != nil {
		return fmt.Errorf("read workflow %s: %w", workflow, err)
	}
	var runs struct {
		Runs []workflowRun `json:"workflow_runs"`
	}
	if err := json.Unmarshal(body, &runs); err != nil {
		return fmt.Errorf("decode workflow runs: %w", err)
	}
	if len(runs.Runs) == 0 || runs.Runs[0].ID == 0 {
		return nil
	}
	run := runs.Runs[0]
	switch run.Conclusion {
	case "success", "failure", "timed_out", "action_required", "startup_failure":
	default:
		return nil
	}

	marker := fmt.Sprintf("<!-- %s:%s -->", name, workflow)
	rawIssues, err := client.GetAll(base + "/issues?state=open&labels=" + url.QueryEscape(label))
	if err != nil {
		return fmt.Errorf("find existing alert: %w", err)
	}
	var existing alertIssue
	for _, raw := range rawIssues {
		var issue alertIssue
		if err := json.Unmarshal(raw, &issue); err != nil {
			return fmt.Errorf("decode alert issue: %w", err)
		}
		if issue.Number > 0 && len(issue.PullRequest) == 0 && strings.HasPrefix(issue.Body, marker+"\n") {
			existing = issue
			break
		}
	}
	if run.Conclusion == "success" {
		if existing.Number == 0 {
			return nil
		}
		_, err := client.Do("PATCH", fmt.Sprintf("%s/issues/%d", base, existing.Number),
			map[string]string{"state": "closed", "state_reason": "completed"})
		return err
	}

	title := "Default branch check failed: " + workflow
	alert := fmt.Sprintf("%s\nThe latest push run of **%s** on `%s` ended with **%s**.\n\n[Open run #%d, attempt %d](%s)\n\nThis issue closes when a push run passes.",
		marker, workflow, info.DefaultBranch, run.Conclusion, run.ID, run.Attempt, run.URL)
	if existing.Number > 0 {
		if existing.Title == title && existing.Body == alert {
			return nil
		}
		_, err := client.Do("PATCH", fmt.Sprintf("%s/issues/%d", base, existing.Number),
			map[string]string{"title": title, "body": alert})
		return err
	}

	labelPath := base + "/labels/" + url.PathEscape(label)
	if _, err := client.Do("GET", labelPath, nil); err != nil {
		if !ghbot.IsNotFound(err) {
			return fmt.Errorf("read alert label: %w", err)
		}
		if _, err := client.Do("POST", base+"/labels", map[string]string{
			"name": label, "color": "b60205", "description": "Managed default branch workflow alerts",
		}); err != nil {
			return fmt.Errorf("create alert label: %w", err)
		}
	}
	_, err = client.Do("POST", base+"/issues",
		map[string]any{"title": title, "body": alert, "labels": []string{label}})
	return err
}

func main() {
	cfg := ghbot.LoadConfig("default-branch-ci-watchdog", "8133")
	if ghbot.HandleCLIArgs(name, cfg, "cron") {
		return
	}
	repo := os.Getenv("REPO")
	workflow := os.Getenv("CI_WORKFLOW")
	if !repoName.MatchString(repo) || !workflowName.MatchString(workflow) {
		log.Fatal("set REPO to owner/repository and CI_WORKFLOW to a workflow filename, e.g. ci.yml, or its numeric ID")
	}
	client := ghbot.NewClient(cfg.Token)
	ghbot.ServeCron(cfg, name, ghbot.SweepInterval(1), func() {
		if err := sweep(client, repo, workflow); err != nil {
			if os.Getenv("GITHUB_ACTIONS") == "true" {
				log.Fatal(err)
			}
			log.Print(err)
		}
	})
}
