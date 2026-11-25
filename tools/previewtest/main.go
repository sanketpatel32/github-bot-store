// previewtest — hermetic behavior assertions: replays canned webhook events
// at real bot binaries backed by the fake GitHub API, then asserts the exact
// mutation transcript. No network, no credentials (#145).
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type scenario struct {
	Name     string
	Bot      string
	Event    string
	Contains []string
	Absent   []string
}

func main() {
	matrix := []scenario{
		{
			Name: "bug-report-issue", Bot: "keyword-issue-labeler", Event: "issues",
			Contains: []string{"POST /repos/example/demo/issues/42/labels"},
			Absent:   []string{"DELETE"},
		},
		{
			Name: "large-pr", Bot: "pr-size-labeler", Event: "pull_request",
			Contains: []string{"POST /repos/example/demo/issues/7/labels"},
			Absent:   []string{"DELETE"},
		},
	}
	failures := 0
	for i, sc := range matrix {
		fmt.Printf("scenario %s with %s... ", sc.Name, sc.Bot)
		if err := runScenario(sc, i); err != nil {
			fmt.Printf("FAIL\n  %v\n", err)
			failures++
		} else {
			fmt.Println("ok")
		}
	}
	exec.Command("pkill", "-f", "exe/fakeapi").Run()
	if failures > 0 {
		os.Exit(1)
	}
	fmt.Println("all behavior scenarios pass")
}

func runScenario(sc scenario, seq int) error {
	// unique ports per scenario — zombies from a previous run must never
	// serve the wrong fixtures or transcripts
	fakePort := fmt.Sprint(21000 + seq*10)
	botPort := fmt.Sprint(22000 + seq*10)
	fixtures := "fixtures/" + sc.Name

	fake := exec.Command("go", "run", "./tools/fakeapi")
	fake.Env = append(os.Environ(), "FIXTURES="+fixtures, "PORT="+fakePort)
	if err := fake.Start(); err != nil {
		return err
	}
	defer fake.Process.Kill()
	defer exec.Command("pkill", "-f", "exe/fakeapi").Run()

	bot := exec.Command("go", "run", "./bots/"+sc.Bot)
	bot.Env = append(os.Environ(),
		"GITHUB_TOKEN=test", "GITHUB_API_URL=http://localhost:"+fakePort,
		"ALLOW_UNSIGNED_WEBHOOKS=true", "PORT="+botPort, "REPO=example/demo",
		"GITHUB_ACTIONS=",
	)
	var botLog strings.Builder
	bot.Stdout = &botLog
	bot.Stderr = &botLog
	if err := bot.Start(); err != nil {
		return err
	}
	defer bot.Process.Kill()
	defer exec.Command("pkill", "-f", "exe/"+sc.Bot).Run()

	// poll healthz — go run compiles first, and a replay fired before the
	// listener exists vanishes silently
	ready := false
	for i := 0; i < 40; i++ {
		if r, err := http.Get("http://localhost:" + botPort + "/healthz"); err == nil {
			r.Body.Close()
			ready = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !ready {
		return fmt.Errorf("bot never became healthy\nlog: %s", botLog.String())
	}

	replay := exec.Command("go", "run", "./tools/replay",
		"-event", sc.Event, "-target", "localhost:"+botPort, fixtures+"/payload.json")
	replay.Env = append(os.Environ(), "WEBHOOK_SECRET=")
	if out, err := replay.CombinedOutput(); err != nil {
		return fmt.Errorf("replay: %v — %s", err, out)
	}
	time.Sleep(2 * time.Second)

	resp, err := http.Get("http://localhost:" + fakePort + "/transcript")
	if err != nil {
		return err
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	transcript := string(body)

	for _, want := range sc.Contains {
		if !strings.Contains(transcript, want) {
			return fmt.Errorf("transcript missing %q\ngot:\n%s\nbotlog: %s", want, transcript, botLog.String())
		}
	}
	for _, ban := range sc.Absent {
		if strings.Contains(transcript, ban) {
			return fmt.Errorf("transcript contains forbidden %q\ngot:\n%s", ban, transcript)
		}
	}
	return nil
}
