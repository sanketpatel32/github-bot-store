// first-touch-welcomer — posts a friendly comment the first time someone
// opens an issue or PR in your repo. Uses the author's association plus a
// search for prior contributions to detect first-timers.
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

type event struct {
	Action string `json:"action"`
	Issue  *struct {
		Number int                    `json:"number"`
		User   struct{ Login string } `json:"user"`
		Author string                 `json:"author_association"`
	} `json:"issue"`
	PR *struct {
		Number int                    `json:"number"`
		User   struct{ Login string } `json:"user"`
		Author string                 `json:"author_association"`
	} `json:"pull_request"`
	Repo struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func isFirstTimer(client *ghbot.Client, repo, user string) (bool, error) {
	// count prior merged/closed contributions by this user in this repo
	path := fmt.Sprintf("/search/issues?q=repo:%s+author:%s&per_page=1", repo, user)
	body, err := client.Do("GET", path, nil)
	if err != nil {
		return false, err
	}
	var result struct {
		TotalCount int `json:"total_count"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return false, err
	}
	// the one we just saw counts as 1 — first-timer means exactly 1 total
	return result.TotalCount <= 1, nil
}

func main() {
	cfg := ghbot.LoadConfig("first-touch-welcomer", "8102")
	if ghbot.HandleCLIArgs("first-touch-welcomer", cfg) {
		return
	}
	client := ghbot.NewClient(cfg.Token)

	ghbot.Serve(cfg, "first-touch-welcomer", func(eventType string, payload []byte) {
		var ev event
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		if ev.Action != "opened" {
			return
		}

		var number int
		var login, association, kind string
		switch {
		case eventType == "issues" && ev.Issue != nil:
			number, login, association, kind = ev.Issue.Number, ev.Issue.User.Login, ev.Issue.Author, "issue"
		case eventType == "pull_request" && ev.PR != nil:
			number, login, association, kind = ev.PR.Number, ev.PR.User.Login, ev.PR.Author, "pull request"
		default:
			return
		}

		if association == "OWNER" || association == "MEMBER" || association == "COLLABORATOR" {
			return
		}
		first, err := isFirstTimer(client, ev.Repo.FullName, login)
		if err != nil {
			log.Printf("search: %v", err)
			return
		}
		if !first {
			return
		}

		body := fmt.Sprintf(
			"Welcome, @%s! 🎉 Thanks for opening your first %s here. "+
				"A maintainer will take a look soon — meanwhile make sure the details above are complete.",
			login, kind)
		if err := client.Comment(ev.Repo.FullName, number, body); err != nil {
			log.Printf("comment: %v", err)
			return
		}
		log.Printf("welcomed %s on #%d", login, number)
	})
}
