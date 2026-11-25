// conventional-pr-check — posts a commit status enforcing conventional-commit
// PR titles: feat:, fix:, chore:, docs:, refactor:, test:, perf:, ci:, build:, style:
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

// titleRe builds the validation regex from configuration (#26):
// ALLOWED_TYPES overrides the default type list, REQUIRE_SCOPE=true
// makes the (scope) mandatory, and BREAKING=true/false controls whether
// the ! marker is permitted.
func titleRe() *regexp.Regexp {
	types := os.Getenv("ALLOWED_TYPES")
	if types == "" {
		types = "feat,fix,chore,docs,refactor,test,perf,ci,build,style,revert"
	}
	typeAlt := strings.Join(strings.Split(types, ","), "|")
	scope := `(\(.+\))?`
	if os.Getenv("REQUIRE_SCOPE") == "true" {
		scope = `(\(.+\))`
	}
	bang := "!?"
	if os.Getenv("ALLOW_BREAKING") == "false" {
		bang = ""
	}
	return regexp.MustCompile(`^(` + typeAlt + `)` + scope + bang + `: .+`)
}

var defaultTypes = "feat,fix,chore,docs,refactor,test,perf,ci,build,style,revert"

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
	if ghbot.HandleCLIArgs("conventional-pr-check", cfg) {
		return
	}
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

		ok := titleRe().MatchString(ev.PR.Title)
		state, desc := "success", "PR title follows conventional commits"
		if !ok {
			state = "failure"
			types := os.Getenv("ALLOWED_TYPES")
			if types == "" {
				types = defaultTypes
			}
			first := strings.Split(types, ",")[0]
			desc = fmt.Sprintf("Title must look like '%s: description' (types: %s)", first, types)
			if len(desc) > 140 {
				desc = desc[:137] + "..."
			}
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
