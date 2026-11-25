// changelog-checker — commit status enforcing that PRs either touch
// CHANGELOG.md or carry a skip-changelog label. Keeps release notes honest.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

type PREvent struct {
	Action string `json:"action"`
	Number int    `json:"number"`
	PR     struct {
		Title  string `json:"title"`
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

func changelogFile() string {
	if f := os.Getenv("CHANGELOG_FILE"); f != "" {
		return f
	}
	return "CHANGELOG.md"
}

func skipLabel() string {
	if l := os.Getenv("SKIP_LABEL"); l != "" {
		return l
	}
	return "skip-changelog"
}

func main() {
	cfg := ghbot.LoadConfig("changelog-checker", "8119")
	if ghbot.HandleCLIArgs("changelog-checker", cfg, "webhook") {
		return
	}
	client := ghbot.NewClient(cfg.Token)

	ghbot.Serve(cfg, "changelog-checker", func(event string, payload []byte) {
		if event != "pull_request" {
			return
		}
		var ev PREvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		switch ev.Action {
		case "opened", "synchronize", "reopened", "labeled", "unlabeled":
		default:
			return
		}

		skip := skipLabel()
		for _, l := range ev.PR.Labels {
			if l.Name == skip {
				postStatus(client, ev, "success", fmt.Sprintf("changelog skipped via %s label", skip))
				return
			}
		}

		files, err := client.GetAll(fmt.Sprintf("/repos/%s/pulls/%d/files", ev.Repo.FullName, ev.Number))
		if err != nil {
			log.Printf("files: %v", err)
			return
		}
		target := changelogFile()
		found := false
		for _, raw := range files {
			var f struct {
				Filename string `json:"filename"`
			}
			if json.Unmarshal(raw, &f) != nil {
				continue
			}
			if strings.EqualFold(f.Filename, target) {
				found = true
				break
			}
		}
		if found {
			postStatus(client, ev, "success", "changelog updated")
		} else {
			postStatus(client, ev, "failure",
				fmt.Sprintf("update %s or add the %s label", target, skip))
		}
	})
}

func postStatus(client *ghbot.Client, ev PREvent, state, desc string) {
	if len(desc) > 140 {
		desc = desc[:137] + "..."
	}
	_, err := client.Do("POST",
		fmt.Sprintf("/repos/%s/statuses/%s", ev.Repo.FullName, ev.PR.Head.SHA),
		map[string]string{"state": state, "context": "changelog", "description": desc})
	if err != nil {
		log.Printf("status: %v", err)
		return
	}
	log.Printf("PR #%d -> %s", ev.Number, state)
}
