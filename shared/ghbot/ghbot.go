// Package ghbot provides the shared plumbing every bot in the store uses:
// webhook signature verification, a tiny GitHub REST client, and a
// one-line HTTP server bootstrap. Standard library only.
package ghbot

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---- bounded worker pool (#11) ----

const workerCount = 8
const queueDepth = 64

var jobQueue = make(chan func(), queueDepth)
var workersOnce sync.Once

func startWorkers() {
	for i := 0; i < workerCount; i++ {
		go func() {
			for job := range jobQueue {
				job()
			}
		}()
	}
}

// dispatch queues a job; returns false when the queue is saturated.
func dispatch(job func()) bool {
	workersOnce.Do(startWorkers)
	select {
	case jobQueue <- job:
		return true
	default:
		return false
	}
}

// ---- delivery dedupe (#10) ----

const dedupeRetention = 4096

var dedupe = struct {
	sync.Mutex
	ids   map[string]bool
	order []string
}{ids: map[string]bool{}}

// seenDelivery records a delivery ID; true if it was already seen.
func seenDelivery(id string) bool {
	if id == "" {
		return false
	}
	dedupe.Lock()
	defer dedupe.Unlock()
	if dedupe.ids[id] {
		return true
	}
	dedupe.ids[id] = true
	dedupe.order = append(dedupe.order, id)
	if len(dedupe.order) > dedupeRetention {
		delete(dedupe.ids, dedupe.order[0])
		dedupe.order = dedupe.order[1:]
	}
	return false
}

// newServer builds an http.Server with production timeouts — slow-client
// and slowloris resistance out of the box.
func newServer(port string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

// Config holds the environment configuration all bots share.
type Config struct {
	Token  string
	Secret string
	Port   string
}

// LoadConfig reads env vars and exits with a helpful message if the token is missing.
// defaultPort is used when PORT is not set.
func LoadConfig(botName, defaultPort string) Config {
	cfg := Config{
		Token:  os.Getenv("GITHUB_TOKEN"),
		Secret: os.Getenv("WEBHOOK_SECRET"),
		Port:   os.Getenv("PORT"),
	}
	if cfg.Token == "" {
		log.Fatalf("[%s] GITHUB_TOKEN is not set — copy .env.example to .env", botName)
	}
	if cfg.Port == "" {
		cfg.Port = defaultPort
	}
	return cfg
}

// VerifySignature checks the X-Hub-Signature-256 header against the body.
// If no secret is configured it returns true (local testing convenience).
func VerifySignature(secret string, body []byte, sigHeader string) bool {
	if secret == "" {
		return true
	}
	if len(sigHeader) < 7 || sigHeader[:7] != "sha256=" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sigHeader))
}

// WebhookHandler is a function that receives the GitHub event type
// (e.g. "pull_request") and the raw JSON payload.
type WebhookHandler func(event string, payload []byte)

// ServeHealth runs only the health endpoint — for cron-style bots that
// don't consume webhooks, so there's nothing to sign.
func ServeHealth(cfg Config, name string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	log.Printf("[%s] health-only server listening on :%s", name, cfg.Port)
	log.Fatal(newServer(cfg.Port, mux).ListenAndServe())
}

// RunAction executes the handler once using the GitHub Actions event
// payload (GITHUB_EVENT_PATH / GITHUB_EVENT_NAME) instead of a webhook
// delivery — this is how bots self-host on a repo with zero servers.
// Returns true when running inside Actions.
func RunAction(name string, handler WebhookHandler) bool {
	if os.Getenv("GITHUB_ACTIONS") != "true" {
		return false
	}
	eventPath := os.Getenv("GITHUB_EVENT_PATH")
	eventName := os.Getenv("GITHUB_EVENT_NAME")
	data, err := os.ReadFile(eventPath)
	if err != nil {
		log.Printf("[%s] no actions event payload at %s: %v", name, eventPath, err)
		return true
	}
	log.Printf("[%s] running as GitHub Action for event %q", name, eventName)
	handler(eventName, data)
	return true
}

// Serve starts the bot's HTTP server: POST /webhook plus GET /healthz.
// A nonempty WEBHOOK_SECRET is required in production — unsigned webhooks
// mean anyone who finds your port can trigger bot actions. Set
// ALLOW_UNSIGNED_WEBHOOKS=true for local testing only.
func Serve(cfg Config, name string, handler WebhookHandler) {
	if RunAction(name, handler) {
		return
	}
	if cfg.Secret == "" && os.Getenv("ALLOW_UNSIGNED_WEBHOOKS") != "true" {
		log.Fatalf("[%s] WEBHOOK_SECRET is not set. Configure the same secret in "+
			"GitHub webhook settings, or set ALLOW_UNSIGNED_WEBHOOKS=true for local testing only.", name)
	}
	if cfg.Secret == "" {
		log.Printf("[%s] WARNING: accepting unsigned webhooks (ALLOW_UNSIGNED_WEBHOOKS) — do not expose this port", name)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.ContentLength > 1<<20 {
			http.Error(w, "payload too large (1MB max)", http.StatusRequestEntityTooLarge)
			return
		}
		if r.Header.Get("X-GitHub-Event") == "" || r.Header.Get("X-GitHub-Delivery") == "" {
			http.Error(w, "missing github event headers", http.StatusBadRequest)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20+1))
		if err != nil || int64(len(body)) > 1<<20 {
			http.Error(w, "payload too large or unreadable", http.StatusRequestEntityTooLarge)
			return
		}
		if !VerifySignature(cfg.Secret, body, r.Header.Get("X-Hub-Signature-256")) {
			http.Error(w, "bad signature", http.StatusUnauthorized)
			return
		}
		event := r.Header.Get("X-GitHub-Event")
		deliveryID := r.Header.Get("X-GitHub-Delivery")

		// feedback-loop prevention (#70): bots reacting to bot-authored
		// events can loop forever (bot comments -> triggers another bot ->
		// comments again). Skip events whose sender is a GitHub App/bot,
		// unless SKIP_BOT_EVENTS=false is explicitly set.
		if os.Getenv("SKIP_BOT_EVENTS") != "false" {
			var meta struct {
				Sender struct {
					Type  string `json:"type"`
					Login string `json:"login"`
				} `json:"sender"`
			}
			if json.Unmarshal(body, &meta) == nil &&
				(meta.Sender.Type == "Bot" || strings.HasSuffix(meta.Sender.Login, "[bot]")) {
				log.Printf("[%s] skipping bot-authored %s event from %s", name, event, meta.Sender.Login)
				w.WriteHeader(http.StatusAccepted)
				return
			}
		}

		// dedupe redeliveries — GitHub retries on timeouts and users click
		// "redeliver"; without this every retry repeats the bot's actions
		if seenDelivery(deliveryID) {
			log.Printf("[%s] duplicate delivery %s — skipping", name, deliveryID)
			w.WriteHeader(http.StatusAccepted)
			return
		}

		job := func() {
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("[%s] panic handling %s: %v", name, event, rec)
				}
			}()
			handler(event, body)
		}
		if !dispatch(job) {
			// workers saturated — 429 tells GitHub to retry later instead of
			// dropping the event silently
			http.Error(w, "busy — retry", http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	log.Printf("[%s] listening on :%s", name, cfg.Port)
	log.Fatal(newServer(cfg.Port, mux).ListenAndServe())
}

// Client is a minimal authenticated GitHub REST client.
type Client struct {
	Token string
	hc    *http.Client
}

// NewClient builds a client from a token.
func NewClient(token string) *Client {
	return &Client{Token: token, hc: &http.Client{Timeout: 20 * time.Second}}
}

// SetHTTPClient overrides the underlying HTTP client — used by tests to
// redirect API calls to a local server.
func (c *Client) SetHTTPClient(hc *http.Client) {
	c.hc = hc
}

// Do performs an API call. path is relative, e.g. "/repos/owner/repo/issues/1/comments".
// payload may be nil. Returns the response body.
func (c *Client) Do(method, path string, payload any) ([]byte, error) {
	var reader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, "https://api.github.com"+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	var resp *http.Response
	for attempt := 0; attempt < 4; attempt++ {
		resp, err = c.hc.Do(req)
		if err != nil {
			if attempt == 3 {
				return nil, err
			}
			time.Sleep(time.Duration(1<<attempt) * time.Second)
			continue
		}
		// retry on server errors and secondary rate limits
		if resp.StatusCode == 502 || resp.StatusCode == 503 ||
			(resp.StatusCode == 429) || (resp.StatusCode == 403 && resp.Header.Get("X-RateLimit-Remaining") == "0") {
			wait := time.Duration(1<<attempt) * 2 * time.Second
			if rs := resp.Header.Get("Retry-After"); rs != "" {
				if secs, cerr := strconv.Atoi(rs); cerr == nil {
					wait = time.Duration(secs) * time.Second
				}
			}
			resp.Body.Close()
			if attempt == 3 {
				return nil, fmt.Errorf("github %s %s: %d after retries", method, path, resp.StatusCode)
			}
			time.Sleep(wait)
			continue
		}
		break
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return body, fmt.Errorf("github %s %s: %d: %.200s", method, path, resp.StatusCode, body)
	}
	return body, nil
}

// GetAll follows Link-header pagination and returns concatenated JSON array items
// for list endpoints. Pass a path without per_page; it appends per_page=100.
func (c *Client) GetAll(path string) ([]json.RawMessage, error) {
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	var all []json.RawMessage
	next := path + sep + "per_page=100"
	for next != "" {
		req, err := http.NewRequest("GET", "https://api.github.com"+next, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.Token)
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, err := c.hc.Do(req)
		if err != nil {
			return all, err
		}
		body, _ := io.ReadAll(resp.Body)
		link := resp.Header.Get("Link")
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			return all, fmt.Errorf("github GET %s: %d", next, resp.StatusCode)
		}
		var page []json.RawMessage
		if err := json.Unmarshal(body, &page); err != nil {
			return all, err
		}
		all = append(all, page...)
		next = nextLink(link)
	}
	return all, nil
}

// nextLink parses the Link header for rel="next" and returns the path+query portion.
func nextLink(header string) string {
	for _, part := range strings.Split(header, ",") {
		if !strings.Contains(part, `rel="next"`) {
			continue
		}
		start := strings.Index(part, "<")
		end := strings.Index(part, ">")
		if start == -1 || end == -1 {
			return ""
		}
		u := part[start+1 : end]
		if idx := strings.Index(u, "api.github.com"); idx != -1 {
			return u[idx+len("api.github.com"):]
		}
	}
	return ""
}

// Comment posts a comment on an issue or PR.
func (c *Client) Comment(repo string, number int, body string) error {
	_, err := c.Do("POST", fmt.Sprintf("/repos/%s/issues/%d/comments", repo, number),
		map[string]string{"body": body})
	return err
}

// AddLabels adds labels to an issue or PR, creating nothing if missing.
func (c *Client) AddLabels(repo string, number int, labels []string) error {
	_, err := c.Do("POST", fmt.Sprintf("/repos/%s/issues/%d/labels", repo, number),
		map[string]any{"labels": labels})
	return err
}

// EnsureLabel creates the label if it doesn't exist (ignores already-exists 422).
func (c *Client) EnsureLabel(repo, name, color, description string) {
	_, err := c.Do("POST", fmt.Sprintf("/repos/%s/labels", repo), map[string]string{
		"name": name, "color": color, "description": description,
	})
	if err != nil {
		log.Printf("ensure label %s: %v (usually fine — it exists)", name, err)
	}
}
