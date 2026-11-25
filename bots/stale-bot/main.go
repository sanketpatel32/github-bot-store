// stale-bot — marks issues/PRs with no activity for STALE_DAYS as stale,
// then closes them CLOSE_DAYS later if nothing happens. Any genuine
// contributor activity removes the stale label and resets the clock.
//
// Cron-style: polls on an interval instead of reacting to webhooks.
// State (when each item was marked stale) persists in stale-state.json.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

// envIntValidated parses a positive integer env var. Zero, negative, or
// malformed values are fatal with a helpful message — silently falling back
// would hide configuration mistakes that change destructive behavior.
func envIntValidated(key string, fallback, min, max int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Fatalf("%s=%q is not a number — expected an integer between %d and %d", key, v, min, max)
	}
	if n < min || n > max {
		log.Fatalf("%s=%d out of range — expected between %d and %d", key, n, min, max)
	}
	return n
}

type issue struct {
	Number    int       `json:"number"`
	UpdatedAt time.Time `json:"updated_at"`
	Draft     bool      `json:"draft"`
	Labels    []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Assignees   []struct{} `json:"assignees"`
	Milestone   *struct{}  `json:"milestone"`
	PullRequest *struct{}  `json:"pull_request"`
}

func hasLabel(i issue, name string) bool {
	for _, l := range i.Labels {
		if l.Name == name {
			return true
		}
	}
	return false
}

// state tracks when each issue/PR number was marked stale, so close timing
// and activity detection don't depend on updated_at alone (which our own
// bot comments also bump).
type state struct {
	mu     sync.Mutex
	Marked map[int]time.Time `json:"marked"`
	path   string
}

func loadState(path string) *state {
	s := &state{Marked: map[int]time.Time{}, path: path}
	if data, err := os.ReadFile(path); err == nil {
		var raw map[string]string
		if json.Unmarshal(data, &raw) == nil {
			for k, v := range raw {
				var num int
				if _, err := fmt.Sscanf(k, "%d", &num); err == nil {
					if t, err := time.Parse(time.RFC3339, v); err == nil {
						s.Marked[num] = t
					}
				}
			}
		}
	}
	return s
}

func (s *state) save() {
	raw := map[string]string{}
	for num, t := range s.Marked {
		raw[fmt.Sprint(num)] = t.UTC().Format(time.RFC3339)
	}
	if data, err := json.Marshal(raw); err == nil {
		os.WriteFile(s.path, data, 0o600)
	}
}

type options struct {
	exemptLabels    map[string]bool
	exemptAssigned  bool
	exemptMilestone bool
	exemptDraftPRs  bool
	dryRun          bool
}

func loadOptions() options {
	o := options{
		exemptLabels:    map[string]bool{},
		exemptAssigned:  os.Getenv("EXEMPT_ASSIGNED") == "true",
		exemptMilestone: os.Getenv("EXEMPT_MILESTONE") == "true",
		exemptDraftPRs:  os.Getenv("EXEMPT_DRAFT_PRS") != "false", // drafts exempt by default
		dryRun:          os.Getenv("DRY_RUN") == "true",
	}
	for _, l := range strings.Split(os.Getenv("EXEMPT_LABELS"), ",") {
		if l = strings.TrimSpace(l); l != "" {
			o.exemptLabels[l] = true
		}
	}
	o.exemptLabels["pinned"] = true // pinned is always exempt
	return o
}

func sweep(client *ghbot.Client, repo string, st *state, staleDays, closeDays int, opts options) {
	items, err := client.GetAll(fmt.Sprintf("/repos/%s/issues?state=open", repo))
	if err != nil {
		log.Printf("list issues: %v", err)
		return
	}

	now := time.Now()
	staleCutoff := now.Add(-time.Duration(staleDays) * 24 * time.Hour)

	st.mu.Lock()
	defer st.mu.Unlock()

	for _, raw := range items {
		var i issue
		if json.Unmarshal(raw, &i) != nil {
			continue
		}
		kind := "issue"
		if i.PullRequest != nil {
			kind = "pull request"
		}
		exempt := false
		for _, l := range i.Labels {
			if opts.exemptLabels[l.Name] {
				exempt = true
			}
		}
		if opts.exemptAssigned && len(i.Assignees) > 0 {
			exempt = true
		}
		if opts.exemptMilestone && i.Milestone != nil {
			exempt = true
		}
		if opts.exemptDraftPRs && i.PullRequest != nil && i.Draft {
			exempt = true
		}

		markedAt, isMarked := st.Marked[i.Number]

		if isMarked {
			// policy may have changed since marking — a newly configured
			// exemption must protect already-marked items, not just future
			// ones (#121)
			if exempt {
				if opts.dryRun {
					log.Printf("[dry-run] would un-stale %s #%d (now exempt)", kind, i.Number)
					continue
				}
				client.Do("DELETE", fmt.Sprintf("/repos/%s/issues/%d/labels/stale", repo, i.Number), nil)
				delete(st.Marked, i.Number)
				st.save()
				log.Printf("un-staled %s #%d (newly exempt)", kind, i.Number)
				continue
			}
			// genuine activity since we marked it? markedAt is recorded after
			// the marking comment completes, so only a few seconds of slack
			// are needed — a human reply 90 seconds later is real activity
			// and must reset the clock (#121)
			if i.UpdatedAt.After(markedAt.Add(5 * time.Second)) {
				if opts.dryRun {
					log.Printf("[dry-run] would un-stale %s #%d", kind, i.Number)
					continue
				}
				client.Do("DELETE", fmt.Sprintf("/repos/%s/issues/%d/labels/stale", repo, i.Number), nil)
				delete(st.Marked, i.Number)
				st.save()
				log.Printf("un-staled %s #%d (activity)", kind, i.Number)
				continue
			}
			if now.Sub(markedAt) >= time.Duration(closeDays)*24*time.Hour {
				if opts.dryRun {
					log.Printf("[dry-run] would close %s #%d", kind, i.Number)
					continue
				}
				client.Comment(repo, i.Number,
					"Closing due to inactivity. Happy to reopen if there's interest — just comment.")
				client.Do("PATCH", fmt.Sprintf("/repos/%s/issues/%d", repo, i.Number),
					map[string]string{"state": "closed"})
				delete(st.Marked, i.Number)
				st.save()
				log.Printf("closed %s #%d", kind, i.Number)
				ghbot.Notify("stale-bot", fmt.Sprintf("closed stale issue #%d in %s", i.Number, repo))
			}
			continue
		}

		if exempt {
			continue
		}

		// not marked — is it inactive long enough? (client-side filter; the
		// API `since` param selects *recently updated* items, the opposite)
		if hasLabel(i, "stale") {
			// labeled stale externally — adopt it into state
			st.Marked[i.Number] = i.UpdatedAt
			st.save()
			continue
		}
		if i.UpdatedAt.Before(staleCutoff) {
			if opts.dryRun {
				log.Printf("[dry-run] would mark %s #%d stale", kind, i.Number)
				continue
			}
			client.EnsureLabel(repo, "stale", "ededed", "Inactive — will close without activity")
			client.AddLabels(repo, i.Number, []string{"stale"})
			client.Comment(repo, i.Number, fmt.Sprintf(
				"This %s has been inactive for over %d days and is now marked stale. "+
					"It will close in %d days unless there's new activity — any comment resets the clock.",
				kind, staleDays, closeDays))
			// record the mark only after the comment lands — everything after
			// this timestamp is genuinely new activity, no guessing window
			st.Marked[i.Number] = time.Now()
			st.save()
			log.Printf("marked %s #%d stale", kind, i.Number)
		}
	}
}

func main() {
	cfg := ghbot.LoadConfig("stale-bot", "8103")
	if ghbot.HandleCLIArgs("stale-bot", cfg) {
		return
	}
	client := ghbot.NewClient(cfg.Token)

	repo := os.Getenv("REPO") // owner/name — required for cron bots
	if repo == "" {
		log.Fatal("REPO (owner/name) env var is required")
	}
	staleDays := envIntValidated("STALE_DAYS", 30, 1, 365)
	closeDays := envIntValidated("CLOSE_AFTER_DAYS", 7, 1, 90)
	intervalHours := envIntValidated("SWEEP_INTERVAL_HOURS", 12, 1, 168)
	opts := loadOptions()
	if opts.dryRun {
		log.Printf("DRY_RUN enabled — logging actions without writing to GitHub")
	}
	st := loadState("stale-state.json")

	if os.Getenv("GITHUB_ACTIONS") == "true" {
		log.Printf("sweeping %s once (actions mode)", repo)
		sweep(client, repo, st, staleDays, closeDays, opts)
		return
	}

	go func() {
		for {
			log.Printf("sweeping %s (stale>%dd, close after %dd marked)", repo, staleDays, closeDays)
			sweep(client, repo, st, staleDays, closeDays, opts)
			time.Sleep(time.Duration(intervalHours) * time.Hour)
		}
	}()

	// no webhooks needed — just the health endpoint
	ghbot.ServeHealth(cfg, "stale-bot")
}
