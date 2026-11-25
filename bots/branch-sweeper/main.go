// branch-sweeper — deletes the head branch when a PR is merged.
// Safety rails: skips forks, protected branches (both by pattern and the
// repo's actual branch-protection settings), the default branch, and any
// branch still referenced by another open PR.
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
	PR     struct {
		Merged bool `json:"merged"`
		Head   struct {
			Ref  string `json:"ref"`
			SHA  string `json:"sha"`
			Repo struct {
				FullName string `json:"full_name"`
				Fork     bool   `json:"fork"`
			} `json:"repo"`
		} `json:"head"`
	} `json:"pull_request"`
	Repo struct {
		FullName      string `json:"full_name"`
		DefaultBranch string `json:"default_branch"`
	} `json:"repository"`
}

var defaultProtected = []string{
	"main", "master", "develop", "dev", "release", "staging", "production",
}

// protectedByPattern matches exact names and simple prefix patterns (release/*).
func protectedByPattern(branch string, patterns []string) bool {
	for _, p := range patterns {
		p = strings.TrimSpace(strings.ToLower(p))
		if p == "" {
			continue
		}
		if strings.HasSuffix(p, "*") {
			if strings.HasPrefix(branch, strings.TrimSuffix(p, "*")) {
				return true
			}
		} else if branch == p {
			return true
		}
	}
	return false
}

// branchInUseByOtherPR returns true if another open PR uses this branch as head.
func branchInUseByOtherPR(client *ghbot.Client, repo, branch string, mergedPRNumber int) bool {
	items, err := client.GetAll(fmt.Sprintf("/repos/%s/pulls?state=open&head=%s", repo, branch))
	if err != nil {
		// head filter needs owner:branch sometimes; fall back to scanning
		items, err = client.GetAll(fmt.Sprintf("/repos/%s/pulls?state=open", repo))
		if err != nil {
			log.Printf("open pr scan: %v — treating branch as in use", err)
			return true
		}
	}
	for _, raw := range items {
		var pr struct {
			Number int `json:"number"`
			Head   struct {
				Ref string `json:"ref"`
			} `json:"head"`
		}
		if json.Unmarshal(raw, &pr) != nil {
			continue
		}
		if pr.Head.Ref == branch && pr.Number != mergedPRNumber {
			return true
		}
	}
	return false
}

func main() {
	cfg := ghbot.LoadConfig("branch-sweeper", "8104")
	if ghbot.HandleCLIArgs("branch-sweeper", cfg) {
		return
	}
	client := ghbot.NewClient(cfg.Token)

	patterns := append([]string{}, defaultProtected...)
	if extra := os.Getenv("PROTECTED_BRANCHES"); extra != "" {
		patterns = append(patterns, strings.Split(extra, ",")...)
	}

	ghbot.Serve(cfg, "branch-sweeper", func(event string, payload []byte) {
		if event != "pull_request" {
			return
		}
		var ev PREvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		if ev.Action != "closed" || !ev.PR.Merged {
			return
		}

		head := ev.PR.Head
		branch := head.Ref

		if head.Repo.Fork || head.Repo.FullName != ev.Repo.FullName {
			log.Printf("skip fork branch %s", branch)
			return
		}
		if branch == ev.Repo.DefaultBranch {
			log.Printf("skip default branch %s", branch)
			return
		}
		if protectedByPattern(strings.ToLower(branch), patterns) {
			log.Printf("skip protected-pattern branch %s", branch)
			return
		}

		// confirm the branch tip is still exactly the merged head — if someone
		// pushed after merge, the branch may carry unmerged work now
		body, err := client.Do("GET",
			fmt.Sprintf("/repos/%s/branches/%s", ev.Repo.FullName, branch), nil)
		if err != nil {
			log.Printf("branch %s lookup failed (already gone?): %v", branch, err)
			return
		}
		var info struct {
			Protected bool `json:"protected"`
			Commit    struct {
				SHA string `json:"sha"`
			} `json:"commit"`
		}
		if json.Unmarshal(body, &info) != nil {
			return
		}
		if info.Protected {
			log.Printf("skip protected branch %s", branch)
			return
		}
		if info.Commit.SHA != head.SHA {
			log.Printf("skip %s — tip moved past merged head (%s != %s)", branch,
				info.Commit.SHA[:7], head.SHA[:7])
			return
		}
		if branchInUseByOtherPR(client, ev.Repo.FullName, branch, 0) {
			log.Printf("skip %s — still used by another open PR", branch)
			return
		}

		_, err = client.Do("DELETE", "/repos/"+ev.Repo.FullName+"/git/refs/heads/"+branch, nil)
		if err != nil {
			log.Printf("delete %s: %v", branch, err)
			return
		}
		log.Printf("deleted merged branch %s", branch)
	})
}
