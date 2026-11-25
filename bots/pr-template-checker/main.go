// pr-template-checker — validates that PR descriptions actually populate
// required template sections. Parses Markdown headings, strips HTML comments
// and template placeholder text, and updates one persistent feedback comment
// instead of spamming new ones on each edit.
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

const marker = "<!-- pr-template-checker -->"

type PREvent struct {
	Action string `json:"action"`
	Number int    `json:"number"`
	PR     struct {
		Body  string `json:"body"`
		Title string `json:"title"`
		User  struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"pull_request"`
	Repo struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

var (
	headingRe = regexp.MustCompile(`(?mi)^#{1,6}\s+(.+?)\s*#*\s*$`)
	commentRe = regexp.MustCompile(`(?s)<!--.*?-->`)
)

// placeholderPatterns match common template filler that shouldn't count as
// populated content.
var placeholderPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^\s*(your .+ here|describe .+|explain .+|enter .+|n/?a|tbd|todo|fill .+ in|_?no response_?)[\s.]*$`),
}

// parseSections splits a markdown body into heading -> content pairs.
// HTML comments are stripped first. Content before the first heading is
// returned under "".
func parseSections(body string) map[string]string {
	body = commentRe.ReplaceAllString(body, "")
	sections := map[string]string{}
	locs := headingRe.FindAllStringSubmatchIndex(body, -1)
	for i, loc := range locs {
		title := strings.ToLower(strings.TrimSpace(body[loc[2]:loc[3]]))
		start := loc[1]
		end := len(body)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		sections[title] = strings.TrimSpace(body[start:end])
	}
	return sections
}

// isPopulated reports whether section content is real text — not empty,
// not just placeholder filler, not a repetition of the template prompt.
func isPopulated(content string) bool {
	content = strings.TrimSpace(content)
	if len(content) < 5 {
		return false
	}
	for _, p := range placeholderPatterns {
		if p.MatchString(content) {
			return false
		}
	}
	return true
}

// findSection looks up a required keyword against parsed headings.
// Matches when the heading *contains* the keyword.
func findSection(sections map[string]string, keyword string) (string, bool) {
	for title, content := range sections {
		if strings.Contains(title, keyword) {
			return content, true
		}
	}
	return "", false
}

func validate(body string, required, optional []string) (missing, empty []string) {
	sections := parseSections(body)
	for _, req := range required {
		content, found := findSection(sections, req)
		if !found {
			missing = append(missing, req)
		} else if !isPopulated(content) {
			empty = append(empty, req)
		}
	}
	// optional sections: fine when absent, but must be populated when present
	for _, opt := range optional {
		if content, found := findSection(sections, opt); found && !isPopulated(content) {
			empty = append(empty, opt)
		}
	}
	return missing, empty
}

func main() {
	cfg := ghbot.LoadConfig("pr-template-checker", "8112")
	client := ghbot.NewClient(cfg.Token)

	required := splitEnv("REQUIRED_SECTIONS", "summary,testing")
	optional := splitEnv("OPTIONAL_SECTIONS", "")

	ghbot.Serve(cfg, "pr-template-checker", func(event string, payload []byte) {
		if event != "pull_request" {
			return
		}
		var ev PREvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		if ev.Action != "opened" && ev.Action != "edited" && ev.Action != "synchronize" {
			return
		}

		body := strings.TrimSpace(ev.PR.Body)
		var feedback string
		if body == "" {
			feedback = "Your PR description is empty — please fill in the template sections."
		} else {
			missing, empty := validate(body, required, optional)
			if len(missing) == 0 && len(empty) == 0 {
				// all good — resolve any prior feedback comment
				resolveComment(client, ev.Repo.FullName, ev.Number)
				return
			}
			var parts []string
			if len(missing) > 0 {
				parts = append(parts, "**missing sections**: "+strings.Join(missing, ", "))
			}
			if len(empty) > 0 {
				parts = append(parts, "**sections with placeholder/empty content**: "+strings.Join(empty, ", "))
			}
			feedback = "Your PR description needs a bit more detail — " + strings.Join(parts, "; ") +
				". Even a one-liner per section helps reviewers move faster."
		}
		upsertComment(client, ev.Repo.FullName, ev.Number, feedback)
		log.Printf("PR #%d feedback updated", ev.Number)
	})
}

func splitEnv(key, fallback string) []string {
	v := os.Getenv(key)
	if v == "" {
		v = fallback
	}
	var out []string
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(strings.ToLower(s)); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// upsertComment edits the bot's existing feedback comment or posts a new one.
func upsertComment(client *ghbot.Client, repo string, number int, feedback string) {
	body := marker + "\n" + feedback
	id := findComment(client, repo, number)
	if id != 0 {
		if _, err := client.Do("PATCH", fmt.Sprintf("/repos/%s/issues/comments/%d", repo, id),
			map[string]string{"body": body}); err == nil {
			return
		}
	}
	if err := client.Comment(repo, number, body); err != nil {
		log.Printf("comment: %v", err)
	}
}

// resolveComment marks the feedback resolved when everything is fixed.
func resolveComment(client *ghbot.Client, repo string, number int) {
	id := findComment(client, repo, number)
	if id == 0 {
		return
	}
	client.Do("PATCH", fmt.Sprintf("/repos/%s/issues/comments/%d", repo, id),
		map[string]string{"body": marker + "\n✅ All required sections are populated — thanks for updating!"})
}

func findComment(client *ghbot.Client, repo string, number int) int64 {
	body, err := client.Do("GET",
		fmt.Sprintf("/repos/%s/issues/%d/comments?per_page=100", repo, number), nil)
	if err != nil {
		return 0
	}
	var comments []struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
	}
	if json.Unmarshal(body, &comments) != nil {
		return 0
	}
	for _, c := range comments {
		if strings.Contains(c.Body, marker) {
			return c.ID
		}
	}
	return 0
}
