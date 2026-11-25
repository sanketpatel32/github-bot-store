// pr-path-labeler — labels PRs by which directories they touch, using the
// repo's own labels: touching web/ gets area/web if that label exists, or a
// configured mapping in path-labels.json. Only adds, never removes human
// labels.
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
	Repo   struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

// loadMapping reads path-prefix -> label pairs from path-labels.json.
// Format: {"web/": "area/frontend", "api/": "area/backend"}
func loadMapping() map[string]string {
	data, err := os.ReadFile("path-labels.json")
	if err != nil {
		return nil
	}
	var m map[string]string
	if json.Unmarshal(data, &m) != nil {
		return nil
	}
	return m
}

func main() {
	cfg := ghbot.LoadConfig("pr-path-labeler", "8121")
	if ghbot.HandleCLIArgs("pr-path-labeler", cfg) {
		return
	}
	client := ghbot.NewClient(cfg.Token)

	ghbot.Serve(cfg, "pr-path-labeler", func(event string, payload []byte) {
		if event != "pull_request" {
			return
		}
		var ev PREvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		switch ev.Action {
		case "opened", "synchronize", "reopened":
		default:
			return
		}

		files, err := client.GetAll(fmt.Sprintf("/repos/%s/pulls/%d/files", ev.Repo.FullName, ev.Number))
		if err != nil {
			log.Printf("files: %v", err)
			return
		}
		mapping := loadMapping()
		topDirs := map[string]bool{}
		for _, raw := range files {
			var f struct {
				Filename string `json:"filename"`
			}
			if json.Unmarshal(raw, &f) != nil {
				continue
			}
			// explicit mapping wins — a file covered by a deliberate mapping
			// must not ALSO collect the implicit area label for its directory,
			// or maintainers cannot keep automation/ownership signals clean (#136)
			mapped := false
			for prefix, label := range mapping {
				if strings.HasPrefix(f.Filename, prefix) {
					topDirs[label] = true
					mapped = true
				}
			}
			if mapped {
				continue
			}
			// fall back to top-level directory convention: area/<dir>
			if idx := strings.Index(f.Filename, "/"); idx > 0 {
				topDirs["area/"+f.Filename[:idx]] = true
			}
		}

		// only apply labels that already exist in the repo — never invent
		var wanted []string
		for label := range topDirs {
			_, err := client.Do("GET",
				fmt.Sprintf("/repos/%s/labels/%s", ev.Repo.FullName, label), nil)
			if err == nil {
				wanted = append(wanted, label)
			}
		}
		if len(wanted) == 0 {
			log.Printf("PR #%d: no matching existing labels", ev.Number)
			return
		}
		if err := client.AddLabels(ev.Repo.FullName, ev.Number, wanted); err != nil {
			log.Printf("labels: %v", err)
			return
		}
		log.Printf("PR #%d: applied %v", ev.Number, wanted)
	})
}
