// discussion-triage-bot — cron bot sweeping repo discussions: flags
// unanswered questions older than a grace period so they don't rot, and
// celebrates answered ones. Uses the GraphQL API (discussions aren't in REST).
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

type discussion struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"createdAt"`
	Answered  bool      `json:"isAnswered"`
	Comments  struct {
		TotalCount int `json:"totalCount"`
	} `json:"comments"`
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
}

const query = `query($owner:String!, $repo:String!) {
  repository(owner:$owner, name:$repo) {
    discussions(first:100, orderBy:{field:CREATED_AT, direction:DESC}) {
      nodes {
        number
        title
        createdAt
        isAnswered
        comments { totalCount }
        author { login }
      }
    }
  }
}`

func sweep(client *ghbot.Client, repo string, graceDays int) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		log.Printf("REPO must be owner/repo, got %q", repo)
		return
	}
	data, err := client.GraphQL(query, map[string]any{"owner": parts[0], "repo": parts[1]})
	if err != nil {
		log.Printf("graphql: %v", err)
		return
	}
	var result struct {
		Repository struct {
			Discussions struct {
				Nodes []discussion `json:"nodes"`
			} `json:"discussions"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		log.Printf("parse: %v", err)
		return
	}

	now := time.Now()
	stale := 0
	for _, d := range result.Repository.Discussions.Nodes {
		unanswered := !d.Answered && d.Comments.TotalCount == 0
		old := now.Sub(d.CreatedAt) > time.Duration(graceDays)*24*time.Hour
		if unanswered && old {
			stale++
			log.Printf("discussion #%d %q unanswered for %d+ days", d.Number, d.Title, graceDays)
		}
	}

	// post/update one tracking issue rather than touching each discussion —
	// maintainers want a triage list, not noise in every thread
	title := fmt.Sprintf("Unanswered discussions: %d need attention", stale)
	marker := "<!-- discussion-triage -->"
	var sb strings.Builder
	sb.WriteString(marker + "\n")
	if stale == 0 {
		sb.WriteString("✅ No stale unanswered discussions.\n")
	} else {
		fmt.Fprintf(&sb, "**%d discussions** have no replies after %d days:\n\n", stale, graceDays)
		for _, d := range result.Repository.Discussions.Nodes {
			if !d.Answered && d.Comments.TotalCount == 0 &&
				now.Sub(d.CreatedAt) > time.Duration(graceDays)*24*time.Hour {
				fmt.Fprintf(&sb, "- #%d %s (by @%s)\n", d.Number, d.Title, d.Author.Login)
			}
		}
		sb.WriteString("\n_This issue updates itself on each sweep._")
	}

	// upsert managed issue via REST
	existing := findManaged(client, repo, marker)
	if stale == 0 {
		if existing != 0 {
			client.Comment(repo, existing, "All caught up — closing.")
			client.Do("PATCH", fmt.Sprintf("/repos/%s/issues/%d", repo, existing),
				map[string]string{"state": "closed"})
		}
		return
	}
	if existing != 0 {
		client.Do("PATCH", fmt.Sprintf("/repos/%s/issues/%d", repo, existing),
			map[string]string{"title": title, "body": sb.String()})
	} else {
		client.Do("POST", fmt.Sprintf("/repos/%s/issues", repo),
			map[string]any{"title": title, "body": sb.String(), "labels": []string{"discussions"}})
	}
}

func findManaged(client *ghbot.Client, repo, marker string) int {
	body, err := client.Do("GET", fmt.Sprintf(
		"/search/issues?q=repo:%s+type:issue+state:open+in:body+%s&per_page=1", repo, marker), nil)
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
	cfg := ghbot.LoadConfig("discussion-triage-bot", "8131")
	if ghbot.HandleCLIArgs("discussion-triage-bot", cfg) {
		return
	}
	client := ghbot.NewClient(cfg.Token)
	repo := os.Getenv("REPO")
	if repo == "" {
		log.Fatal("REPO is not set (owner/repo)")
	}
	grace := 3
	if v := os.Getenv("GRACE_DAYS"); v != "" {
		fmt.Sscanf(v, "%d", &grace)
	}
	ghbot.ServeCron(cfg, "discussion-triage-bot", ghbot.SweepInterval(24), func() {
		sweep(client, repo, grace)
	})
}
