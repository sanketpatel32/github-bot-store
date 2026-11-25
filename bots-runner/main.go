// bots-runner — optional all-in-one supervisor: runs several bots as child
// processes in one command, restarts them on crash with backoff, and pipes
// their logs through with a prefix. For people who'd rather manage one
// process than fifteen.
//
// Usage: bots-runner stale-bot pr-size-labeler auto-merger
// (binaries must be in PATH or same directory; config via env as usual)
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

type child struct {
	name     string
	cmd      *exec.Cmd
	restarts int
}

func resolveBinary(name string) string {
	// same directory as runner first, then PATH
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return name // fall back to PATH lookup
}

func supervise(name string, wg *sync.WaitGroup, stop <-chan struct{}) {
	defer wg.Done()
	backoff := time.Second
	for {
		select {
		case <-stop:
			return
		default:
		}
		cmd := exec.Command(resolveBinary(name))
		cmd.Stdout = &prefixWriter{prefix: "[" + name + "] ", w: os.Stdout}
		cmd.Stderr = &prefixWriter{prefix: "[" + name + "] ", w: os.Stderr}
		cmd.Env = os.Environ()
		if err := cmd.Start(); err != nil {
			log.Printf("[%s] failed to start: %v", name, err)
			time.Sleep(backoff)
			backoff = min(backoff*2, 60*time.Second)
			continue
		}
		log.Printf("[%s] started (pid %d)", name, cmd.Process.Pid)
		err := cmd.Wait()
		select {
		case <-stop:
			return
		default:
		}
		log.Printf("[%s] exited (%v) — restarting in %s", name, err, backoff)
		time.Sleep(backoff)
		backoff = min(backoff*2, 60*time.Second)
	}
}

type prefixWriter struct {
	prefix string
	w      *os.File
}

func (p *prefixWriter) Write(b []byte) (int, error) {
	lines := strings.Split(string(b), "\n")
	for i, line := range lines {
		if line == "" && i == len(lines)-1 {
			continue
		}
		fmt.Fprintf(p.w, "%s%s\n", p.prefix, line)
	}
	return len(b), nil
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: bots-runner <bot> [<bot>...]\nexample: bots-runner stale-bot pr-size-labeler auto-merger")
		os.Exit(1)
	}
	names := os.Args[1:]
	log.Printf("supervising %d bots: %s", len(names), strings.Join(names, ", "))

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for _, name := range names {
		wg.Add(1)
		go supervise(name, &wg, stop)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Printf("shutting down — stopping children")
	close(stop)
	// children receive SIGINT via process group; give them a moment
	time.Sleep(500 * time.Millisecond)
	os.Exit(0)
}
