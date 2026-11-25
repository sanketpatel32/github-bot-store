// flaky-test-tracker — spots flaky CI: a check run that fails and then
// passes on the same SHA without code changes is a flake, not a fix. After
// enough flakes from the same check, it opens a tracking issue.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

const marker = "<!-- flaky-test-tracker -->"

type checkEvent struct {
	Action   string `json:"action"`
	CheckRun struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		HeadSHA    string `json:"head_sha"`
		DetailsURL string `json:"details_url"`
	} `json:"check_run"`
	Repo struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

// state tracks the last conclusion per (check name, SHA) and flake counts
// per check name. Persisted to flaky-state.json like stale-bot's state.
type state struct {
	mu       sync.Mutex
	LastSeen map[string]lastSeen `json:"last_seen"`
	Flakes   map[string]int      `json:"flakes"`
}

type lastSeen struct {
	SHA        string    `json:"sha"`
	Conclusion string    `json:"conclusion"`
	At         time.Time `json:"at"`
}

func loadState() *state {
	st := &state{LastSeen: map[string]lastSeen{}, Flakes: map[string]int{}}
	data, err := os.ReadFile("flaky-state.json")
	if err == nil {
		json.Unmarshal(data, st)
	}
	if st.LastSeen == nil {
		st.LastSeen = map[string]lastSeen{}
	}
	if st.Flakes == nil {
		st.Flakes = map[string]int{}
	}
	return st
}

func (st *state) save() {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile("flaky-state.json", data, 0o644)
}

func main() {
	cfg := ghbot.LoadConfig("flaky-test-tracker", "8129")
	client := ghbot.NewClient(cfg.Token)
	st := loadState()

	threshold := 3
	if v := os.Getenv("FLAKE_THRESHOLD"); v != "" {
		fmt.Sscanf(v, "%d", &threshold)
	}

	ghbot.Serve(cfg, "flaky-test-tracker", func(event string, payload []byte) {
		if event != "check_run" {
			return
		}
		var ev checkEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		if ev.Action != "completed" || ev.CheckRun.Conclusion == "" {
			return
		}

		key := ev.CheckRun.Name
		st.mu.Lock()
		prev, seen := st.LastSeen[key]
		isFlake := seen && prev.SHA == ev.CheckRun.HeadSHA &&
			prev.Conclusion == "failure" && ev.CheckRun.Conclusion == "success"
		st.LastSeen[key] = lastSeen{
			SHA:        ev.CheckRun.HeadSHA,
			Conclusion: ev.CheckRun.Conclusion,
			At:         time.Now(),
		}
		if isFlake {
			st.Flakes[key]++
		}
		count := st.Flakes[key]
		st.save()
		st.mu.Unlock()

		if !isFlake {
			return
		}
		log.Printf("flake detected: %s passed after failing on same SHA (total %d)", key, count)

		if count < threshold {
			return
		}
		// already have an open issue for this check?
		search, err := client.Do("GET", fmt.Sprintf(
			"/search/issues?q=repo:%s+type:issue+state:open+in:body+flaky:%s&per_page=1",
			ev.Repo.FullName, key), nil)
		if err == nil {
			var result struct {
				Items []struct{} `json:"items"`
			}
			if json.Unmarshal(search, &result) == nil && len(result.Items) > 0 {
				return
			}
		}
		title := fmt.Sprintf("Flaky check: %s (%d flakes)", key, count)
		body := fmt.Sprintf(`%s
The check **%s** has flaked **%d times** — failing then passing on the *same commit* with no code changes.

Latest example: %s

A test that passes on retry without code changes is unreliable signal.
Consider: quarantining it, fixing the nondeterminism, or increasing timeouts.

<!-- flaky:%s -->`,
			marker, key, count, ev.CheckRun.DetailsURL, key)
		_, err = client.Do("POST", fmt.Sprintf("/repos/%s/issues", ev.Repo.FullName),
			map[string]any{"title": title, "body": body, "labels": []string{"flaky-test"}})
		if err != nil {
			if strings.Contains(err.Error(), "422") {
				client.EnsureLabel(ev.Repo.FullName, "flaky-test", "d29922", "Flaky CI check")
				client.Do("POST", fmt.Sprintf("/repos/%s/issues", ev.Repo.FullName),
					map[string]any{"title": title, "body": body, "labels": []string{"flaky-test"}})
			}
			return
		}
		log.Printf("opened flaky-test issue for %s", key)
	})
}
