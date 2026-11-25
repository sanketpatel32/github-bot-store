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

// templateProfile describes a known issue form: a heading that identifies
// the template and the sections that must carry real content.
type templateProfile struct {
	name      string
	signature string   // heading substring identifying the form
	required  []string // headings that must exist and be filled
}

// knownTemplates mirrors the forms in .github/ISSUE_TEMPLATE — a completely
// filled form must always pass validation (#112).
var knownTemplates = []templateProfile{
	{"bug report", "what happened", []string{"what happened"}},
	{"feature request", "summary", []string{"summary", "details"}},
}

// requiredSections parses comma-separated REQUIRED_SECTIONS env. When unset,
// validation is template-aware: known forms are checked against their own
// fields, and freeform issues (including API-created maintenance/roadmap
// issues) pass — operators opt into freeform validation by setting the var.
func requiredSections() []string {
	raw := os.Getenv("REQUIRED_SECTIONS")
	if raw == "" {
		return nil
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
	check := func(want string) []string {
		var out []string
		found := false
		for heading, content := range sections {
			if strings.Contains(heading, want) {
				found = true
				text := strings.TrimSpace(content)
				if len(text) < 5 || placeholderRe.MatchString(text) {
					out = append(out, fmt.Sprintf("**%s** is present but empty or placeholder", want))
				}
				break
			}
		}
		if !found {
			out = append(out, fmt.Sprintf("**%s** section is missing", want))
		}
		return out
	}

	if len(required) > 0 {
		// explicit operator configuration applies to every issue
		var problems []string
		for _, want := range required {
			problems = append(problems, check(want)...)
		}
		return problems
	}

	// template-aware default: validate against the form actually used (#112)
	for _, tmpl := range knownTemplates {
		for heading := range sections {
			if strings.Contains(heading, tmpl.signature) {
				var problems []string
				for _, want := range tmpl.required {
					problems = append(problems, check(want)...)
				}
				return problems
			}
		}
	}
	// freeform issue — no form detected, nothing to validate against
	return nil
}

func main() {
	cfg := ghbot.LoadConfig("issue-form-validator", "8127")
	if ghbot.HandleCLIArgs("issue-form-validator", cfg, "webhook") {
		return
	}
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
			// valid — remove the label AND resolve any earlier error comment;
			// leaving the comment contradicting the label confused reporters (#112)
			client.Do("DELETE", fmt.Sprintf("/repos/%s/issues/%d/labels/invalid-issue",
				ev.Repo.FullName, ev.Issue.Number), nil)
			resolveComment(client, ev.Repo.FullName, ev.Issue.Number)
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

// resolveComment marks the managed feedback comment resolved so an edited,
// now-valid issue no longer displays stale error text (#112).
func resolveComment(client *ghbot.Client, repo string, number int) {
	comments, err := client.GetAll(fmt.Sprintf("/repos/%s/issues/%d/comments", repo, number))
	if err != nil {
		return
	}
	for _, raw := range comments {
		var c struct {
			ID   int    `json:"id"`
			Body string `json:"body"`
		}
		if json.Unmarshal(raw, &c) != nil {
			continue
		}
		if strings.Contains(c.Body, marker) && !strings.Contains(c.Body, "✅") {
			client.Do("PATCH", fmt.Sprintf("/repos/%s/issues/comments/%d", repo, c.ID),
				map[string]string{"body": marker + "\n✅ Template feedback resolved — thanks for updating the issue!"})
		}
	}
}
