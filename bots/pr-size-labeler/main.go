// pr-size-labeler — labels pull requests XS/S/M/L/XL by total lines changed,
// and nudges authors when a PR is too big to review well.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
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

// thresholds are configurable via SIZE_TIER_MAX env: comma-separated upper
// bounds for XS,S,M,L (XL is everything above). Default matches the
// long-standing 20/100/300/800 tiers (#27).
func tierBounds() []int {
	raw := os.Getenv("SIZE_TIER_MAX")
	if raw == "" {
		return []int{20, 100, 300, 800}
	}
	var out []int
	for _, part := range strings.Split(raw, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || n <= 0 {
			log.Printf("SIZE_TIER_MAX entry %q invalid — using defaults", part)
			return []int{20, 100, 300, 800}
		}
		out = append(out, n)
	}
	if len(out) != 4 {
		log.Printf("SIZE_TIER_MAX needs exactly 4 comma-separated bounds — using defaults")
		return []int{20, 100, 300, 800}
	}
	// bounds must strictly ascend — a typo like "100,20,300,800" silently
	// makes tiers unreachable and mislabels every large PR (#133)
	for i := 1; i < len(out); i++ {
		if out[i] <= out[i-1] {
			log.Printf("SIZE_TIER_MAX must strictly ascend (got %v) — using defaults", out)
			return []int{20, 100, 300, 800}
		}
	}
	return out
}

func sizeLabel(changed int, bounds []int) string {
	labels := []string{"size/XS", "size/S", "size/M", "size/L"}
	for i, b := range bounds {
		if changed < b {
			return labels[i]
		}
	}
	return "size/XL"
}

// generatedFileRe matches files that shouldn't count toward PR size:
// lockfiles, generated code, minified assets (#27). Override with
// GENERATED_PATTERNS (comma-separated substrings).
var defaultGenerated = []string{
	"package-lock.json", "yarn.lock", "pnpm-lock.yaml", "go.sum", "Cargo.lock",
	"composer.lock", "Gemfile.lock", "poetry.lock", "Pipfile.lock",
	".pb.go", "_generated", ".min.js", ".min.css", ".snap", "package.json.md5",
}

func generatedPatterns() []string {
	if raw := os.Getenv("GENERATED_PATTERNS"); raw != "" {
		return strings.Split(raw, ",")
	}
	return defaultGenerated
}

func isGenerated(path string, patterns []string) bool {
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		// empty patterns match EVERYTHING in strings.Contains — one trailing
		// comma in GENERATED_PATTERNS would classify every file as generated
		// and shrink big PRs to XS (#133)
		if p == "" {
			continue
		}
		if strings.Contains(path, p) {
			return true
		}
	}
	return false
}

var labelColors = map[string]string{
	"size/XS": "3fb950", "size/S": "56d364", "size/M": "d29922",
	"size/L": "db6d28", "size/XL": "f85149",
}

func main() {
	cfg := ghbot.LoadConfig("pr-size-labeler", "8101")
	if ghbot.HandleCLIArgs("pr-size-labeler", cfg) {
		return
	}
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
		// count only non-generated files when exclusion is enabled
		// (EXCLUDE_GENERATED=false to disable) — a lockfile-only update
		// shouldn't label a PR XL (#27)
		changed := ev.PR.Additions + ev.PR.Deletions
		if os.Getenv("EXCLUDE_GENERATED") != "false" {
			patterns := generatedPatterns()
			if files, ferr := client.GetAll(fmt.Sprintf("/repos/%s/pulls/%d/files",
				ev.Repo.FullName, ev.Number)); ferr == nil {
				real := 0
				for _, raw := range files {
					var f struct {
						Filename  string `json:"filename"`
						Additions int    `json:"additions"`
						Deletions int    `json:"deletions"`
					}
					if json.Unmarshal(raw, &f) != nil {
						continue
					}
					if !isGenerated(f.Filename, patterns) {
						real += f.Additions + f.Deletions
					}
				}
				changed = real
			}
		}
		label := sizeLabel(changed, tierBounds())

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
