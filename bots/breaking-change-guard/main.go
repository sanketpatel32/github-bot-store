// breaking-change-guard — detects breaking changes signaled in PR titles
// (feat!: ...) or bodies (BREAKING CHANGE: footer) and requires explicit
// acknowledgment: a breaking-change label plus a commit status that stays
// pending until a maintainer applies the label.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

type PREvent struct {
	Action string `json:"action"`
	Number int    `json:"number"`
	PR     struct {
		Title  string `json:"title"`
		Body   string `json:"body"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
	} `json:"pull_request"`
	Repo struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

// isBreaking reports whether the PR declares a breaking change via
// conventional-commit markers.
func isBreaking(title, body string) bool {
	// feat!: or feat(scope)!: in title
	if idx := strings.Index(title, ":"); idx > 0 {
		prefix := title[:idx]
		if strings.HasSuffix(prefix, "!") {
			return true
		}
	}
	// BREAKING CHANGE / BREAKING-CHANGE footer anywhere in body
	up := strings.ToUpper(body)
	return strings.Contains(up, "BREAKING CHANGE") || strings.Contains(up, "BREAKING-CHANGE")
}

func main() {
	cfg := ghbot.LoadConfig("breaking-change-guard", "8123")
	if ghbot.HandleCLIArgs("breaking-change-guard", cfg) {
		return
	}
	client := ghbot.NewClient(cfg.Token)

	ghbot.Serve(cfg, "breaking-change-guard", func(event string, payload []byte) {
		if event != "pull_request" {
			return
		}
		var ev PREvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		switch ev.Action {
		case "opened", "edited", "synchronize", "reopened", "labeled", "unlabeled":
		default:
			return
		}

		breaking := isBreaking(ev.PR.Title, ev.PR.Body)
		acknowledged := false
		for _, l := range ev.PR.Labels {
			if l.Name == "breaking-change" {
				acknowledged = true
			}
		}

		client.EnsureLabel(ev.Repo.FullName, "breaking-change", "f85149", "Declares a breaking change — needs migration notes")

		var state, desc string
		switch {
		case !breaking:
			state, desc = "success", "no breaking change markers found"
		case breaking && acknowledged:
			state, desc = "success", "breaking change acknowledged with label"
		default:
			state, desc = "failure", "breaking change declared — needs breaking-change label and migration notes"
		}
		_, err := client.Do("POST", fmt.Sprintf("/repos/%s/statuses/%s", ev.Repo.FullName, ev.PR.Head.SHA),
			map[string]string{"state": state, "context": "breaking-change", "description": desc})
		if err != nil {
			log.Printf("status: %v", err)
			return
		}
		log.Printf("PR #%d breaking=%v ack=%v -> %s", ev.Number, breaking, acknowledged, state)
	})
}
