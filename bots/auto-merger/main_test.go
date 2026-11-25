package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

// testClient points a ghbot.Client at a local test server by overriding
// the API base — ghbot hardcodes api.github.com, so we rewrite via transport.
type rewriteTransport struct{ target string }

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = t.target
	req.Host = t.target
	return http.DefaultTransport.RoundTrip(req)
}

func newTestClient(t *testing.T, handler http.HandlerFunc) (*ghbot.Client, func()) {
	t.Helper()
	srv := httptest.NewServer(handler)
	host := srv.URL[len("http://"):]
	c := ghbot.NewClient("test-token")
	c.SetHTTPClient(&http.Client{Transport: rewriteTransport{host}})
	return c, srv.Close
}

func checkRunsPayload(total int, statuses [][2]string) string {
	type run struct {
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
	}
	runs := []run{}
	for _, s := range statuses {
		runs = append(runs, run{Status: s[0], Conclusion: s[1]})
	}
	out, _ := json.Marshal(map[string]any{"total_count": total, "check_runs": runs})
	return string(out)
}

func TestChecksGreen(t *testing.T) {
	cases := []struct {
		name      string
		runs      string
		legacy    string
		wantGreen bool
	}{
		{"all success", checkRunsPayload(2, [][2]string{{"completed", "success"}, {"completed", "success"}}),
			`{"state":"success","total_count":1}`, true},
		{"one failure", checkRunsPayload(2, [][2]string{{"completed", "success"}, {"completed", "failure"}}),
			`{"state":"success","total_count":0}`, false},
		{"in progress", checkRunsPayload(1, [][2]string{{"in_progress", ""}}),
			`{"state":"success","total_count":1}`, false},
		{"skipped counts green", checkRunsPayload(2, [][2]string{{"completed", "success"}, {"completed", "skipped"}}),
			`{"state":"success","total_count":0}`, true},
		{"neutral counts green", checkRunsPayload(1, [][2]string{{"completed", "neutral"}}),
			`{"state":"success","total_count":0}`, true},
		{"no checks at all", checkRunsPayload(0, nil),
			`{"state":"pending","total_count":0}`, true},
		{"legacy failure blocks", checkRunsPayload(0, nil),
			`{"state":"failure","total_count":2}`, false},
		{"legacy pending blocks", checkRunsPayload(0, nil),
			`{"state":"pending","total_count":1}`, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, closeSrv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/repos/o/r/commits/sha1/check-runs" {
					fmt.Fprint(w, tc.runs)
					return
				}
				if r.URL.Path == "/repos/o/r/commits/sha1/status" {
					fmt.Fprint(w, tc.legacy)
					return
				}
				http.Error(w, "unexpected: "+r.URL.Path, 500)
			})
			defer closeSrv()
			if got := checksGreen(client, "o/r", "sha1"); got != tc.wantGreen {
				t.Errorf("checksGreen = %v, want %v", got, tc.wantGreen)
			}
		})
	}
}

func TestAutomergeable(t *testing.T) {
	mk := func(state string, labels ...string) *prInfo {
		p := &prInfo{State: state}
		for _, l := range labels {
			p.Labels = append(p.Labels, struct {
				Name string `json:"name"`
			}{Name: l})
		}
		return p
	}
	if !mk("open", "automerge").automergeable() {
		t.Error("open+automerge should be automergeable")
	}
	if mk("closed", "automerge").automergeable() {
		t.Error("closed PR must not be automergeable")
	}
	if mk("open", "bug").automergeable() {
		t.Error("open without label must not be automergeable")
	}
}
