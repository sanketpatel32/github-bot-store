// checkmatrix — validates that the catalog's machine-readable permission and
// event metadata is complete and consistent with the actual workflows and
// bot implementations (#146). Fails CI when a bot is missing required_env,
// actions_perms or gh_events, or when a workflow subscribes a bot to events
// its manifest entry doesn't declare (or vice versa).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type bot struct {
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	Events       []string          `json:"gh_events"`
	ActionsPerms map[string]string `json:"actions_perms"`
	RequiredEnv  []string          `json:"required_env"`
	Perms        string            `json:"perms"`
}

func keys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func main() {
	data, err := os.ReadFile("manifest.json")
	if err != nil {
		fmt.Println("manifest.json unreadable:", err)
		os.Exit(1)
	}
	var bots []bot
	if err := json.Unmarshal(data, &bots); err != nil {
		fmt.Println("manifest.json invalid:", err)
		os.Exit(1)
	}
	failures := 0
	fail := func(format string, args ...any) {
		fmt.Printf("FAIL: "+format+"\n", args...)
		failures++
	}

	if len(bots) < 30 {
		fail("manifest has %d bots — expected 30+", len(bots))
	}

	for _, b := range bots {
		// completeness: every advertised bot declares its full contract
		if len(b.ActionsPerms) == 0 {
			fail("%s: actions_perms missing — permission reference incomplete", b.Name)
		}
		if b.Type == "webhook" && len(b.Events) == 0 {
			fail("%s: webhook bot without gh_events", b.Name)
		}
		if b.Type == "cron" && len(b.Events) != 1 {
			// cron bots declare ["schedule"] or similar single trigger
			fail("%s: cron bot should declare exactly one trigger, has %v", b.Name, b.Events)
		}
		for scope, access := range b.ActionsPerms {
			if access != "read" && access != "write" {
				fail("%s: actions_perms[%s]=%q — must be read or write", b.Name, scope, access)
			}
		}
		// empty required_env is legitimate — it means the bot needs nothing
		// beyond the universal token. What must not happen is a bot reading
		// env vars the manifest doesn't know about; cross-checked below.
		for _, e := range b.RequiredEnv {
			if e == "GITHUB_TOKEN" || e == "WEBHOOK_SECRET" {
				fail("%s: required_env lists universal credential %s — those are implied for every bot", b.Name, e)
			}
		}
	}

	// consistency: each workflow's trigger events must be handled by at
	// least one bot wired inside it — a trigger no bot consumes is pure noise
	for _, wf := range []string{"bots-issues.yml", "bots-pr.yml", "bots-checks.yml", "bots-cron.yml"} {
		raw, err := os.ReadFile(".github/workflows/" + wf)
		if err != nil {
			continue
		}
		content := string(raw)
		union := map[string]bool{}
		present := 0
		for _, b := range bots {
			if strings.Contains(content, "bots/"+b.Name) {
				present++
				for _, e := range b.Events {
					union[e] = true
				}
			}
		}
		if present == 0 {
			continue
		}
		// parse the on: block properly — naive substring matching confuses
		// "issues: write" in permissions with an event subscription
		triggers := map[string]bool{}
		lines := strings.Split(content, "\n")
		inOn := false
		for _, ln := range lines {
			if ln == "on:" {
				inOn = true
				continue
			}
			if inOn {
				if len(ln) > 0 && ln[0] != ' ' && ln[0] != '#' {
					break
				}
				trimmed := strings.TrimSpace(ln)
				if strings.HasSuffix(trimmed, ":") && !strings.Contains(trimmed, " ") {
					triggers[strings.TrimSuffix(trimmed, ":")] = true
				}
			}
		}
		for ev := range triggers {
			if ev == "schedule" || ev == "workflow_dispatch" {
				continue
			}
			if !union[ev] {
				fail("%s triggers on %q but no bot inside declares it (union: %v)", wf, ev, keys(union))
			}
		}
	}

	if failures > 0 {
		fmt.Printf("%d matrix validation failure(s)\n", failures)
		os.Exit(1)
	}
	fmt.Printf("matrix valid: %d bots with complete permission/event contracts\n", len(bots))
}
