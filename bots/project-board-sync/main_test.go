package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sanketpatel32/github-bot-store/shared/ghbot"
)

// GitHub answers the wrong owner type with NOT_FOUND. The fallback to the
// other scope must still find the project and its Status field.
func TestFindProjectFallsBackAcrossOwnerType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch {
		case strings.Contains(string(body), "organization(login:$owner)"):
			w.Write([]byte(`{"data":{"organization":{"projectV2":null}},"errors":[{"type":"NOT_FOUND","message":"Could not resolve to an Organization with the login of 'o'."}]}`))
		case strings.Contains(string(body), "fields(first:20)"):
			w.Write([]byte(`{"data":{"node":{"fields":{"nodes":[{"id":"F1","name":"Status","options":[{"id":"O1","name":"In Progress"}]}]}}}}`))
		default:
			w.Write([]byte(`{"data":{"user":{"projectV2":{"id":"PVT_user"}}}}`))
		}
	}))
	t.Setenv("GITHUB_API_URL", srv.URL)
	defer srv.Close()

	projectID, fieldID, options, err := findProject(ghbot.NewClient("x"), "o", 7, true)
	if err != nil {
		t.Fatalf("findProject: %v", err)
	}
	if projectID != "PVT_user" || fieldID != "F1" || options["in progress"] != "O1" {
		t.Errorf("got project %q field %q options %v", projectID, fieldID, options)
	}
}
