// issue-form-validator — checks new issues against your issue template:
// required sections present and filled in. Posts one managed comment and
// applies/removes an invalid-issue label accordingly.
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

const marker = "<!-- issue-form-validator -->"

type issueEvent struct {
	Action string `json:"action"`
	Issue  struct {
		Number int    `json:"number"`
		Body   string `json:"body"`
		PR     any    `json:"pull_request"`
	} `json:"issue"`
	Repo struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

// requiredSections parses comma-separated REQUIRED_SECTIONS env or defaults.
func requiredSections() []string {
	raw := os.Getenv("REQUIRED_SECTIONS")
	if raw == "" {
		raw = "what happened,steps to reproduce,expected"
	}
	var out []string
	for _, s := range strings.Split(raw, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, strings.ToLower(s))
		}
	}
	return out
}

var placeholderRe = regexp.MustCompile(`(?i)^(your .+ here|describe|n/?a|tbd|todo|none|\.\.\.)?$`)

// validateIssue checks each required section exists as a heading and has
// substantive content.
func validateIssue(body string, required []string) []string {
	sections := map[string]string{}
	var current string
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "#") {
			current = strings.ToLower(strings.TrimSpace(strings.TrimLeft(line, "#")))
			sections[current] = ""
			continue
		}
		if current != "" {
			// strip html comments (template instructions)
			if strings.HasPrefix(strings.TrimSpace(line), "<!--") {
				continue
			}
			sections[current] += line + "\n"
		}
	}
	var problems []string
	for _, want := range required {
		found := false
		for heading, content := range sections {
			if strings.Contains(heading, want) {
				found = true
				text := strings.TrimSpace(content)
				if len(text) < 5 || placeholderRe.MatchString(text) {
					problems = append(problems, fmt.Sprintf("**%s** is present but empty or placeholder", want))
				}
				break
			}
		}
		if !found {
			problems = append(problems, fmt.Sprintf("**%s** section is missing", want))
		}
	}
	return problems
}

func main() {
	cfg := ghbot.LoadConfig("issue-form-validator", "8127")
	client := ghbot.NewClient(cfg.Token)

	ghbot.Serve(cfg, "issue-form-validator", func(event string, payload []byte) {
		if event != "issues" {
			return
		}
		var ev issueEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		if ev.Action != "opened" && ev.Action != "edited" {
			return
		}
		if ev.Issue.PR != nil {
			return
		}

		problems := validateIssue(ev.Issue.Body, requiredSections())
		client.EnsureLabel(ev.Repo.FullName, "invalid-issue", "f85149", "Issue doesn't follow the template")

		if len(problems) == 0 {
			// valid — remove label if present, resolve comment
			client.Do("DELETE", fmt.Sprintf("/repos/%s/issues/%d/labels/invalid-issue",
				ev.Repo.FullName, ev.Issue.Number), nil)
			log.Printf("issue #%d: valid", ev.Issue.Number)
			return
		}

		client.AddLabels(ev.Repo.FullName, ev.Issue.Number, []string{"invalid-issue"})
		var sb strings.Builder
		sb.WriteString(marker + "\n**This issue doesn't follow the template:**\n\n")
		for _, p := range problems {
			sb.WriteString("- " + p + "\n")
		}
		sb.WriteString("\nPlease edit your issue to fill in the missing sections — it helps maintainers triage much faster.")
		upsert(client, ev.Repo.FullName, ev.Issue.Number, sb.String())
		log.Printf("issue #%d: %d template problems", ev.Issue.Number, len(problems))
	})
}

func upsert(client *ghbot.Client, repo string, number int, body string) {
	comments, err := client.GetAll(fmt.Sprintf("/repos/%s/issues/%d/comments", repo, number))
	if err == nil {
		for _, raw := range comments {
			var c struct {
				ID   int    `json:"id"`
				Body string `json:"body"`
			}
			if json.Unmarshal(raw, &c) != nil {
				continue
			}
			if strings.Contains(c.Body, marker) {
				client.Do("PATCH", fmt.Sprintf("/repos/%s/issues/comments/%d", repo, c.ID),
					map[string]string{"body": body})
				return
			}
		}
	}
	client.Comment(repo, number, body)
}
