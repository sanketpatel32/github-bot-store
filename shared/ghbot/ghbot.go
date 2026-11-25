// Package ghbot provides the shared plumbing every bot in the store uses:
// webhook signature verification, a tiny GitHub REST client, and a
// one-line HTTP server bootstrap. Standard library only.
package ghbot

import (
	"bytes"
	"crypto"
	"crypto/hmac"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
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

// unseeDelivery forgets a delivery ID — used when admission fails after the
// dedupe check (e.g. workers saturated), so GitHub's redelivery is not
// silently swallowed by the cache (#102).
func unseeDelivery(id string) {
	dedupe.Lock()
	defer dedupe.Unlock()
	delete(dedupe.ids, id)
}

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

// ---- lightweight metrics + structured logs (#35) ----

var metrics = struct {
	sync.Mutex
	events   map[string]int
	errors   map[string]int
	apiCalls int
}{events: map[string]int{}, errors: map[string]int{}}

// Logf emits a structured key=value log line — greppable without a log
// pipeline, still plain enough for journald (#35).
func Logf(bot, level, msg string, kv ...any) {
	pairs := ""
	for i := 0; i+1 < len(kv); i += 2 {
		pairs += fmt.Sprintf(" %v=%v", kv[i], kv[i+1])
	}
	log.Printf("level=%s bot=%s msg=%q%s", level, bot, msg, pairs)
}

// recordEvent increments the per-event-type counter.
func recordEvent(event string) {
	metrics.Lock()
	metrics.events[event]++
	metrics.Unlock()
}

// recordError increments the per-event error counter.
func recordError(event string) {
	metrics.Lock()
	metrics.errors[event]++
	metrics.Unlock()
}

// metricsHandler exposes counters in the Prometheus text format — scrape it
// or curl it, no client library needed at this scale.
func metricsHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		metrics.Lock()
		defer metrics.Unlock()
		fmt.Fprintf(w, "# TYPE ghbot_events_total counter\n")
		for ev, n := range metrics.events {
			fmt.Fprintf(w, "ghbot_events_total{bot=%q,event=%q} %d\n", name, ev, n)
		}
		fmt.Fprintf(w, "# TYPE ghbot_errors_total counter\n")
		for ev, n := range metrics.errors {
			fmt.Fprintf(w, "ghbot_errors_total{bot=%q,event=%q} %d\n", name, ev, n)
		}
		fmt.Fprintf(w, "# TYPE ghbot_api_calls_total counter\nghbot_api_calls_total{bot=%q} %d\n", name, metrics.apiCalls)
	}
}

// RecordAPICall increments the outbound API call counter — called by the
// client on every request.
func (c *Client) recordAPI() {
	metrics.Lock()
	metrics.apiCalls++
	metrics.Unlock()
}

// ---- outbound notifications (#88) ----

// Notify posts a plain-text message to NOTIFY_WEBHOOK_URL — works with
// Slack incoming webhooks, Discord webhooks, or any endpoint accepting
// {"text": "..."} / {"content": "..."}. Errors are logged, never fatal:
// notifications are best-effort side channels, not bot logic.
// Opt-in per bot; nothing is sent when the env var is unset.
func Notify(bot, message string) {
	url := os.Getenv("NOTIFY_WEBHOOK_URL")
	if url == "" {
		return
	}
	payload := map[string]string{"text": fmt.Sprintf("[%s] %s", bot, message), "content": fmt.Sprintf("[%s] %s", bot, message)}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	hc := &http.Client{Timeout: 5 * time.Second}
	resp, err := hc.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		log.Printf("notify: %v", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Printf("notify: webhook returned %d", resp.StatusCode)
	}
}

// ---- persistent scheduler (#75) ----

// ServeCron runs fn once immediately, then on every interval, while serving
// the health/metrics endpoints — cron bots become single long-running
// one-shot executions instead of needing an external crontab. In GitHub
// Actions mode it runs fn exactly once and returns (the workflow schedule
// is the timer).
func ServeCron(cfg Config, name string, interval time.Duration, fn func()) {
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		fn()
		return
	}
	go func() {
		for {
			func() {
				defer func() {
					if rec := recover(); rec != nil {
						log.Printf("[%s] panic in scheduled run: %v — continuing", name, rec)
					}
				}()
				fn()
			}()
			jitter := time.Duration(rand.Int63n(int64(interval / 10)))
			time.Sleep(interval + jitter)
		}
	}()
	log.Printf("[%s] scheduled every %s (with jitter)", name, interval)
	ServeHealth(cfg, name)
}

// ---- CLI conveniences: version + doctor (#44) ----

// Version is stamped at build time: go build -ldflags "-X ghbot.Version=x.y.z"
var Version = "dev"

// HandleCLIArgs implements the shared subcommands every bot gets for free:
//
//	<bot> version  — print version and exit
//	<bot> doctor   — validate configuration and API connectivity, exit non-zero on failure
//
// Returns true if a subcommand ran (caller should exit).
func HandleCLIArgs(name string, cfg Config) bool {
	if len(os.Args) < 2 {
		return false
	}
	switch os.Args[1] {
	case "version":
		fmt.Printf("%s %s\n", name, Version)
		return true
	case "doctor":
		return runDoctor(name, cfg)
	}
	return false
}

func runDoctor(name string, cfg Config) bool {
	problems := 0
	check := func(ok bool, label, detail string) {
		status := "ok"
		if !ok {
			status = "FAIL"
			problems++
		}
		fmt.Printf("[%s] %s: %s\n", status, label, detail)
	}

	check(cfg.Token != "", "token", "GITHUB_TOKEN is set")
	if os.Getenv("GITHUB_ACTIONS") != "true" {
		webhook := cfg.Secret != "" || os.Getenv("ALLOW_UNSIGNED_WEBHOOKS") == "true"
		check(webhook, "webhook-secret", "WEBHOOK_SECRET set (or unsigned explicitly allowed)")
	}

	// connectivity + auth validity
	client := NewClient(cfg.Token)
	_, err := client.Do("GET", "/rate_limit", nil)
	check(err == nil, "api-connectivity", fmt.Sprintf("GET /rate_limit err=%v", err))

	if err == nil {
		body, _ := client.Do("GET", "/rate_limit", nil)
		var rl struct {
			Resources struct {
				Core struct {
					Remaining int `json:"remaining"`
				} `json:"core"`
			} `json:"resources"`
		}
		if json.Unmarshal(body, &rl) == nil {
			check(rl.Resources.Core.Remaining > 100, "rate-limit",
				fmt.Sprintf("%d requests remaining", rl.Resources.Core.Remaining))
		}
	}

	if repo := os.Getenv("REPO"); repo != "" {
		_, err := client.Do("GET", "/repos/"+repo, nil)
		check(err == nil, "repo-access", fmt.Sprintf("repo %s reachable", repo))
	}

	if appCfg != nil {
		check(true, "github-app", "app credentials configured")
	}

	fmt.Printf("%s doctor: %d problem(s)\n", name, problems)
	os.Exit(map[bool]int{true: 1, false: 0}[problems > 0])
	return true
}

// ---- per-resource mutation coordination (#72) ----

var resourceLocks = struct {
	sync.Mutex
	locks map[string]*sync.Mutex
}{locks: map[string]*sync.Mutex{}}

// WithResourceLock serializes mutations against the same logical resource
// (e.g. "owner/repo#123") across all goroutines in this process — two
// webhook deliveries for the same PR can't double-comment or race label
// updates when they arrive together (#72).
func WithResourceLock(key string, fn func()) {
	resourceLocks.Lock()
	mu, ok := resourceLocks.locks[key]
	if !ok {
		mu = &sync.Mutex{}
		resourceLocks.locks[key] = mu
	}
	resourceLocks.Unlock()
	mu.Lock()
	defer mu.Unlock()
	fn()
}

// newServer builds an http.Server with production timeouts — slow-client
// and slowloris resistance out of the box.
func newServer(port string, handler http.Handler) *http.Server {
	bind := os.Getenv("BIND_ADDR") // e.g. "127.0.0.1" — default listens on all interfaces
	addr := ":" + port
	if bind != "" {
		addr = bind + ":" + port
	}
	return &http.Server{
		Addr:              addr,
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
	// AllowedRepos restricts webhook processing to these full repo names
	// (owner/repo). Empty means all repos allowed (#15).
	AllowedRepos map[string]bool
}

// LoadConfig reads env vars and exits with a helpful message if the token is missing.
// defaultPort is used when PORT is not set.
// secretOrFile reads a secret from an env var, falling back to reading it
// from the file named by the "<NAME>_FILE" variant — the pattern used by
// Docker secrets and systemd credentials (#71).
func secretOrFile(name string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	if f := os.Getenv(name + "_FILE"); f != "" {
		if data, err := os.ReadFile(f); err == nil {
			return strings.TrimSpace(string(data))
		} else {
			log.Printf("warning: %s_FILE set but unreadable: %v", name, err)
		}
	}
	return ""
}

func LoadConfig(botName, defaultPort string) Config {
	cfg := Config{
		Token:  secretOrFile("GITHUB_TOKEN"),
		Secret: secretOrFile("WEBHOOK_SECRET"),
		Port:   os.Getenv("PORT"),
	}
	if cfg.Token == "" && appCfg == nil {
		// either a static token or complete, valid app credentials will do —
		// requiring both defeated the documented app-auth mode (#100)
		if os.Getenv("GITHUB_APP_ID") != "" {
			log.Fatalf("[%s] GitHub App auth is incomplete or invalid — need GITHUB_APP_ID, GITHUB_APP_INSTALLATION_ID and a parseable GITHUB_APP_PRIVATE_KEY", botName)
		}
		log.Fatalf("[%s] GITHUB_TOKEN is not set — copy .env.example to .env (or use GITHUB_TOKEN_FILE)", botName)
	}
	// config validation (#71): catch malformed values at boot, not mid-run
	if p, err := strconv.Atoi(cfg.Port); cfg.Port != "" && (err != nil || p < 1 || p > 65535) {
		if os.Getenv("PORT") != "" {
			log.Fatalf("[%s] PORT %q is not a valid port number", botName, os.Getenv("PORT"))
		}
	}
	if list := os.Getenv("ALLOWED_REPOS"); list != "" {
		for _, r := range strings.Split(list, ",") {
			r = strings.TrimSpace(r)
			if r != "" && !strings.Contains(r, "/") {
				log.Fatalf("[%s] ALLOWED_REPOS entry %q must be owner/repo", botName, r)
			}
		}
	}
	if cfg.Port == "" {
		cfg.Port = defaultPort
	}
	if list := os.Getenv("ALLOWED_REPOS"); list != "" {
		cfg.AllowedRepos = map[string]bool{}
		for _, r := range strings.Split(list, ",") {
			if r = strings.TrimSpace(r); r != "" {
				cfg.AllowedRepos[r] = true
			}
		}
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
	mux.HandleFunc("/metrics", metricsHandler(name))
	log.Printf("[%s] health-only server listening on %s", name, newServer(cfg.Port, nil).Addr)
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

// ---- durable webhook inbox (#69) ----

// When SPOOL_DIR is set, every accepted delivery is written to disk before
// processing and only deleted after the handler finishes — a crash or
// restart no longer loses events: the spool is replayed on boot.
type spooledEvent struct {
	Event    string `json:"event"`
	Delivery string `json:"delivery"`
	Payload  []byte `json:"payload"`
}

func spoolPath(dir, delivery string) string {
	return dir + "/" + strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' {
			return -1
		}
		return r
	}, delivery) + ".json"
}

func writeSpool(dir, event, delivery string, payload []byte) error {
	data, err := json.Marshal(spooledEvent{event, delivery, payload})
	if err != nil {
		return err
	}
	return os.WriteFile(spoolPath(dir, delivery), data, 0o600)
}

// replaySpool re-processes any deliveries left over from a crash.
func replaySpool(dir, name string, handler WebhookHandler) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(dir + "/" + e.Name())
		if err != nil {
			continue
		}
		var se spooledEvent
		if json.Unmarshal(data, &se) != nil {
			continue
		}
		log.Printf("[%s] replaying spooled delivery %s", name, se.Delivery)
		handler(se.Event, se.Payload)
		os.Remove(dir + "/" + e.Name())
	}
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
	if dir := os.Getenv("SPOOL_DIR"); dir != "" {
		os.MkdirAll(dir, 0o700)
		replaySpool(dir, name, handler)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/metrics", metricsHandler(name))
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

		// repo allowlist (#15): one bot binary can serve a webhook installed
		// org-wide; without this it acts on repos you never intended
		if len(cfg.AllowedRepos) > 0 {
			var meta struct {
				Repo struct {
					FullName string `json:"full_name"`
				} `json:"repository"`
			}
			if json.Unmarshal(body, &meta) == nil && meta.Repo.FullName != "" &&
				!cfg.AllowedRepos[meta.Repo.FullName] {
				log.Printf("[%s] repo %s not in ALLOWED_REPOS — ignoring", name, meta.Repo.FullName)
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

		spoolDir := os.Getenv("SPOOL_DIR")
		if spoolDir != "" {
			if err := writeSpool(spoolDir, event, deliveryID, body); err != nil {
				log.Printf("[%s] spool write failed: %v", name, err)
			}
		}
		recordEvent(event)
		job := func() {
			defer func() {
				if rec := recover(); rec != nil {
					recordError(event)
					log.Printf("[%s] panic handling %s: %v — delivery stays in spool for replay", name, event, rec)
				}
			}()
			handler(event, body)
			if spoolDir != "" {
				os.Remove(spoolPath(spoolDir, deliveryID))
			}
		}
		if !dispatch(job) {
			// workers saturated — forget the dedupe record so the redelivery
			// this 429 triggers is actually processed (#102)
			unseeDelivery(deliveryID)
			http.Error(w, "busy — retry", http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	log.Printf("[%s] listening on %s", name, newServer(cfg.Port, nil).Addr)
	log.Fatal(newServer(cfg.Port, mux).ListenAndServe())
}

// APIError carries the method, path, status code and response body of a
// failed GitHub API call, so callers can branch on the status instead of
// parsing strings (#16).
type APIError struct {
	Method string
	Path   string
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("github %s %s: %d: %.200s", e.Method, e.Path, e.Status, e.Body)
}

// IsNotFound reports whether err is a GitHub 404 — the common "resource
// gone, skip silently" case.
func IsNotFound(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status == 404
	}
	return false
}

// ---- GitHub App installation auth (#14) ----

// appConfig holds GitHub App credentials when configured.
type appConfig struct {
	id             string
	installationID string
	key            *rsa.PrivateKey
	mu             sync.Mutex
	token          string
	expiry         time.Time
}

var appCfg *appConfig

func init() {
	appID := os.Getenv("GITHUB_APP_ID")
	instID := os.Getenv("GITHUB_APP_INSTALLATION_ID")
	keyPEM := secretOrFile("GITHUB_APP_PRIVATE_KEY")
	if appID == "" || instID == "" || keyPEM == "" {
		return
	}
	block, _ := pem.Decode([]byte(keyPEM))
	if block == nil {
		log.Printf("github app: GITHUB_APP_PRIVATE_KEY is not valid PEM — ignoring app auth")
		return
	}
	var key *rsa.PrivateKey
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		key = k
	} else if pkcs8, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rsaKey, ok := pkcs8.(*rsa.PrivateKey); ok {
			key = rsaKey
		}
	}
	if key == nil {
		log.Printf("github app: could not parse private key — ignoring app auth")
		return
	}
	appCfg = &appConfig{id: appID, installationID: instID, key: key}
}

// appJWT mints a short-lived JWT signed with the app's private key (RS256).
func (a *appConfig) appJWT() (string, error) {
	now := time.Now()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims := fmt.Sprintf(`{"iat":%d,"exp":%d,"iss":"%s"}`,
		now.Add(-30*time.Second).Unix(), now.Add(9*time.Minute).Unix(), a.id)
	payload := base64.RawURLEncoding.EncodeToString([]byte(claims))
	unsigned := header + "." + payload
	hash := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(cryptorand.Reader, a.key, crypto.SHA256, hash[:])
	if err != nil {
		return "", err
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// installationToken returns a cached installation access token, minting a
// new one when expired. Tokens live an hour; refresh 5 minutes early.
func (a *appConfig) installationToken(hc *http.Client) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.token != "" && time.Now().Before(a.expiry.Add(-5*time.Minute)) {
		return a.token, nil
	}
	jwt, err := a.appJWT()
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("POST",
		fmt.Sprintf(apiBase()+"/app/installations/%s/access_tokens", a.installationID), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("mint installation token: %d: %.200s", resp.StatusCode, body)
	}
	var result struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	a.token, a.expiry = result.Token, result.ExpiresAt
	log.Printf("github app: minted installation token, expires %s", result.ExpiresAt.Format(time.RFC3339))
	return a.token, nil
}

// authToken resolves which token to use for a request: a GitHub App
// installation token when app credentials are configured, otherwise the
// static token.
func (c *Client) authToken() string {
	if appCfg != nil {
		if tok, err := appCfg.installationToken(c.hc); err == nil {
			return tok
		} else {
			log.Printf("github app auth failed (%v) — falling back to static token", err)
		}
	}
	return c.Token
}

// Client is a minimal authenticated GitHub REST client.
type Client struct {
	Token string
	hc    *http.Client
}

// apiBase is the REST API root — GITHUB_API_URL overrides it for GitHub
// Enterprise Server, e.g. https://ghe.corp.example/api/v3 (#38).
func apiBase() string {
	if base := os.Getenv("GITHUB_API_URL"); base != "" {
		return strings.TrimSuffix(base, "/")
	}
	return "https://api.github.com"
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
	// global dry-run (#37): with DRY_RUN=true, mutating calls are logged
	// and skipped — every bot gains a safe preview mode for free. Reads
	// still execute so bots can compute what they would have done.
	if os.Getenv("DRY_RUN") == "true" && method != "GET" && method != "HEAD" {
		log.Printf("dry-run: would %s %s", method, path)
		return []byte(`{}`), nil
	}
	c.recordAPI()
	req, err := http.NewRequest(method, apiBase()+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.authToken())
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	var resp *http.Response
	// reads are idempotent, so network errors are safe to retry; writes
	// (POST/PATCH/DELETE) are not retried on network errors because the
	// mutation may have already landed server-side — only definitive 5xx
	// responses are retried for those (#12).
	isRead := method == "GET" || method == "HEAD"
	for attempt := 0; attempt < 4; attempt++ {
		resp, err = c.hc.Do(req)
		if err != nil {
			if attempt == 3 || !isRead {
				return nil, err
			}
			time.Sleep(jitterBackoff(attempt))
			continue
		}
		// retry on server errors and secondary rate limits
		if resp.StatusCode == 502 || resp.StatusCode == 503 ||
			(resp.StatusCode == 429) || (resp.StatusCode == 403 && resp.Header.Get("X-RateLimit-Remaining") == "0") {
			wait := jitterBackoff(attempt)
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
		return body, &APIError{Method: method, Path: path, Status: resp.StatusCode, Body: string(body)}
	}
	return body, nil
}

// jitterBackoff returns exponential backoff with jitter — without jitter,
// retrying bots thundering-herd the API after an outage (#12).
func jitterBackoff(attempt int) time.Duration {
	base := time.Duration(1<<attempt) * time.Second
	return base + time.Duration(rand.Int63n(int64(base)))
}

// ---- GraphQL support (discussions, projects v2) ----

// GraphQL executes a query against the GitHub GraphQL API and returns the
// raw "data" payload. Needed for the few surfaces REST doesn't cover:
// discussions and Projects v2.
func (c *Client) GraphQL(query string, variables map[string]any) (json.RawMessage, error) {
	// dry-run parity with the REST path (#101): mutations must not leave the
	// process in preview mode. Detection is heuristic — the operation keyword
	// is the first meaningful token of the document — which is exact for every
	// caller in this repo since none lead with whitespace or comments.
	if os.Getenv("DRY_RUN") == "true" && strings.HasPrefix(strings.TrimSpace(query), "mutation") {
		Logf("graphql", "dry-run", "DRY_RUN: skipping mutation %.60s", query)
		return json.RawMessage(`{"data":{"dryRun":true}}`), nil
	}
	payload := map[string]any{"query": query}
	if variables != nil {
		payload["variables"] = variables
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", apiBase()+"/graphql", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.authToken())
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, &APIError{Method: "POST", Path: "/graphql", Status: resp.StatusCode, Body: string(body)}
	}
	var result struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("graphql: %s", result.Errors[0].Message)
	}
	return result.Data, nil
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
		req, err := http.NewRequest("GET", apiBase()+next, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.authToken())
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
		for _, host := range []string{"api.github.com", strings.TrimPrefix(strings.TrimPrefix(apiBase(), "https://"), "http://")} {
			if idx := strings.Index(u, host); idx != -1 {
				return u[idx+len(host):]
			}
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

// SweepInterval reads SWEEP_INTERVAL_HOURS (default 24) as a duration,
// clamping to [1, 720] hours. Zero or negative values previously crashed
// scheduled bots because time.NewTicker panics on non-positive intervals.
func SweepInterval(defaultHours int) time.Duration {
	h := defaultHours
	if v := os.Getenv("SWEEP_INTERVAL_HOURS"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			h = n
		}
	}
	if h < 1 {
		h = 1
	}
	if h > 720 {
		h = 720
	}
	return time.Duration(h) * time.Hour
}
