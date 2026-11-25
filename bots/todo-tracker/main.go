// todo-tracker — scans pushed commits for newly added TODO/FIXME comments
// and opens a tracking issue for each one (deduped by a hash in the title).
package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

type PushEvent struct {
	Ref    string `json:"ref"`
	Repo   struct {
		FullName      string `json:"full_name"`
		DefaultBranch string `json:"default_branch"`
	} `json:"repository"`
	Commits []struct {
		ID      string `json:"id"`
		URL     string `json:"url"`
		Author  struct{ Username string } `json:"author"`
		Message string `json:"message"`
	} `json:"commits"`
}

var todoRe = regexp.MustCompile(`(?m)^\+.*(?:TODO|FIXME)\b[:\s]?(.*)$`)

func todoHash(file string, line string) string {
	h := sha1.Sum([]byte(file + "|" + line))
	return hex.EncodeToString(h)[:8]
}

func alreadyTracked(client *ghbot.Client, repo, hash string) bool {
	path := fmt.Sprintf("/search/issues?q=repo:%s+in:title+%s&per_page=1", repo, hash)
	body, err := client.Do("GET", path, nil)
	if err != nil {
		return true // fail closed — don't spam issues on search errors
	}
	var result struct {
		TotalCount int `json:"total_count"`
	}
	json.Unmarshal(body, &result)
	return result.TotalCount > 0
}

func main() {
	cfg := ghbot.LoadConfig("todo-tracker", "8108")
	client := ghbot.NewClient(cfg.Token)

	ghbot.Serve(cfg, "todo-tracker", func(event string, payload []byte) {
		if event != "push" {
			return
		}
		var ev PushEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		if !strings.HasSuffix(ev.Ref, "/"+ev.Repo.DefaultBranch) {
			return // only track TODOs that land on the default branch
		}

		for _, commit := range ev.Commits {
			// fetch full diff for each commit
			body, err := client.Do("GET",
				fmt.Sprintf("/repos/%s/commits/%s", ev.Repo.FullName, commit.ID), nil)
			if err != nil {
				log.Printf("commit %s: %v", commit.ID[:7], err)
				continue
			}
			var detail struct {
				Files []struct {
					Filename string `json:"filename"`
					Patch    string `json:"patch"`
				} `json:"files"`
			}
			if err := json.Unmarshal(body, &detail); err != nil {
				continue
			}

			for _, f := range detail.Files {
				for _, m := range todoRe.FindAllStringSubmatch(f.Patch, -1) {
					text := strings.TrimSpace(m[1])
					if text == "" {
						text = "(no description)"
					}
					hash := todoHash(f.Filename, text)
					if alreadyTracked(client, ev.Repo.FullName, hash) {
						continue
					}
					title := fmt.Sprintf("TODO [%s]: %.60s", hash, text)
					bodyText := fmt.Sprintf(
						"Found in `%s` by @%s in commit %.7s:\n\n```\n%s\n```\n\n%s",
						f.Filename, commit.Author.Username, commit.ID, text, commit.URL)
					_, err := client.Do("POST",
						fmt.Sprintf("/repos/%s/issues", ev.Repo.FullName),
						map[string]any{"title": title, "body": bodyText, "labels": []string{"todo"}})
					if err != nil {
						log.Printf("create issue: %v", err)
						continue
					}
					log.Printf("opened TODO issue [%s] for %s", hash, f.Filename)
				}
			}
		}
	})
}
