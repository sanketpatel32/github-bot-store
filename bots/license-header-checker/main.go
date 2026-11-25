// license-header-checker — verifies new source files in a PR carry a license
// header comment. Posts one managed comment listing files that miss it and
// a commit status.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

const marker = "<!-- license-header-checker -->"

type PREvent struct {
	Action string `json:"action"`
	Number int    `json:"number"`
	PR     struct {
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
	} `json:"pull_request"`
	Repo struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func headerNeedle() string {
	if h := os.Getenv("HEADER_TEXT"); h != "" {
		return h
	}
	return "copyright"
}

func sourceExt(path string) bool {
	for _, ext := range strings.Split(os.Getenv("SOURCE_EXTENSIONS"), ",") {
		if ext == "" {
			continue
		}
		if strings.HasSuffix(path, strings.TrimSpace(ext)) {
			return true
		}
	}
	// defaults when unset
	if os.Getenv("SOURCE_EXTENSIONS") == "" {
		for _, ext := range []string{".go", ".js", ".ts", ".py", ".java", ".c", ".h", ".cpp", ".rs", ".rb"} {
			if strings.HasSuffix(path, ext) {
				return true
			}
		}
	}
	return false
}

func main() {
	cfg := ghbot.LoadConfig("license-header-checker", "8120")
	client := ghbot.NewClient(cfg.Token)

	ghbot.Serve(cfg, "license-header-checker", func(event string, payload []byte) {
		if event != "pull_request" {
			return
		}
		var ev PREvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		switch ev.Action {
		case "opened", "synchronize", "reopened":
		default:
			return
		}

		files, err := client.GetAll(fmt.Sprintf("/repos/%s/pulls/%d/files", ev.Repo.FullName, ev.Number))
		if err != nil {
			log.Printf("files: %v", err)
			return
		}
		needle := strings.ToLower(headerNeedle())
		var missing []string
		for _, raw := range files {
			var f struct {
				Filename string `json:"filename"`
				Status   string `json:"status"`
				Patch    string `json:"patch"`
			}
			if json.Unmarshal(raw, &f) != nil {
				continue
			}
			// only newly added source files need headers
			if f.Status != "added" || !sourceExt(f.Filename) || f.Patch == "" {
				continue
			}
			// the patch contains only the first chunk of the file for new
			// files — good enough since headers live at the top
			if !strings.Contains(strings.ToLower(f.Patch), needle) {
				missing = append(missing, f.Filename)
			}
		}

		state, desc := "success", "all new source files carry license headers"
		if len(missing) > 0 {
			state = "failure"
			desc = fmt.Sprintf("%d file(s) missing license header", len(missing))
		}
		client.Do("POST", fmt.Sprintf("/repos/%s/statuses/%s", ev.Repo.FullName, ev.PR.Head.SHA),
			map[string]string{"state": state, "context": "license-headers", "description": desc})

		// one managed comment listing missing files
		body := marker + "\n"
		if len(missing) > 0 {
			body += "**Missing license headers** in new files:\n"
			for _, m := range missing {
				body += "- `" + m + "`\n"
			}
			body += fmt.Sprintf("\nExpected header text: `%s` (configure with HEADER_TEXT)", headerNeedle())
		} else {
			body += "✅ All new source files have license headers."
		}
		upsertComment(client, ev.Repo.FullName, ev.Number, body)
		log.Printf("PR #%d: %d files missing headers", ev.Number, len(missing))
	})
}

func upsertComment(client *ghbot.Client, repo string, number int, body string) {
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
