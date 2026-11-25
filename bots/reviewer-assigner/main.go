// reviewer-assigner — assigns reviewers to PRs based on which paths changed.
// Supports a CODEOWNERS file in the repo (standard syntax, including @org/team
// reviewers) or a local reviewers.json fallback. Skips drafts, the author,
// and anyone already requested or who already reviewed.
package main

import (
	"encoding/base64"
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
		Draft bool `json:"draft"`
		User  struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"pull_request"`
	Repo struct {
		FullName      string `json:"full_name"`
		DefaultBranch string `json:"default_branch"`
	} `json:"repository"`
}

// rule is one path pattern mapped to owners (@user or @org/team).
type rule struct {
	pattern string
	owners  []string
}

// parseCODEOWNERS parses standard CODEOWNERS syntax: lines of
// "pattern owner1 owner2", #-comments. Globs are simplified to their
// static prefix for prefix matching.
func parseCODEOWNERS(content string) []rule {
	var rules []rule
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pattern := strings.TrimPrefix(fields[0], "/")
		if idx := strings.IndexAny(pattern, "*?["); idx != -1 {
			pattern = pattern[:idx]
		}
		rules = append(rules, rule{pattern, fields[1:]})
	}
	return rules
}

// loadRules tries the repo's CODEOWNERS first, then local reviewers.json.
func loadRules(client *ghbot.Client, repo, branch string) []rule {
	for _, path := range []string{".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS"} {
		body, err := client.Do("GET",
			fmt.Sprintf("/repos/%s/contents/%s?ref=%s", repo, path, branch), nil)
		if err != nil {
			continue
		}
		var file struct {
			Content string `json:"content"`
		}
		if json.Unmarshal(body, &file) != nil || file.Content == "" {
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(file.Content, "\n", ""))
		if err == nil && len(decoded) > 0 {
			log.Printf("using %s", path)
			return parseCODEOWNERS(string(decoded))
		}
	}

	if data, err := os.ReadFile("reviewers.json"); err == nil {
		var cfg map[string][]string
		if json.Unmarshal(data, &cfg) == nil {
			var rules []rule
			for prefix, owners := range cfg {
				rules = append(rules, rule{prefix, owners})
			}
			log.Printf("using local reviewers.json")
			return rules
		}
	}
	log.Printf("no CODEOWNERS or reviewers.json found — nothing to assign")
	return nil
}

// matchOwners returns deduped owners for the changed files; the longest
// matching pattern wins per file.
func matchOwners(rules []rule, filenames []string) []string {
	set := map[string]bool{}
	for _, f := range filenames {
		bestLen := -1
		var best []string
		for _, r := range rules {
			if strings.HasPrefix(f, r.pattern) && len(r.pattern) > bestLen {
				bestLen = len(r.pattern)
				best = r.owners
			}
		}
		for _, o := range best {
			set[o] = true
		}
	}
	out := make([]string, 0, len(set))
	for o := range set {
		out = append(out, o)
	}
	return out
}

func main() {
	cfg := ghbot.LoadConfig("reviewer-assigner", "8106")
	if ghbot.HandleCLIArgs("reviewer-assigner", cfg) {
		return
	}
	client := ghbot.NewClient(cfg.Token)

	ghbot.Serve(cfg, "reviewer-assigner", func(event string, payload []byte) {
		if event != "pull_request" {
			return
		}
		var ev PREvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		if ev.Action != "opened" && ev.Action != "synchronize" && ev.Action != "ready_for_review" {
			return
		}
		if ev.PR.Draft {
			log.Printf("PR #%d is draft — not assigning reviewers", ev.Number)
			return
		}

		rules := loadRules(client, ev.Repo.FullName, ev.Repo.DefaultBranch)
		if len(rules) == 0 {
			return
		}

		rawFiles, err := client.GetAll(
			fmt.Sprintf("/repos/%s/pulls/%d/files", ev.Repo.FullName, ev.Number))
		if err != nil {
			log.Printf("files: %v", err)
			return
		}
		var filenames []string
		for _, raw := range rawFiles {
			var f struct {
				Filename         string `json:"filename"`
				PreviousFilename string `json:"previous_filename"`
			}
			if json.Unmarshal(raw, &f) != nil {
				continue
			}
			// renamed files count under both names — owners of the old
			// location often care about moves out of their area
			filenames = append(filenames, f.Filename)
			if f.PreviousFilename != "" {
				filenames = append(filenames, f.PreviousFilename)
			}
		}

		owners := matchOwners(rules, filenames)
		var users, teams []string
		for _, o := range owners {
			o = strings.TrimPrefix(o, "@")
			if strings.Contains(o, "/") {
				teams = append(teams, strings.SplitN(o, "/", 2)[1]) // org/team -> team slug
			} else if o != ev.PR.User.Login {
				users = append(users, o)
			}
		}

		// skip anyone already requested or who already reviewed
		existing := existingReviewers(client, ev.Repo.FullName, ev.Number)
		var finalUsers []string
		for _, u := range users {
			if !existing[u] {
				finalUsers = append(finalUsers, u)
			}
		}

		if len(finalUsers) == 0 && len(teams) == 0 {
			log.Printf("PR #%d: no new reviewers to assign", ev.Number)
			return
		}
		req := map[string]any{}
		if len(finalUsers) > 0 {
			req["reviewers"] = finalUsers
		}
		if len(teams) > 0 {
			req["team_reviewers"] = teams
		}
		_, err = client.Do("POST",
			fmt.Sprintf("/repos/%s/pulls/%d/requested_reviewers", ev.Repo.FullName, ev.Number), req)
		if err != nil {
			log.Printf("assign: %v", err)
			return
		}
		log.Printf("PR #%d: requested users %v teams %v", ev.Number, finalUsers, teams)
	})
}

// existingReviewers returns logins already requested or already reviewed.
func existingReviewers(client *ghbot.Client, repo string, number int) map[string]bool {
	out := map[string]bool{}
	body, err := client.Do("GET",
		fmt.Sprintf("/repos/%s/pulls/%d/requested_reviewers", repo, number), nil)
	if err == nil {
		var rr struct {
			Users []struct {
				Login string `json:"login"`
			} `json:"users"`
		}
		if json.Unmarshal(body, &rr) == nil {
			for _, u := range rr.Users {
				out[u.Login] = true
			}
		}
	}
	body, err = client.Do("GET",
		fmt.Sprintf("/repos/%s/pulls/%d/reviews?per_page=100", repo, number), nil)
	if err == nil {
		var reviews []struct {
			User struct {
				Login string `json:"login"`
			} `json:"user"`
		}
		if json.Unmarshal(body, &reviews) == nil {
			for _, r := range reviews {
				out[r.User.Login] = true
			}
		}
	}
	return out
}
