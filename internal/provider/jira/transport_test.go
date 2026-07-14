package jira_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/provider"
	"github.com/jyang234/verdi/internal/provider/jira"
	"github.com/jyang234/verdi/internal/provider/jira/jiratest"
)

// TestResolve_429_MapsToErrUnavailable proves spec/forge-transport ac-3's
// jira-side classification: HTTP 429 (rate limited) routes to
// provider.ErrUnavailable — the same degrade/retry path a 5xx already took
// — rather than falling through to the "unexpected status" default an
// unclassified status gets.
func TestResolve_429_MapsToErrUnavailable(t *testing.T) {
	server := jiratest.NewServer(testRollupField)
	t.Cleanup(server.Close)
	server.ForceStatus("RATE-1", http.StatusTooManyRequests)

	a := newAdapter(t, server, nil)
	_, err := a.Resolve(context.Background(), provider.StoryRef("jira:RATE-1"))
	if err == nil {
		t.Fatal("Resolve against a 429 response: want error, got nil")
	}
	if !errors.Is(err, provider.ErrUnavailable) {
		t.Fatalf("Resolve error = %v, want errors.Is(err, provider.ErrUnavailable)", err)
	}
}

// TestPublishRollup_429_MapsToErrUnavailable proves the same 429
// classification on PublishRollup's read-before-write GET, mirroring
// TestPublishRollup_FailureTable's existing 5xx cases.
func TestPublishRollup_429_MapsToErrUnavailable(t *testing.T) {
	server := jiratest.NewServer(testRollupField)
	t.Cleanup(server.Close)
	server.ForceStatus("RATE-2", http.StatusTooManyRequests)

	a := newAdapter(t, server, nil)
	roll := provider.Rollup{
		Story:  "jira:RATE-2",
		Commit: "abc123",
		Criteria: []provider.CriterionStatus{
			{ID: "ac-1", Status: "pending"},
		},
	}
	err := a.PublishRollup(context.Background(), roll)
	if err == nil {
		t.Fatal("PublishRollup against a 429 response: want error, got nil")
	}
	if !errors.Is(err, provider.ErrUnavailable) {
		t.Fatalf("PublishRollup error = %v, want errors.Is(err, provider.ErrUnavailable)", err)
	}
}

// TestResolve_Timeout_StallingServerWithShortInjectedClient proves
// spec/forge-transport ac-3's deadline: a deliberately stalling handler
// paired with a SHORT injected client (never the 30s default — this test
// must not sleep 30s, co-1) still returns promptly as ErrUnavailable, the
// same sentinel a network failure gets (jira's classify wraps both
// identically).
func TestResolve_Timeout_StallingServerWithShortInjectedClient(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
	}))
	defer ts.Close()

	a := jira.New(jira.Config{
		BaseURL:     ts.URL,
		RollupField: testRollupField,
		HTTPClient:  &http.Client{Timeout: 50 * time.Millisecond},
	})

	start := time.Now()
	_, err := a.Resolve(context.Background(), provider.StoryRef("jira:SLOW-1"))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Resolve against a stalling server: want error, got nil")
	}
	if !errors.Is(err, provider.ErrUnavailable) {
		t.Fatalf("Resolve error = %v, want errors.Is(err, provider.ErrUnavailable)", err)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("Resolve took %v to fail, want it bounded by the short injected client's timeout", elapsed)
	}
}
