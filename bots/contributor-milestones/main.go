// contributor-milestones — celebrates contributors when their PR merges:
// first merge gets a welcome-aboard comment, and milestone counts (5, 10,
// 25, 50, 100) get shout-outs. Recognition keeps contributors coming back.
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
		Merged bool `json:"merged"`
		User   struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"pull_request"`
	Repo struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

var milestones = map[int]string{
	1:   "🎉 First merged PR — welcome aboard, @%s! Thanks for contributing, and here's to many more.",
	5:   "🖐 That's 5 merged PRs for @%s — officially a regular. Thank you!",
	10:  "🔟 Double digits! @%s just merged their 10th PR. Incredible consistency.",
	25:  "🌟 25 merged PRs — @%s is a cornerstone of this project.",
	50:  "🏆 50 merged PRs! @%s, you absolute legend.",
	100: "💯 One hundred merged PRs. @%s has basically co-authored this repo.",
}

func main() {
	cfg := ghbot.LoadConfig("contributor-milestones", "8122")
	if ghbot.HandleCLIArgs("contributor-milestones", cfg) {
		return
	}
	client := ghbot.NewClient(cfg.Token)

	ghbot.Serve(cfg, "contributor-milestones", func(event string, payload []byte) {
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
		login := ev.PR.User.Login
		if strings.HasSuffix(login, "[bot]") {
			return
		}

		// count this user's merged PRs — the one just closed included
		body, err := client.Do("GET", fmt.Sprintf(
			"/search/issues?q=repo:%s+type:pr+is:merged+author:%s&per_page=1",
			ev.Repo.FullName, login), nil)
		if err != nil {
			log.Printf("search: %v", err)
			return
		}
		var result struct {
			TotalCount int `json:"total_count"`
		}
		if json.Unmarshal(body, &result) != nil {
			return
		}

		tmpl, ok := milestones[result.TotalCount]
		if !ok {
			log.Printf("PR #%d: %s at %d merged — no milestone", ev.Number, login, result.TotalCount)
			return
		}
		if err := client.Comment(ev.Repo.FullName, ev.Number, fmt.Sprintf(tmpl, login)); err != nil {
			log.Printf("comment: %v", err)
			return
		}
		log.Printf("PR #%d: celebrated %s milestone %d", ev.Number, login, result.TotalCount)
	})
}
