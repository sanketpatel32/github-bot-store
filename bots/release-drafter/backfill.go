// backfill.go — authoritative rebuild mode: derives every eligible merged
// PR since the previous release (or ever, for a first release) and rewrites
// the draft deterministically. Covers mid-cycle adoption, missed webhook
// deliveries and duplicate replays — the live event path only ever sees
// one merge at a time (#142).
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

// runBackfill rebuilds the draft release from merged PR history. Safe to
// run repeatedly: the output depends only on repo state, not prior drafts.
func runBackfill(client *ghbot.Client, repo string) {
	baseline, since := releaseBaseline(client, repo)

	// merged PRs since the baseline — merged_at is accurate regardless of
	// merge method (merge/squash/rebase), unlike commit-message scraping
	raw, err := client.GetAll(fmt.Sprintf(
		"/repos/%s/pulls?state=closed&sort=updated&direction=desc&base=%s", repo, defaultBranch(client, repo)))
	if err != nil {
		log.Printf("backfill: list prs: %v", err)
		return
	}
	type pr struct {
		Number   int        `json:"number"`
		Title    string     `json:"title"`
		MergedAt *time.Time `json:"merged_at"`
		User     struct {
			Login string `json:"login"`
		} `json:"user"`
	}
	var merged []pr
	for _, r := range raw {
		var p pr
		if json.Unmarshal(r, &p) != nil || p.MergedAt == nil {
			continue
		}
		if !since.IsZero() && p.MergedAt.Before(since) {
			continue
		}
		merged = append(merged, p)
	}
	if len(merged) == 0 {
		log.Printf("backfill: no merged PRs since %s — nothing to write", baseline)
		return
	}

	// deterministic: stable PR identity, sorted by number, grouped by prefix
	sort.Slice(merged, func(i, j int) bool { return merged[i].Number < merged[j].Number })
	groups := map[string][]string{}
	var order []string
	for _, p := range merged {
		g := groupOf(p.Title)
		entry := fmt.Sprintf("- %s (#%d) @%s", p.Title, p.Number, p.User.Login)
		if _, seen := groups[g]; !seen {
			order = append(order, g)
		}
		groups[g] = append(groups[g], entry)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "<!-- rebuilt by backfill from baseline %s -->\n", baseline)
	// maintainer-authored text outside sections is impossible to preserve in
	// a full rebuild — the draft is regenerated wholesale, so keep notes in
	// the dedicated section GitHub renders last.
	for _, g := range order {
		sb.WriteString("\n### " + g + "\n")
		for _, e := range groups[g] {
			sb.WriteString(e + "\n")
		}
	}

	id, _ := findDraft(client, repo)
	if id == 0 {
		_, err := client.Do("POST", fmt.Sprintf("/repos/%s/releases", repo), map[string]any{
			"tag_name": "", "name": "Next release (draft)", "body": sb.String(), "draft": true,
		})
		if err != nil {
			log.Printf("backfill: create draft: %v", err)
			return
		}
		log.Printf("backfill: created draft with %d PRs since %s", len(merged), baseline)
		return
	}
	_, err = client.Do("PATCH", fmt.Sprintf("/repos/%s/releases/%d", repo, id),
		map[string]any{"body": sb.String()})
	if err != nil {
		log.Printf("backfill: update draft: %v", err)
		return
	}
	log.Printf("backfill: rebuilt draft with %d PRs since %s", len(merged), baseline)
}

// releaseBaseline returns the latest release tag and its publish time; a
// repo with no releases gets a zero time (everything is eligible).
func releaseBaseline(client *ghbot.Client, repo string) (string, time.Time) {
	body, err := client.Do("GET", fmt.Sprintf("/repos/%s/releases/latest", repo), nil)
	if err != nil {
		return "first-release", time.Time{}
	}
	var rel struct {
		TagName     string    `json:"tag_name"`
		PublishedAt time.Time `json:"published_at"`
	}
	if json.Unmarshal(body, &rel) != nil || rel.TagName == "" {
		return "first-release", time.Time{}
	}
	return rel.TagName, rel.PublishedAt
}

func defaultBranch(client *ghbot.Client, repo string) string {
	body, err := client.Do("GET", fmt.Sprintf("/repos/%s", repo), nil)
	if err == nil {
		var r struct {
			DefaultBranch string `json:"default_branch"`
		}
		if json.Unmarshal(body, &r) == nil && r.DefaultBranch != "" {
			return r.DefaultBranch
		}
	}
	return "main"
}
