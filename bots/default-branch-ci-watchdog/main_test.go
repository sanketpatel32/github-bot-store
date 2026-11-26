package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

func TestSweepTracksOneIssueUntilCIRecovers(t *testing.T) {
	var lock sync.Mutex
	run := workflowRun{ID: 42, Attempt: 1, Conclusion: "failure", URL: "https://github.com/example/project/actions/runs/42"}
	var issue alertIssue
	var labelExists bool
	var mutations []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lock.Lock()
		defer lock.Unlock()
		reply := func(value any) { _ = json.NewEncoder(w).Encode(value) }
		if r.Header.Get("Authorization") != "Bearer dummy" {
			t.Errorf("request to %s did not use the configured token", r.URL.Path)
		}
		if r.Method != "GET" {
			mutations = append(mutations, r.Method+" "+r.URL.Path)
		}
		switch r.Method + " " + r.URL.Path {
		case "GET /repos/example/project":
			reply(map[string]string{"default_branch": "production"})
		case "GET /repos/example/project/actions/workflows/build.yml/runs":
			q := r.URL.Query()
			if q.Get("branch") != "production" || q.Get("event") != "push" ||
				q.Get("status") != "completed" || q.Get("per_page") != "1" {
				t.Errorf("unexpected workflow query: %v", q)
			}
			reply(map[string][]workflowRun{"workflow_runs": {run}})
		case "GET /repos/example/project/issues":
			if q := r.URL.Query(); q.Get("state") != "open" || q.Get("labels") != label {
				t.Errorf("unexpected issues query: %v", q)
			}
			other := alertIssue{Number: 6, Body: "<!-- default-branch-ci-watchdog:other.yml -->\n"}
			issues := []alertIssue{other}
			if issue.Number != 0 {
				issues = append(issues, issue)
			}
			// The API omits pull_request for issue entries.
			items := make([]map[string]any, 0, len(issues))
			for _, item := range issues {
				items = append(items, map[string]any{"number": item.Number, "title": item.Title, "body": item.Body})
			}
			reply(items)
		case "GET /repos/example/project/labels/" + label:
			if !labelExists {
				w.WriteHeader(http.StatusNotFound)
			}
		case "POST /repos/example/project/labels":
			var data map[string]string
			if err := json.NewDecoder(r.Body).Decode(&data); err != nil || data["name"] != label {
				t.Errorf("wrong label payload: %v, %v", data, err)
			}
			labelExists = true
		case "POST /repos/example/project/issues":
			var data struct {
				Title  string   `json:"title"`
				Body   string   `json:"body"`
				Labels []string `json:"labels"`
			}
			if err := json.NewDecoder(r.Body).Decode(&data); err != nil ||
				len(data.Labels) != 1 || data.Labels[0] != label {
				t.Errorf("wrong issue payload: %+v, %v", data, err)
			}
			issue = alertIssue{Number: 7, Title: data.Title, Body: data.Body}
		case "PATCH /repos/example/project/issues/7":
			var data map[string]string
			if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
				t.Errorf("invalid patch: %v", err)
			}
			if data["state"] == "closed" && data["state_reason"] == "completed" {
				issue = alertIssue{}
				mutations[len(mutations)-1] = "close #7"
			} else {
				issue.Title = data["title"]
				issue.Body = data["body"]
			}
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("GITHUB_API_URL", server.URL)
	client := ghbot.NewClient("dummy")
	check := func() {
		t.Helper()
		if err := sweep(client, "example/project", "build.yml"); err != nil {
			t.Fatal(err)
		}
	}

	check()
	check() // The same failing run must not create or edit anything twice.
	lock.Lock()
	if !strings.Contains(issue.Body, run.URL) {
		t.Error("alert does not link to the failed run")
	}
	run.Attempt++
	lock.Unlock()
	check() // A failed rerun updates the open issue.
	lock.Lock()
	run.Conclusion = "cancelled"
	lock.Unlock()
	check()
	lock.Lock()
	run.Conclusion = "success"
	lock.Unlock()
	check()
	check() // An already-green workflow must not create or close another issue.
	lock.Lock()
	defer lock.Unlock()
	want := []string{"POST /repos/example/project/labels", "POST /repos/example/project/issues", "PATCH /repos/example/project/issues/7", "close #7"}
	if fmt.Sprint(mutations) != fmt.Sprint(want) {
		t.Fatalf("mutations = %v, want %v", mutations, want)
	}
}
