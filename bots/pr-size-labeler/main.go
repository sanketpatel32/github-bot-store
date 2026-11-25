// pr-size-labeler — labels pull requests XS/S/M/L/XL by total lines changed,
// and nudges authors when a PR is too big to review well.
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
	Repo   struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	PR struct {
		Additions int    `json:"additions"`
		Deletions int    `json:"deletions"`
		Title     string `json:"title"`
	} `json:"pull_request"`
}

func sizeLabel(changed int) string {
	switch {
	case changed < 20:
		return "size/XS"
	case changed < 100:
		return "size/S"
	case changed < 300:
		return "size/M"
	case changed < 800:
		return "size/L"
	default:
		return "size/XL"
	}
}

var labelColors = map[string]string{
	"size/XS": "3fb950", "size/S": "56d364", "size/M": "d29922",
	"size/L": "db6d28", "size/XL": "f85149",
}

func main() {
	cfg := ghbot.LoadConfig("pr-size-labeler", "8101")
	client := ghbot.NewClient(cfg.Token)

	ghbot.Serve(cfg, "pr-size-labeler", func(event string, payload []byte) {
		if event != "pull_request" {
			return
		}
		var ev PREvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			log.Printf("bad payload: %v", err)
			return
		}
		switch ev.Action {
		case "opened", "synchronize", "reopened":
		default:
			return
		}
		changed := ev.PR.Additions + ev.PR.Deletions
		label := sizeLabel(changed)

		for name, color := range labelColors {
			client.EnsureLabel(ev.Repo.FullName, name, color, "PR size")
		}

		// remove obsolete size/* labels only — unrelated labels are untouched,
		// and if the tier is unchanged we skip all writes
		body, err := client.Do("GET", fmt.Sprintf("/repos/%s/issues/%d/labels?per_page=100",
			ev.Repo.FullName, ev.Number), nil)
		if err != nil {
			log.Printf("labels fetch: %v", err)
			return
		}
		var current []struct {
			Name string `json:"name"`
		}
		json.Unmarshal(body, &current)
		hasCorrect := false
		var obsolete []string
		for _, l := range current {
			if !strings.HasPrefix(l.Name, "size/") {
				continue
			}
			if l.Name == label {
				hasCorrect = true
			} else {
				obsolete = append(obsolete, l.Name)
			}
		}
		for _, old := range obsolete {
			client.Do("DELETE", fmt.Sprintf("/repos/%s/issues/%d/labels/%s",
				ev.Repo.FullName, ev.Number, old), nil)
		}
		if hasCorrect && len(obsolete) == 0 {
			log.Printf("PR #%d: already %s, no writes", ev.Number, label)
			return
		}
		if !hasCorrect {
			if err := client.AddLabels(ev.Repo.FullName, ev.Number, []string{label}); err != nil {
				log.Printf("label: %v", err)
				return
			}
		}
		log.Printf("PR #%d: +%d/-%d -> %s (removed %d obsolete)", ev.Number,
			ev.PR.Additions, ev.PR.Deletions, label, len(obsolete))

		if label == "size/XL" {
			client.Comment(ev.Repo.FullName, ev.Number, fmt.Sprintf(
				"This PR changes **%d lines** — that's a lot to review well. "+
					"Consider splitting it into smaller stacked PRs if you can.", changed))
		}
	})
}
