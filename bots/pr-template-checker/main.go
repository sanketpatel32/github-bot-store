// pr-template-checker — comments when a PR body is empty or missing
// sections your template expects. Configure sections in REQUIRED_SECTIONS.
package main

import (
	"encoding/json"
	"log"
	"os"
	"strings"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

type PREvent struct {
	Action string `json:"action"`
	Number int    `json:"number"`
	PR     struct {
		Body  string `json:"body"`
		Title string `json:"title"`
		User  struct{ Login string } `json:"user"`
	} `json:"pull_request"`
	Repo struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func main() {
	cfg := ghbot.LoadConfig("pr-template-checker", "8112")
	client := ghbot.NewClient(cfg.Token)

	// comma-separated section keywords, case-insensitive
	sectionsEnv := os.Getenv("REQUIRED_SECTIONS")
	if sectionsEnv == "" {
		sectionsEnv = "summary,testing"
	}
	var required []string
	for _, s := range strings.Split(sectionsEnv, ",") {
		if s = strings.TrimSpace(strings.ToLower(s)); s != "" {
			required = append(required, s)
		}
	}

	ghbot.Serve(cfg, "pr-template-checker", func(event string, payload []byte) {
		if event != "pull_request" {
			return
		}
		var ev PREvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		if ev.Action != "opened" && ev.Action != "edited" {
			return
		}

		body := strings.ToLower(ev.PR.Body)
		if strings.TrimSpace(body) == "" {
			client.Comment(ev.Repo.FullName, ev.Number,
				"Your PR description is empty — future-you (and your reviewers) will thank you "+
					"for filling in the template: what changed, why, and how you tested it.")
			log.Printf("PR #%d empty body", ev.Number)
			return
		}

		var missing []string
		for _, section := range required {
			if !strings.Contains(body, section) {
				missing = append(missing, section)
			}
		}
		if len(missing) == 0 {
			return
		}

		client.Comment(ev.Repo.FullName, ev.Number,
			"Your PR description looks like it's missing these sections: **"+
				strings.Join(missing, "**, **")+
				"**. Even a one-liner per section helps reviewers move faster.")
		log.Printf("PR #%d missing sections: %v", ev.Number, missing)
	})
}
