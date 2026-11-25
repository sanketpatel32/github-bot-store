// docs-link-checker — cron bot scanning markdown files for dead external
// links. Opens one managed issue listing broken links; updates it on each
// run and closes it when everything is fixed.
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

const marker = "<!-- docs-link-checker -->"

var linkRe = regexp.MustCompile(`\[([^\]]*)\]\((https?://[^)\s]+)\)`)

func checkURL(client *http.Client, url string) bool {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "github-bot-store docs-link-checker")
	resp, err := client.Do(req)
	if err != nil {
		// fall back to GET — some servers reject HEAD
		req, _ = http.NewRequest("GET", url, nil)
		req.Header.Set("User-Agent", "github-bot-store docs-link-checker")
		resp, err = client.Do(req)
		if err != nil {
			return false
		}
	}
	defer resp.Body.Close()
	return resp.StatusCode < 400
}

func sweep(gh *ghbot.Client, repo, branch string) {
	// fetch markdown file list from the git tree
	body, err := gh.Do("GET", fmt.Sprintf("/repos/%s/git/trees/%s?recursive=1", repo, branch), nil)
	if err != nil {
		log.Printf("tree: %v", err)
		return
	}
	var tree struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
	}
	if json.Unmarshal(body, &tree) != nil {
		return
	}

	type broken struct {
		file, url string
	}
	var results []broken
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	hc := &http.Client{Timeout: 10 * time.Second}
	checked := map[string]*bool{}
	var checkedMu sync.Mutex

	for _, entry := range tree.Tree {
		if entry.Type != "blob" || !strings.HasSuffix(entry.Path, ".md") {
			continue
		}
		content, err := gh.Do("GET",
			fmt.Sprintf("/repos/%s/contents/%s?ref=%s", repo, entry.Path, branch), nil)
		if err != nil {
			continue
		}
		var file struct {
			Content string `json:"content"`
		}
		if json.Unmarshal(content, &file) != nil {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(file.Content, "\n", ""))
		if err != nil {
			continue
		}
		for _, m := range linkRe.FindAllStringSubmatch(string(raw), -1) {
			url := m[2]
			// skip github.com links to private resources and anchors
			if strings.Contains(url, "github.com") && strings.Contains(url, "/blob/") {
				continue
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(file, url string) {
				defer wg.Done()
				defer func() { <-sem }()
				checkedMu.Lock()
				if res, ok := checked[url]; ok {
					checkedMu.Unlock()
					if !*res {
						mu.Lock()
						results = append(results, broken{file, url})
						mu.Unlock()
					}
					return
				}
				checkedMu.Unlock()
				ok := checkURL(hc, url)
				checkedMu.Lock()
				checked[url] = &ok
				checkedMu.Unlock()
				if !ok {
					mu.Lock()
					results = append(results, broken{file, url})
					mu.Unlock()
				}
			}(entry.Path, url)
		}
	}
	wg.Wait()

	title := fmt.Sprintf("Docs link check: %d broken links", len(results))
	var sb strings.Builder
	sb.WriteString(marker + "\n")
	if len(results) == 0 {
		sb.WriteString("✅ All links in markdown files resolve.\n")
	} else {
		sb.WriteString("**Broken links found:**\n\n")
		seen := map[string]bool{}
		for _, b := range results {
			key := b.file + b.url
			if seen[key] {
				continue
			}
			seen[key] = true
			fmt.Fprintf(&sb, "- `%s` → %s\n", b.file, b.url)
		}
		sb.WriteString("\n_This issue updates itself on each run._")
	}
	fmt.Println(sb.String())

	// upsert managed issue (only when broken links exist; close when clean)
	search, err := gh.Do("GET", fmt.Sprintf(
		"/search/issues?q=repo:%s+type:issue+state:open+in:body+%s&per_page=1", repo, marker), nil)
	existing := 0
	if err == nil {
		var result struct {
			Items []struct {
				Number int `json:"number"`
			} `json:"items"`
		}
		if json.Unmarshal(search, &result) == nil && len(result.Items) > 0 {
			existing = result.Items[0].Number
		}
	}
	if len(results) == 0 {
		if existing != 0 {
			gh.Comment(repo, existing, "All links fixed — closing.")
			gh.Do("PATCH", fmt.Sprintf("/repos/%s/issues/%d", repo, existing),
				map[string]string{"state": "closed"})
		}
		return
	}
	if existing != 0 {
		gh.Do("PATCH", fmt.Sprintf("/repos/%s/issues/%d", repo, existing),
			map[string]string{"title": title, "body": sb.String()})
	} else {
		gh.Do("POST", fmt.Sprintf("/repos/%s/issues", repo),
			map[string]any{"title": title, "body": sb.String(), "labels": []string{"documentation"}})
	}
	log.Printf("scan complete: %d broken links", len(results))
}

func main() {
	cfg := ghbot.LoadConfig("docs-link-checker", "8125")
	if ghbot.HandleCLIArgs("docs-link-checker", cfg) {
		return
	}
	client := ghbot.NewClient(cfg.Token)
	repo := os.Getenv("REPO")
	if repo == "" {
		log.Fatal("REPO is not set (owner/repo)")
	}
	branch := os.Getenv("BRANCH")
	if branch == "" {
		branch = "main"
	}
	ghbot.ServeCron(cfg, "docs-link-checker", ghbot.SweepInterval(24), func() { sweep(client, repo, branch) })
}
