// branch-sweeper — deletes the head branch when a PR is merged.
// Skips forks, protected branches, and the default branch.
package main

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

type PREvent struct {
	Action string `json:"action"`
	PR     struct {
		Merged bool `json:"merged"`
		Head   struct {
			Ref  string `json:"ref"`
			Repo struct {
				FullName string `json:"full_name"`
				Fork     bool   `json:"fork"`
			} `json:"repo"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
	} `json:"pull_request"`
	Repo struct {
		FullName      string `json:"full_name"`
		DefaultBranch string `json:"default_branch"`
	} `json:"repository"`
}

var protected = map[string]bool{
	"main": true, "master": true, "develop": true, "dev": true,
	"release": true, "staging": true, "production": true,
}

func main() {
	cfg := ghbot.LoadConfig("branch-sweeper", "8104")
	client := ghbot.NewClient(cfg.Token)

	ghbot.Serve(cfg, "branch-sweeper", func(event string, payload []byte) {
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

		head := ev.PR.Head
		// only sweep branches inside this repo, not forks
		if head.Repo.Fork || head.Repo.FullName != ev.Repo.FullName {
			log.Printf("skip fork branch %s", head.Ref)
			return
		}
		if protected[strings.ToLower(head.Ref)] || head.Ref == ev.Repo.DefaultBranch {
			log.Printf("skip protected branch %s", head.Ref)
			return
		}

		_, err := client.Do("DELETE", "/repos/"+ev.Repo.FullName+"/git/refs/heads/"+head.Ref, nil)
		if err != nil {
			log.Printf("delete %s: %v", head.Ref, err)
			return
		}
		log.Printf("deleted merged branch %s", head.Ref)
	})
}
