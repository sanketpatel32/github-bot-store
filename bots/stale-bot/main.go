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
	"sync"
	"time"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

type issue struct {
	Number    int       `json:"number"`
	UpdatedAt time.Time `json:"updated_at"`
	Labels    []struct {
		Name string `json:"name"`
	} `json:"labels"`
	PullRequest *struct{} `json:"pull_request"`
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

func sweep(client *ghbot.Client, repo string, st *state, staleDays, closeDays int) {
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
		markedAt, isMarked := st.Marked[i.Number]

		if isMarked {
			// genuine activity since we marked it? (our marking comment lands
			// within seconds of markedAt; anything comfortably after is real)
			if i.UpdatedAt.After(markedAt.Add(2 * time.Minute)) {
				client.Do("DELETE", fmt.Sprintf("/repos/%s/issues/%d/labels/stale", repo, i.Number), nil)
				delete(st.Marked, i.Number)
				st.save()
				log.Printf("un-staled %s #%d (activity)", kind, i.Number)
				continue
			}
			if now.Sub(markedAt) >= time.Duration(closeDays)*24*time.Hour {
				client.Comment(repo, i.Number,
					"Closing due to inactivity. Happy to reopen if there's interest — just comment.")
				client.Do("PATCH", fmt.Sprintf("/repos/%s/issues/%d", repo, i.Number),
					map[string]string{"state": "closed"})
				delete(st.Marked, i.Number)
				st.save()
				log.Printf("closed %s #%d", kind, i.Number)
			}
			continue
		}

		// not marked — is it inactive long enough? (client-side filter; the
		// API `since` param selects *recently updated* items, the opposite)
		if hasLabel(i, "pinned") || hasLabel(i, "stale") {
			if hasLabel(i, "stale") {
				// labeled stale externally — adopt it into state
				st.Marked[i.Number] = i.UpdatedAt
				st.save()
			}
			continue
		}
		if i.UpdatedAt.Before(staleCutoff) {
			client.EnsureLabel(repo, "stale", "ededed", "Inactive — will close without activity")
			client.AddLabels(repo, i.Number, []string{"stale"})
			client.Comment(repo, i.Number, fmt.Sprintf(
				"This %s has been inactive for over %d days and is now marked stale. "+
					"It will close in %d days unless there's new activity — any comment resets the clock.",
				kind, staleDays, closeDays))
			st.Marked[i.Number] = now
			st.save()
			log.Printf("marked %s #%d stale", kind, i.Number)
		}
	}
}

func main() {
	cfg := ghbot.LoadConfig("stale-bot", "8103")
	client := ghbot.NewClient(cfg.Token)

	repo := os.Getenv("REPO") // owner/name — required for cron bots
	if repo == "" {
		log.Fatal("REPO (owner/name) env var is required")
	}
	staleDays := envInt("STALE_DAYS", 30)
	closeDays := envInt("CLOSE_AFTER_DAYS", 7)
	intervalHours := envInt("SWEEP_INTERVAL_HOURS", 12)
	st := loadState("stale-state.json")

	go func() {
		for {
			log.Printf("sweeping %s (stale>%dd, close after %dd marked)", repo, staleDays, closeDays)
			sweep(client, repo, st, staleDays, closeDays)
			time.Sleep(time.Duration(intervalHours) * time.Hour)
		}
	}()

	// no webhooks needed — just the health endpoint
	ghbot.ServeHealth(cfg, "stale-bot")
}
