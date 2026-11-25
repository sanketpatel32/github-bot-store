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

// live tracks each bot's currently running process so shutdown can stop
// and reap every child — exiting without this leaves orphaned bots holding
// the ports a replacement supervisor needs (#119)
var live = struct {
	sync.Mutex
	procs map[string]*exec.Cmd
}{procs: map[string]*exec.Cmd{}}

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
			select {
			case <-stop:
				return
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, 60*time.Second)
			continue
		}
		log.Printf("[%s] started (pid %d)", name, cmd.Process.Pid)
		live.Lock()
		live.procs[name] = cmd
		live.Unlock()
		err := cmd.Wait()
		live.Lock()
		delete(live.procs, name)
		live.Unlock()
		select {
		case <-stop:
			return
		default:
		}
		log.Printf("[%s] exited (%v) — restarting in %s", name, err, backoff)
		select {
		case <-stop:
			return
		case <-time.After(backoff):
		}
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
	// signal every child explicitly and wait for reaping — a SIGTERM aimed
	// at the supervisor alone does not reach them (#119)
	live.Lock()
	for name, cmd := range live.procs {
		log.Printf("[%s] sending SIGTERM to pid %d", name, cmd.Process.Pid)
		cmd.Process.Signal(syscall.SIGTERM)
	}
	live.Unlock()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		live.Lock()
		remaining := len(live.procs)
		live.Unlock()
		if remaining == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	live.Lock()
	for name, cmd := range live.procs {
		log.Printf("[%s] still alive after grace period — SIGKILL", name)
		cmd.Process.Kill()
	}
	live.Unlock()
	wg.Wait()
	log.Printf("all children stopped")
	os.Exit(0)
}
