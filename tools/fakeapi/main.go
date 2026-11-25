// fakeapi — a local stand-in for the GitHub REST+GraphQL API used by the
// offline preview mode (#125). It answers reads from fixture files, records
// every mutation into a human-readable transcript, and never touches the
// network itself. Bots point at it with GITHUB_API_URL — the same override
// GitHub Enterprise users use — so preview mode cannot leak external calls.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type mutation struct {
	At     time.Time `json:"at"`
	Method string    `json:"method"`
	Path   string    `json:"path"`
	Body   string    `json:"body"`
}

var (
	mu        sync.Mutex
	mutations []mutation
)

// fixtures maps "METHOD /path/prefix" → response body. Loaded from the
// scenario's responses.json.
var fixtures = map[string]json.RawMessage{}

func fixtureKey(method, path string) string { return method + " " + path }

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9911"
	}
	dir := os.Getenv("FIXTURES")
	if dir == "" {
		log.Fatal("FIXTURES dir not set — point it at a scenario directory")
	}
	data, err := os.ReadFile(dir + "/responses.json")
	if err != nil {
		log.Fatalf("responses.json: %v", err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		log.Fatalf("parse responses.json: %v", err)
	}
	log.Printf("fakeapi: %d fixture responses loaded from %s", len(fixtures), dir)

	mux := http.NewServeMux()
	mux.HandleFunc("/transcript", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if len(mutations) == 0 {
			fmt.Fprintln(w, "(no mutations — the bot decided no action was needed)")
			return
		}
		for _, m := range mutations {
			fmt.Fprintf(w, "%s %s\n", m.Method, m.Path)
			if m.Body != "" && m.Body != "null" {
				fmt.Fprintf(w, "  body: %s\n", m.Body)
			}
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, 0)
		if r.Body != nil {
			buf := new(strings.Builder)
			b := make([]byte, 4096)
			for {
				n, err := r.Body.Read(b)
				buf.Write(b[:n])
				if err != nil {
					break
				}
			}
			body = []byte(buf.String())
		}
		path := r.URL.Path
		if r.URL.RawQuery != "" {
			path += "?" + r.URL.RawQuery
		}

		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			mu.Lock()
			mutations = append(mutations, mutation{time.Now(), r.Method, path, string(body)})
			mu.Unlock()
			log.Printf("mutation: %s %s", r.Method, path)
		}

		// exact match first, then longest-prefix match so a fixture for
		// /repos/o/r/issues covers numbered children too
		w.Header().Set("Content-Type", "application/json")
		if fx, ok := fixtures[fixtureKey(r.Method, path)]; ok {
			w.Write(fx)
			return
		}
		if r.Method == http.MethodGet {
			var keys []string
			for k := range fixtures {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			best := ""
			for _, k := range keys {
				parts := strings.SplitN(k, " ", 2)
				if len(parts) == 2 && strings.HasPrefix(path, parts[1]) && len(parts[1]) > len(best) {
					best = parts[1]
				}
			}
			if best != "" {
				w.Write(fixtures[fixtureKey("GET", best)])
				return
			}
			// unknown reads return empty objects/arrays — never an error that
			// would push the bot into a retry storm during preview
			w.Write([]byte(`[]`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	})
	log.Printf("fakeapi listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
