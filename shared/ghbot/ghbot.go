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
	"time"
)

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

// Serve starts the bot's HTTP server: POST /webhook plus GET /healthz.
func Serve(cfg Config, name string, handler WebhookHandler) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}
		if !VerifySignature(cfg.Secret, body, r.Header.Get("X-Hub-Signature-256")) {
			http.Error(w, "bad signature", http.StatusUnauthorized)
			return
		}
		event := r.Header.Get("X-GitHub-Event")
		go func() {
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("[%s] panic handling %s: %v", name, event, rec)
				}
			}()
			handler(event, body)
		}()
		w.WriteHeader(http.StatusAccepted)
	})
	log.Printf("[%s] listening on :%s", name, cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, mux))
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
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return body, fmt.Errorf("github %s %s: %d: %.200s", method, path, resp.StatusCode, body)
	}
	return body, nil
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
