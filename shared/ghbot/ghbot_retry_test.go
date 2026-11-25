package ghbot

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// flakyServer answers 502 for the first `fail` requests, then 200 with ok.
// It records the raw body of every request so tests can prove a retry
// resent the payload instead of an empty, already-drained reader.
func flakyServer(t *testing.T, fail int, ok string) (*httptest.Server, *[]string) {
	t.Helper()
	bodies := &[]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		*bodies = append(*bodies, string(data))
		if len(*bodies) <= fail {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Write([]byte(ok))
	}))
	t.Setenv("GITHUB_API_URL", srv.URL)
	return srv, bodies
}

func noBackoff(t *testing.T) {
	t.Helper()
	old := retryBackoff
	retryBackoff = func(int) time.Duration { return 0 }
	t.Cleanup(func() { retryBackoff = old })
}

func TestDoRetriesWriteWithBody(t *testing.T) {
	noBackoff(t)
	srv, bodies := flakyServer(t, 1, `{"ok":true}`)
	defer srv.Close()
	c := NewClient("x")
	if _, err := c.Do("POST", "/repos/o/r/issues/1/comments", map[string]string{"body": "hi"}); err != nil {
		t.Fatalf("retry after 502 failed: %v", err)
	}
	if len(*bodies) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(*bodies))
	}
	for i, b := range *bodies {
		if b != `{"body":"hi"}` {
			t.Errorf("attempt %d sent body %q — retry must resend the payload", i+1, b)
		}
	}
}

// a retry can land on a fresh connection (the 502 closed the old one, or the
// backoff outlived the keep-alive). Then net/http's own GetBody retry never
// runs, so the body must be rewound by the caller.
func TestDoRetryResendsBodyOnFreshConnection(t *testing.T) {
	noBackoff(t)
	bodies := &[]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		*bodies = append(*bodies, string(data))
		if len(*bodies) == 1 {
			w.Header().Set("Connection", "close")
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	t.Setenv("GITHUB_API_URL", srv.URL)
	defer srv.Close()
	c := NewClient("x")
	if _, err := c.Do("POST", "/repos/o/r/issues/1/comments", map[string]string{"body": "hi"}); err != nil {
		t.Fatalf("retry on a fresh connection failed: %v", err)
	}
	if len(*bodies) != 2 || (*bodies)[1] != `{"body":"hi"}` {
		t.Fatalf("retry sent bodies %q — must resend the payload", *bodies)
	}
}

func TestGraphQLRetryResendsBodyOnFreshConnection(t *testing.T) {
	noBackoff(t)
	bodies := &[]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		*bodies = append(*bodies, string(data))
		if len(*bodies) == 1 {
			w.Header().Set("Connection", "close")
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Write([]byte(`{"data":{"viewer":{"login":"x"}}}`))
	}))
	t.Setenv("GITHUB_API_URL", srv.URL)
	defer srv.Close()
	c := NewClient("x")
	if _, err := c.GraphQL("query { viewer { login } }", map[string]any{"n": 1}); err != nil {
		t.Fatalf("retry on a fresh connection failed: %v", err)
	}
	if len(*bodies) != 2 || (*bodies)[1] != (*bodies)[0] {
		t.Fatalf("retry sent bodies %q — must resend the query", *bodies)
	}
}

func TestGraphQLRetriesWithBody(t *testing.T) {
	noBackoff(t)
	srv, bodies := flakyServer(t, 1, `{"data":{"viewer":{"login":"x"}}}`)
	defer srv.Close()
	c := NewClient("x")
	if _, err := c.GraphQL("query { viewer { login } }", map[string]any{"n": 1}); err != nil {
		t.Fatalf("retry after 502 failed: %v", err)
	}
	if len(*bodies) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(*bodies))
	}
	for i, b := range *bodies {
		if !strings.Contains(b, `"n":1`) || !strings.Contains(b, "viewer") {
			t.Errorf("attempt %d sent body %q — retry must resend the query", i+1, b)
		}
	}
}

// the spool decides whether to keep an event for replay by asking whether
// the failure looked like GitHub's — retries running out on 503 is exactly
// that case, so it must be recorded (#132).
func TestExhaustedRetriesRecordAPIFailure(t *testing.T) {
	noBackoff(t)
	srv, _ := flakyServer(t, 99, `{"ok":true}`)
	defer srv.Close()
	c := NewClient("x")
	before := apiFailureCount()
	if _, err := c.Do("POST", "/repos/o/r/issues/1/comments", map[string]string{"body": "hi"}); err == nil {
		t.Fatal("persistent 503 must fail")
	}
	if _, err := c.GraphQL("query { viewer { login } }", nil); err == nil {
		t.Fatal("persistent 503 must fail")
	}
	if got := apiFailureCount() - before; got != 2 {
		t.Errorf("recorded %d API failures, want 2 (REST + GraphQL)", got)
	}
}

// a GraphQL errors payload is a 200 from a healthy GitHub. NOT_FOUND is an
// authoritative "no such thing", not a failure, so it must not keep a
// delivery for replay; other error types (rate limits, server errors) still do.
func TestGraphQLErrorsRecordAPIFailureExceptNotFound(t *testing.T) {
	serve := func(errors string) *httptest.Server {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"errors":[` + errors + `]}`))
		}))
		t.Setenv("GITHUB_API_URL", srv.URL)
		return srv
	}
	before := apiFailureCount()

	srv := serve(`{"type":"RATE_LIMITED","message":"slow down"}`)
	defer srv.Close()
	if _, err := NewClient("x").GraphQL("query { viewer { login } }", nil); err == nil {
		t.Fatal("graphql error must fail")
	}
	if got := apiFailureCount() - before; got != 1 {
		t.Errorf("RATE_LIMITED recorded %d API failures, want 1", got)
	}

	srv2 := serve(`{"type":"NOT_FOUND","message":"no such thing"}`)
	defer srv2.Close()
	if _, err := NewClient("x").GraphQL("query { viewer { login } }", nil); err == nil {
		t.Fatal("graphql error must fail")
	}
	if got := apiFailureCount() - before; got != 1 {
		t.Errorf("NOT_FOUND recorded %d API failures, want still 1 — GitHub answered, it did not fail", got)
	}
}
