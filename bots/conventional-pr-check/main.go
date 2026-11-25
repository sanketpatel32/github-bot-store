// conventional-pr-check — posts a commit status enforcing conventional-commit
// PR titles: feat:, fix:, chore:, docs:, refactor:, test:, perf:, ci:, build:, style:
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

var conventional = regexp.MustCompile(`^(feat|fix|chore|docs|refactor|test|perf|ci|build|style|revert)(\(.+\))?!?: .+`)

type PREvent struct {
	Action string `json:"action"`
	Number int    `json:"number"`
	PR     struct {
		Title string `json:"title"`
		Head  struct {
			SHA string `json:"sha"`
		} `json:"head"`
	} `json:"pull_request"`
	Repo struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func main() {
	cfg := ghbot.LoadConfig("conventional-pr-check", "8105")
	client := ghbot.NewClient(cfg.Token)

	ghbot.Serve(cfg, "conventional-pr-check", func(event string, payload []byte) {
		if event != "pull_request" {
			return
		}
		var ev PREvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		switch ev.Action {
		case "opened", "edited", "synchronize", "reopened":
		default:
			return
		}

		ok := conventional.MatchString(ev.PR.Title)
		state, desc := "success", "PR title follows conventional commits"
		if !ok {
			state = "failure"
			desc = "Title must start with feat:, fix:, chore:, docs:, refactor:, test:, perf:..."
		}

		path := fmt.Sprintf("/repos/%s/statuses/%s", ev.Repo.FullName, ev.PR.Head.SHA)
		_, err := client.Do("POST", path, map[string]string{
			"state":       state,
			"context":     "conventional-pr-title",
			"description": desc,
		})
		if err != nil {
			log.Printf("status: %v", err)
			return
		}
		log.Printf("PR #%d %q -> %s", ev.Number, ev.PR.Title, state)

		if !ok && ev.Action == "opened" {
			client.Comment(ev.Repo.FullName, ev.Number,
				"Your PR title doesn't follow [conventional commits](https://www.conventionalcommits.org/). "+
					"Please rename it to something like `feat: add login page` or `fix(api): handle nil token`.")
		}
	})
}
