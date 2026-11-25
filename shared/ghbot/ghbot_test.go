package ghbot

import (
	"sync"
	"time"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestVerifySignature(t *testing.T) {
	body := []byte(`{"action":"opened"}`)
	cases := []struct {
		name   string
		secret string
		header string
		want   bool
	}{
		{"valid", "s3cret", sign("s3cret", body), true},
		{"wrong secret", "s3cret", sign("other", body), false},
		{"garbage header", "s3cret", "not-a-signature", false},
		{"missing prefix", "s3cret", hex.EncodeToString([]byte("x")), false},
		{"empty header", "s3cret", "", false},
		{"empty secret allows (local testing)", "", "anything", true},
		{"empty secret allows empty header", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := VerifySignature(tc.secret, body, tc.header); got != tc.want {
				t.Errorf("VerifySignature = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNextLink(t *testing.T) {
	header := `<https://api.github.com/repos/o/r/issues?per_page=100&page=2>; rel="next", <https://api.github.com/repos/o/r/issues?per_page=100&page=3>; rel="last"`
	got := nextLink(header)
	want := "/repos/o/r/issues?per_page=100&page=2"
	if got != want {
		t.Errorf("nextLink = %q, want %q", got, want)
	}
	if nextLink("") != "" {
		t.Error("empty header should yield empty next link")
	}
	lastOnly := `<https://api.github.com/x?page=3>; rel="last"`
	if nextLink(lastOnly) != "" {
		t.Error("header without next rel should yield empty")
	}
}

func TestSeenDeliveryDedupes(t *testing.T) {
	if seenDelivery("d-1") {
		t.Error("first delivery should not be a duplicate")
	}
	if !seenDelivery("d-1") {
		t.Error("second identical delivery must be flagged duplicate")
	}
	if seenDelivery("d-2") {
		t.Error("distinct delivery should not be a duplicate")
	}
	if seenDelivery("") {
		t.Error("empty id never dedupes")
	}
}

func TestDispatchSaturation(t *testing.T) {
	// fill the queue with blocking jobs; eventually dispatch must refuse
	block := make(chan struct{})
	refused := false
	for i := 0; i < queueDepth+workerCount+4; i++ {
		if !dispatch(func() { <-block }) {
			refused = true
			break
		}
	}
	close(block)
	if !refused {
		t.Error("saturated queue must refuse instead of dropping silently")
	}
}

func TestWithResourceLock(t *testing.T) {
	var mu sync.Mutex
	inFlight := 0
	maxInFlight := 0
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			WithResourceLock("owner/repo#1", func() {
				mu.Lock()
				inFlight++
				if inFlight > maxInFlight {
					maxInFlight = inFlight
				}
				mu.Unlock()
				time.Sleep(time.Millisecond)
				mu.Lock()
				inFlight--
				mu.Unlock()
			})
		}()
	}
	wg.Wait()
	if maxInFlight != 1 {
		t.Errorf("resource lock violated: maxInFlight=%d", maxInFlight)
	}
}
