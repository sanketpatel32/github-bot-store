// release-drafter — maintains a running draft GitHub release. Every merged PR
// appends an entry to the draft body, grouped by conventional-commit type.
// Publish the draft whenever you actually cut the release.
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
	PR     struct {
		Merged bool                   `json:"merged"`
		Title  string                 `json:"title"`
		Number int                    `json:"number"`
		User   struct{ Login string } `json:"user"`
	} `json:"pull_request"`
	Repo struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

var sections = []struct{ key, heading string }{
	{"feat", "Features"},
	{"fix", "Bug Fixes"},
	{"perf", "Performance"},
	{"refactor", "Refactoring"},
	{"docs", "Documentation"},
	{"test", "Testing"},
	{"ci", "CI"},
	{"chore", "Chores"},
}

func groupOf(title string) string {
	lower := strings.ToLower(title)
	for _, s := range sections {
		if strings.HasPrefix(lower, s.key+":") || strings.HasPrefix(lower, s.key+"(") {
			return s.heading
		}
	}
	return "Other"
}

// findDraft returns the id+body of the newest draft release, or 0.
func findDraft(client *ghbot.Client, repo string) (int64, string) {
	body, err := client.Do("GET", fmt.Sprintf("/repos/%s/releases?per_page=20", repo), nil)
	if err != nil {
		return 0, ""
	}
	var releases []struct {
		ID    int64  `json:"id"`
		Draft bool   `json:"draft"`
		Body  string `json:"body"`
	}
	if json.Unmarshal(body, &releases) != nil {
		return 0, ""
	}
	for _, r := range releases {
		if r.Draft {
			return r.ID, r.Body
		}
	}
	return 0, ""
}

func insertEntry(body, heading, entry string) string {
	if strings.Contains(body, entry) {
		return body // idempotent — replays don't duplicate
	}
	header := "### " + heading
	if idx := strings.Index(body, header); idx != -1 {
		// insert right after the heading line
		lineEnd := strings.Index(body[idx:], "\n")
		if lineEnd == -1 {
			return body + "\n" + entry
		}
		pos := idx + lineEnd + 1
		return body[:pos] + entry + "\n" + body[pos:]
	}
	// append a new section at the end
	if !strings.HasSuffix(body, "\n") && body != "" {
		body += "\n"
	}
	return body + "\n" + header + "\n" + entry + "\n"
}

func main() {
	cfg := ghbot.LoadConfig("release-drafter", "8113")
	if ghbot.HandleCLIArgs("release-drafter", cfg) {
		return
	}
	client := ghbot.NewClient(cfg.Token)

	ghbot.Serve(cfg, "release-drafter", func(event string, payload []byte) {
		if event != "pull_request" {
			return
		}
		var ev PREvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		if ev.Action != "closed" || !ev.PR.Merged {
			return
		}

		repo := ev.Repo.FullName
		entry := fmt.Sprintf("- %s (#%d) @%s", ev.PR.Title, ev.PR.Number, ev.PR.User.Login)
		heading := groupOf(ev.PR.Title)

		id, body := findDraft(client, repo)
		if id == 0 {
			// create the draft
			newBody := "\n### " + heading + "\n" + entry + "\n"
			_, err := client.Do("POST", fmt.Sprintf("/repos/%s/releases", repo), map[string]any{
				"tag_name": "", "name": "Next release (draft)", "body": newBody, "draft": true,
			})
			if err != nil {
				log.Printf("create draft: %v", err)
				return
			}
			log.Printf("created draft release with #%d", ev.PR.Number)
			return
		}

		updated := insertEntry(body, heading, entry)
		_, err := client.Do("PATCH", fmt.Sprintf("/repos/%s/releases/%d", repo, id),
			map[string]any{"body": updated})
		if err != nil {
			log.Printf("update draft: %v", err)
			return
		}
		log.Printf("added #%d to draft under %s", ev.PR.Number, heading)
	})
}
