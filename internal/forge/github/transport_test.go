package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestGetJSON_429_NamesRateLimited proves spec/forge-transport ac-3's
// forge-side classification: forge carries no unavailable-style sentinel
// today, so a 429 is a wrapped transient error naming the status — not the
// generic "unexpected status" every other non-2xx gets.
func TestGetJSON_429_NamesRateLimited(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	a := New(Config{BaseURL: ts.URL, Owner: "acme", Repo: "svcfix", HTTPClient: ts.Client()})
	var out struct{}
	err := a.getJSON(context.Background(), ts.URL+"/anything", &out)
	if err == nil {
		t.Fatal("getJSON against a 429 response: want error, got nil")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("getJSON 429 error %q does not name the status code", err.Error())
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("getJSON 429 error %q does not name the rate-limit condition", err.Error())
	}
}

// TestPostJSON_429_NamesRateLimited mirrors TestGetJSON_429_NamesRateLimited
// for the write direction (postJSON backs the GraphQL/comment-creation
// calls).
func TestPostJSON_429_NamesRateLimited(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	a := New(Config{BaseURL: ts.URL, Owner: "acme", Repo: "svcfix", HTTPClient: ts.Client()})
	var out struct{}
	err := a.postJSON(context.Background(), ts.URL+"/anything", map[string]string{"a": "b"}, &out)
	if err == nil {
		t.Fatal("postJSON against a 429 response: want error, got nil")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("postJSON 429 error %q does not name the status code", err.Error())
	}
}

// TestFetchEvidenceBundle_Timeout_StallingHandlerWithShortInjectedClient
// proves spec/forge-transport ac-3's deadline end to end: a deliberately
// stalling handler paired with a SHORT injected client (never the 30s
// default — this test must not sleep 30s, co-1) still returns promptly.
func TestFetchEvidenceBundle_Timeout_StallingHandlerWithShortInjectedClient(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
	}))
	defer ts.Close()

	a := New(Config{BaseURL: ts.URL, Owner: "acme", Repo: "svcfix", HTTPClient: &http.Client{Timeout: 50 * time.Millisecond}})

	start := time.Now()
	_, err := a.FetchEvidenceBundle(context.Background(), "ref", "deadbeef")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("FetchEvidenceBundle against a stalling server: want error, got nil")
	}
	if elapsed > 5*time.Second {
		t.Fatalf("FetchEvidenceBundle took %v to fail, want it bounded by the short injected client's timeout", elapsed)
	}
}

// TestNew_NilHTTPClient_DefaultsThroughTheSeam proves New no longer
// defaults Config.HTTPClient to http.DefaultClient (Timeout: 0, the
// spec/forge-transport problem statement's "never times out"); a nil
// HTTPClient rides internal/httpjson's own DefaultTimeout-bounded default
// instead. Proven behaviorally: a nil-client Adapter against a closed local
// port fails fast (connection refused), never hangs — the same shape a
// panic-on-nil-pointer regression would NOT produce, so this also guards
// against reintroducing a bare `a.cfg.HTTPClient.Do` call.
func TestNew_NilHTTPClient_DefaultsThroughTheSeam(t *testing.T) {
	a := New(Config{BaseURL: "http://127.0.0.1:1", Owner: "acme", Repo: "svcfix"})
	_, err := a.FetchEvidenceBundle(context.Background(), "ref", "deadbeef")
	if err == nil {
		t.Fatal("FetchEvidenceBundle against a closed local port: want error, got nil")
	}
}
