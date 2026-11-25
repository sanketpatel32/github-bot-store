// todo-tracker — scans pushed commits for added/removed TODO/FIXME comments
// and reconciles tracking issues: opens one per new TODO (stable hash marker
// in the body), closes the issue when the TODO disappears from the default
// branch. Added lines are parsed from the actual diff, not raw patch text.
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

const markerPrefix = "<!-- todo-tracker:"

type PushEvent struct {
	Ref  string `json:"ref"`
	Repo struct {
		FullName      string `json:"full_name"`
		DefaultBranch string `json:"default_branch"`
	} `json:"repository"`
	Commits []struct {
		ID     string `json:"id"`
		URL    string `json:"url"`
		Author struct {
			Username string `json:"username"`
		} `json:"author"`
		Message string `json:"message"`
	} `json:"commits"`
}

// addedLineRe matches diff lines adding content; the +/- prefix is stripped
// by the patch parser, never regexed out of raw text.
var todoRe = regexp.MustCompile(`(?:TODO|FIXME)\b[:\s]?(.*)`)

// commentNoise is everything that may precede a marker for it to still count
// as a real TODO comment: comment openers and decoration. If anything else
// (letters, words) comes before it, the marker is prose mentioning TODO —
// e.g. a comment explaining the scanner itself (#105).
func isCommentPrefix(s string) bool {
	for _, r := range s {
		if !strings.ContainsRune(" \t/*#-;<!()'\"`+", r) {
			return false
		}
	}
	return true
}

func marker(hash string) string { return markerPrefix + hash + " -->" }

func todoHash(file, text string) string {
	h := sha1.Sum([]byte(file + "|" + text))
	return hex.EncodeToString(h[:])[:8]
}

// scannable reports whether a file is source/config prose where TODO
// comments are meaningful. Generated catalog pages (docs/bots/*.html) and
// markdown cause false positives — the bots' own descriptions mention
// TODO/FIXME as words, which the todo-tracker then "found" in itself.
func scannable(path string) bool {
	for _, skip := range []string{".html", ".md", ".json", ".svg", ".png", ".lock", ".sum"} {
		if strings.HasSuffix(path, skip) {
			return false
		}
	}
	for _, ext := range []string{".go", ".js", ".ts", ".py", ".java", ".c", ".h", ".cpp", ".rs", ".rb", ".sh", ".yml", ".yaml", ".toml"} {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

// parsePatch extracts added and removed TODOs from a unified diff patch.
// Handles truncated patches gracefully (they just yield fewer lines) and
// renamed files (caller passes the current filename).
func parsePatch(patch string) (added, removed []string) {
	for _, line := range strings.Split(patch, "\n") {
		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") ||
			strings.HasPrefix(line, "@@") {
			continue
		}
		if len(line) == 0 {
			continue
		}
		sign := line[0]
		content := line[1:]
		loc := todoRe.FindStringSubmatchIndex(content)
		if loc == nil {
			continue
		}
		// the marker must be the first thing a comment says — matches buried
		// in prose (like this very sentence) are not tasks (#105)
		if !isCommentPrefix(content[:loc[0]]) {
			continue
		}
		text := strings.TrimSpace(content[loc[2]:loc[3]])
		if len([]rune(text)) < 4 {
			// empty or one-word markers carry no actionable content and only
			// create noise issues (#105)
			continue
		}
		switch sign {
		case '+':
			added = append(added, text)
		case '-':
			removed = append(removed, text)
		}
	}
	return added, removed
}

// findOpenTODOIssue returns the issue number for a hash marker, or 0.
func findOpenTODOIssue(client *ghbot.Client, repo, hash string) int {
	body, err := client.Do("GET", fmt.Sprintf(
		"/search/issues?q=repo:%s+type:issue+state:open+%s&per_page=1", repo, marker(hash)), nil)
	if err != nil {
		return 0
	}
	var result struct {
		Items []struct {
			Number int `json:"number"`
		} `json:"items"`
	}
	if json.Unmarshal(body, &result) != nil || len(result.Items) == 0 {
		return 0
	}
	return result.Items[0].Number
}

func main() {
	cfg := ghbot.LoadConfig("todo-tracker", "8108")
	if ghbot.HandleCLIArgs("todo-tracker", cfg) {
		return
	}
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
			return
		}

		for _, commit := range ev.Commits {
			body, err := client.Do("GET",
				fmt.Sprintf("/repos/%s/commits/%s", ev.Repo.FullName, commit.ID), nil)
			if err != nil {
				log.Printf("commit %s: %v", commit.ID[:7], err)
				continue
			}
			var detail struct {
				Files []struct {
					Filename string `json:"filename"`
					Patch    string `json:"patch"` // empty when truncated/binary
				} `json:"files"`
			}
			if json.Unmarshal(body, &detail) != nil {
				continue
			}

			for _, f := range detail.Files {
				if f.Patch == "" {
					continue // truncated or binary — skip rather than guess
				}
				if !scannable(f.Filename) {
					continue
				}
				added, removed := parsePatch(f.Patch)

				for _, text := range added {
					hash := todoHash(f.Filename, text)
					if findOpenTODOIssue(client, ev.Repo.FullName, hash) != 0 {
						continue // already tracked — no duplicates
					}
					title := fmt.Sprintf("TODO [%s]: %.60s", hash, text)
					bodyText := fmt.Sprintf(
						"%s\nFound in `%s` by @%s in commit %.7s:\n\n```\n%s\n```\n\n%s\n\n_This issue closes automatically when the TODO is removed from `%s`._",
						marker(hash), f.Filename, commit.Author.Username, commit.ID, text,
						commit.URL, ev.Repo.DefaultBranch)
					_, err := client.Do("POST",
						fmt.Sprintf("/repos/%s/issues", ev.Repo.FullName),
						map[string]any{"title": title, "body": bodyText, "labels": []string{"todo"}})
					if err != nil {
						log.Printf("create issue: %v", err)
						continue
					}
					log.Printf("opened TODO issue [%s] for %s", hash, f.Filename)
				}

				for _, text := range removed {
					hash := todoHash(f.Filename, text)
					number := findOpenTODOIssue(client, ev.Repo.FullName, hash)
					if number == 0 {
						continue
					}
					client.Comment(ev.Repo.FullName, number, fmt.Sprintf(
						"This TODO was removed in commit %.7s — closing automatically.", commit.ID))
					client.Do("PATCH", fmt.Sprintf("/repos/%s/issues/%d", ev.Repo.FullName, number),
						map[string]string{"state": "closed"})
					log.Printf("closed TODO issue #%d [%s] — removed from %s", number, hash, f.Filename)
				}
			}
		}
	})
}
