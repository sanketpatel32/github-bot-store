// botctl — lightweight management utility for a host running several bots:
// list what's running, probe health endpoints, and restart supervision
// checks without digging through process lists.
//
//	botctl list            — running bot processes
//	botctl probe           — curl /healthz + /metrics summary per known port
//	botctl ports           — the port registry from manifest.json
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: botctl <list|probe|ports>")
		os.Exit(1)
	}
	switch os.Args[1] {
	case "list":
		out, _ := exec.Command("sh", "-c", "ps aux | grep -E 'bots?-?[a-z-]*' | grep -v grep").Output()
		if len(out) == 0 {
			fmt.Println("no bot processes found")
			return
		}
		fmt.Print(string(out))
	case "probe":
		probe()
	case "ports":
		ports()
	default:
		fmt.Println("unknown command:", os.Args[1])
		os.Exit(1)
	}
}

func ports() []struct {
	Name string `json:"name"`
	Port int    `json:"port"`
} {
	var manifest []struct {
		Name string `json:"name"`
		Port int    `json:"port"`
	}
	data, err := os.ReadFile("manifest.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: manifest.json not found in %s — no probe targets (run from the repo root)\n", mustGetwd())
	} else if err := json.Unmarshal(data, &manifest); err != nil {
		fmt.Fprintf(os.Stderr, "warning: manifest.json is invalid: %v\n", err)
	}
	if len(os.Args) > 1 && os.Args[1] == "ports" {
		for _, b := range manifest {
			fmt.Printf("%-28s :%d\n", b.Name, b.Port)
		}
	}
	return manifest
}

func mustGetwd() string {
	d, err := os.Getwd()
	if err != nil {
		return "unknown directory"
	}
	return d
}

func probe() {
	hc := &http.Client{Timeout: 2 * time.Second}
	up, down := 0, 0
	targets := ports()
	if len(targets) == 0 {
		fmt.Println("no bots discovered — nothing to probe")
		os.Exit(2)
	}
	for _, b := range targets {
		url := fmt.Sprintf("http://localhost:%d/healthz", b.Port)
		resp, err := hc.Get(url)
		if err != nil {
			fmt.Printf("✗ %-28s :%d — down\n", b.Name, b.Port)
			down++
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		fmt.Printf("✓ %-28s :%d — healthy\n", b.Name, b.Port)
		up++
		// pull event counters if metrics enabled
		mresp, err := hc.Get(fmt.Sprintf("http://localhost:%d/metrics", b.Port))
		if err == nil {
			body, _ := io.ReadAll(mresp.Body)
			mresp.Body.Close()
			for _, line := range strings.Split(string(body), "\n") {
				if strings.HasPrefix(line, "ghbot_events_total") && !strings.HasPrefix(line, "#") {
					fmt.Printf("    %s\n", line)
				}
			}
		}
	}
	fmt.Printf("\n%d up, %d down\n", up, down)
	if down > 0 {
		os.Exit(1)
	}
}
