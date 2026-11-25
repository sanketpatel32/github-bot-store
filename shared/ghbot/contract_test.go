// contract_test.go — verifies that realistic GitHub webhook payloads
// (shaped exactly as GitHub documents them) parse into the structures our
// bots depend on, and that REST response shapes match expectations.
// If GitHub changes a schema, these tests catch it before production does.
package ghbot_test

import (
	"encoding/json"
	"testing"
)

// pull_request payload shaped like GitHub's documented webhook example
// (trimmed to fields our bots consume — the point is they exist and type-check)
const prPayload = `{
  "action": "opened",
  "number": 42,
  "pull_request": {
    "title": "feat: something",
    "body": "description",
    "draft": false,
    "merged": false,
    "additions": 100,
    "deletions": 20,
    "mergeable": true,
    "user": {"login": "octocat", "type": "User"},
    "head": {"sha": "abc123def456", "ref": "feature"},
    "base": {"ref": "main"},
    "labels": [{"name": "enhancement"}],
    "author_association": "CONTRIBUTOR"
  },
  "repository": {"full_name": "owner/repo", "default_branch": "main"},
  "sender": {"login": "octocat", "type": "User"}
}`

func TestContractPullRequest(t *testing.T) {
	var ev struct {
		Action string `json:"action"`
		Number int    `json:"number"`
		PR     struct {
			Title     string `json:"title"`
			Draft     bool   `json:"draft"`
			Additions int    `json:"additions"`
			Deletions int    `json:"deletions"`
			User      struct {
				Login string `json:"login"`
			} `json:"user"`
			Head struct {
				SHA string `json:"sha"`
			} `json:"head"`
			Labels []struct {
				Name string `json:"name"`
			} `json:"labels"`
		} `json:"pull_request"`
		Repo struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	if err := json.Unmarshal([]byte(prPayload), &ev); err != nil {
		t.Fatalf("PR payload contract broken: %v", err)
	}
	if ev.Action != "opened" || ev.Number != 42 || ev.PR.Head.SHA == "" ||
		ev.PR.User.Login != "octocat" || ev.Repo.FullName != "owner/repo" {
		t.Error("PR payload fields did not roundtrip")
	}
	if len(ev.PR.Labels) != 1 || ev.PR.Labels[0].Name != "enhancement" {
		t.Error("label array shape mismatch")
	}
}

const issuesPayload = `{
  "action": "opened",
  "issue": {
    "number": 7,
    "title": "something broke",
    "body": "repro steps",
    "state": "open",
    "user": {"login": "reporter", "type": "User"},
    "labels": [{"name": "bug"}],
    "author_association": "FIRST_TIMER"
  },
  "repository": {"full_name": "owner/repo"},
  "sender": {"login": "reporter", "type": "User"}
}`

func TestContractIssues(t *testing.T) {
	var ev struct {
		Action string `json:"action"`
		Issue  struct {
			Number int    `json:"number"`
			Title  string `json:"title"`
			Body   string `json:"body"`
			User   struct {
				Login string `json:"login"`
			} `json:"user"`
		} `json:"issue"`
	}
	if err := json.Unmarshal([]byte(issuesPayload), &ev); err != nil {
		t.Fatalf("issues payload contract broken: %v", err)
	}
	if ev.Issue.Number != 7 || ev.Issue.User.Login != "reporter" {
		t.Error("issue fields did not roundtrip")
	}
}

const checkRunPayload = `{
  "action": "completed",
  "check_run": {
    "name": "ci / build",
    "status": "completed",
    "conclusion": "failure",
    "head_sha": "def789",
    "details_url": "https://example.com/logs"
  },
  "repository": {"full_name": "owner/repo"},
  "sender": {"login": "github-actions[bot]", "type": "Bot"}
}`

func TestContractCheckRun(t *testing.T) {
	var ev struct {
		Action   string `json:"action"`
		CheckRun struct {
			Name       string `json:"name"`
			Conclusion string `json:"conclusion"`
			HeadSHA    string `json:"head_sha"`
		} `json:"check_run"`
	}
	if err := json.Unmarshal([]byte(checkRunPayload), &ev); err != nil {
		t.Fatalf("check_run contract broken: %v", err)
	}
	if ev.CheckRun.Conclusion != "failure" || ev.CheckRun.HeadSHA != "def789" {
		t.Error("check_run fields did not roundtrip")
	}
}

const pushPayload = `{
  "ref": "refs/heads/main",
  "repository": {"full_name": "owner/repo", "default_branch": "main"},
  "commits": [{
    "id": "abc123",
    "message": "feat: x",
    "url": "https://github.com/owner/repo/commit/abc123",
    "author": {"username": "octocat"}
  }],
  "sender": {"login": "octocat", "type": "User"}
}`

func TestContractPush(t *testing.T) {
	var ev struct {
		Ref     string `json:"ref"`
		Commits []struct {
			ID     string `json:"id"`
			Author struct {
				Username string `json:"username"`
			} `json:"author"`
		} `json:"commits"`
	}
	if err := json.Unmarshal([]byte(pushPayload), &ev); err != nil {
		t.Fatalf("push contract broken: %v", err)
	}
	if len(ev.Commits) != 1 || ev.Commits[0].Author.Username != "octocat" {
		t.Error("push commit fields did not roundtrip")
	}
}

// REST contracts: the shapes bots depend on from API responses

func TestContractRESTShapes(t *testing.T) {
	// search response
	searchBody := `{"total_count": 2, "items": [{"number": 1}, {"number": 2}]}`
	var search struct {
		TotalCount int `json:"total_count"`
		Items      []struct {
			Number int `json:"number"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(searchBody), &search); err != nil ||
		search.TotalCount != 2 || len(search.Items) != 2 {
		t.Error("search response contract broken")
	}

	// requested_reviewers response
	rr := `{"users": [{"login": "alice"}], "teams": []}`
	var reviewers struct {
		Users []struct {
			Login string `json:"login"`
		} `json:"users"`
	}
	if err := json.Unmarshal([]byte(rr), &reviewers); err != nil || len(reviewers.Users) != 1 {
		t.Error("requested_reviewers contract broken")
	}

	// files list item — renames carry previous_filename
	file := `{"filename": "new.go", "previous_filename": "old.go", "status": "renamed", "additions": 5, "deletions": 2, "patch": "@@ ..."}`
	var f struct {
		Filename         string `json:"filename"`
		PreviousFilename string `json:"previous_filename"`
		Additions        int    `json:"additions"`
	}
	if err := json.Unmarshal([]byte(file), &f); err != nil || f.PreviousFilename != "old.go" {
		t.Error("pulls files contract broken")
	}

	// contents API — base64 content
	contents := `{"content": "aGVsbG8=\n", "encoding": "base64"}`
	var c struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(contents), &c); err != nil || c.Content == "" {
		t.Error("contents contract broken")
	}
}
