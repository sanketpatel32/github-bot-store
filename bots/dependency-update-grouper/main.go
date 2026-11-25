// dependency-update-grouper — labels dependency-update PRs by semver bump
// size (deps/patch, deps/minor, deps/major) parsed from the title, so you
// can batch-merge the safe ones and review the risky ones carefully.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

type PREvent struct {
	Action string `json:"action"`
	Number int    `json:"number"`
	PR     struct {
		Title string `json:"title"`
		User  struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"pull_request"`
	Repo struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

// dependabot titles look like: "Bump lodash from 4.17.20 to 4.17.21"
var bumpRe = regexp.MustCompile(`(?i)(?:bump|update)\s+\S+\s+from\s+v?(\d+)\.(\d+)\.(\d+)\s+to\s+v?(\d+)\.(\d+)\.(\d+)`)

func bumpSize(title string) string {
	m := bumpRe.FindStringSubmatch(title)
	if m == nil {
		return ""
	}
	major1, _ := strconv.Atoi(m[1])
	minor1, _ := strconv.Atoi(m[2])
	patch1, _ := strconv.Atoi(m[3])
	major2, _ := strconv.Atoi(m[4])
	minor2, _ := strconv.Atoi(m[5])
	patch2, _ := strconv.Atoi(m[6])
	switch {
	case major2 > major1:
		return "deps/major"
	case minor2 > minor1:
		return "deps/minor"
	case patch2 > patch1:
		return "deps/patch"
	}
	return ""
}

func main() {
	cfg := ghbot.LoadConfig("dependency-update-grouper", "8128")
	if ghbot.HandleCLIArgs("dependency-update-grouper", cfg) {
		return
	}
	client := ghbot.NewClient(cfg.Token)

	ghbot.Serve(cfg, "dependency-update-grouper", func(event string, payload []byte) {
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
		// only process PRs from dependency bots
		login := strings.ToLower(ev.PR.User.Login)
		if !strings.Contains(login, "dependabot") && !strings.Contains(login, "renovate") {
			return
		}

		size := bumpSize(ev.PR.Title)
		if size == "" {
			log.Printf("PR #%d: can't parse bump from %q", ev.Number, ev.PR.Title)
			return
		}
		colors := map[string]string{"deps/patch": "3fb950", "deps/minor": "d29922", "deps/major": "f85149"}
		for name, color := range colors {
			client.EnsureLabel(ev.Repo.FullName, name, color, "Dependency update size")
		}
		// remove other dep-size labels so exactly one applies
		for name := range colors {
			if name != size {
				client.Do("DELETE", fmt.Sprintf("/repos/%s/issues/%d/labels/%s",
					ev.Repo.FullName, ev.Number, name), nil)
			}
		}
		if err := client.AddLabels(ev.Repo.FullName, ev.Number, []string{size}); err != nil {
			log.Printf("label: %v", err)
			return
		}
		log.Printf("PR #%d: %s", ev.Number, size)
	})
}
