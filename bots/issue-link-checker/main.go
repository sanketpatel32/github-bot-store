// issue-link-checker — validates external links posted in new issues and
// comments once on the issue listing any that don't resolve. Catches
// reproduction links and references that are dead on arrival.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

const marker = "<!-- issue-link-checker -->"

var linkRe = regexp.MustCompile(`https?://[^\s)\]>"]+`)

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

func linkOK(hc *http.Client, url string) bool {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "github-bot-store issue-link-checker")
	resp, err := hc.Do(req)
	if err != nil {
		req, _ = http.NewRequest("GET", url, nil)
		resp, err = hc.Do(req)
		if err != nil {
			return false
		}
	}
	defer resp.Body.Close()
	return resp.StatusCode < 400
}

func main() {
	cfg := ghbot.LoadConfig("issue-link-checker", "8126")
	if ghbot.HandleCLIArgs("issue-link-checker", cfg) {
		return
	}
	client := ghbot.NewClient(cfg.Token)

	ghbot.Serve(cfg, "issue-link-checker", func(event string, payload []byte) {
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
		if ev.Issue.PR != nil || ev.Issue.Body == "" {
			return
		}

		urls := map[string]bool{}
		for _, u := range linkRe.FindAllString(ev.Issue.Body, -1) {
			u = strings.TrimRight(u, ".,;")
			if strings.Contains(u, "github.com") {
				continue // github links are virtually always valid; skip quota burn
			}
			urls[u] = true
		}
		if len(urls) == 0 {
			return
		}

		hc := &http.Client{Timeout: 10 * time.Second}
		var broken []string
		for u := range urls {
			if !linkOK(hc, u) {
				broken = append(broken, u)
			}
		}

		if len(broken) == 0 {
			log.Printf("issue #%d: all %d links ok", ev.Issue.Number, len(urls))
			return
		}
		var sb strings.Builder
		sb.WriteString(marker + "\n**Links in this issue that don't resolve:**\n\n")
		for _, b := range broken {
			fmt.Fprintf(&sb, "- %s\n", b)
		}
		sb.WriteString("\nDead repro links make triage hard — please double-check them.")
		// one managed comment — update if we already commented
		upsert(client, ev.Repo.FullName, ev.Issue.Number, sb.String())
		log.Printf("issue #%d: %d broken links flagged", ev.Issue.Number, len(broken))
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
