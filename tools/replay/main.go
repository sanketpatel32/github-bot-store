// replay — fires a captured webhook payload at a locally running bot, so you
// can reproduce a delivery without waiting for GitHub to send it again.
//
// Capture fixtures from GitHub's webhook delivery log (redeliver panel) and
// sanitize them first — strip tokens/emails. Then:
//
//	go run ./tools/replay -event pull_request -target localhost:8101 payload.json
//
// If WEBHOOK_SECRET is set, the request is signed exactly like GitHub would.
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func main() {
	event := flag.String("event", "", "github event type (e.g. pull_request) — required")
	target := flag.String("target", "localhost:8101", "host:port of the running bot")
	secret := flag.String("secret", os.Getenv("WEBHOOK_SECRET"), "webhook secret for signing")
	flag.Parse()
	if *event == "" || flag.NArg() == 0 {
		fmt.Println("usage: replay -event <type> [-target host:port] <payload.json>")
		os.Exit(1)
	}

	payload, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		log("read payload: %v", err)
		os.Exit(1)
	}

	url := fmt.Sprintf("http://%s/webhook", *target)
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		log("build request: %v", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", *event)
	id := make([]byte, 16)
	rand.Read(id)
	req.Header.Set("X-GitHub-Delivery", fmt.Sprintf("replay-%x", id))
	if *secret != "" {
		mac := hmac.New(sha256.New, []byte(*secret))
		mac.Write(payload)
		req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log("delivery failed: %v", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("-> %s %s: %d %s\n", *event, url, resp.StatusCode, string(body))
	if resp.StatusCode >= 300 {
		os.Exit(1)
	}
	fmt.Println("delivered — check the bot's logs")
}

func log(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "replay: "+format+"\n", args...)
}
