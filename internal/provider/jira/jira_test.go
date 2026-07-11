package jira_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/provider"
	"github.com/OWNER/verdi/internal/provider/jira"
	"github.com/OWNER/verdi/internal/provider/jira/jiratest"
)

func newAdapter(t *testing.T, server *jiratest.Server, getenv func(string) string) *jira.Adapter {
	t.Helper()
	return jira.New(jira.Config{
		BaseURL:     server.URL,
		RollupField: testRollupField,
		Token:       "test-token",
		HTTPClient:  server.Client(),
		Getenv:      getenv,
	})
}

// TestResolve_Mapping proves Resolve maps key/summary/status/URL exactly
// as 04 §Jira adapter describes: "GET /rest/api/3/issue/{key} -> key,
// summary, status, URL".
func TestResolve_Mapping(t *testing.T) {
	server := jiratest.NewServer(testRollupField)
	t.Cleanup(server.Close)
	server.SeedIssue("LOAN-1482", "Stale decline handling", "In Progress", "https://example.atlassian.net/rest/api/3/issue/10002")

	a := newAdapter(t, server, nil)
	got, err := a.Resolve(context.Background(), provider.StoryRef("jira:LOAN-1482"))
	if err != nil {
		t.Fatalf("Resolve error = %v, want nil", err)
	}
	want := provider.Story{
		Ref:    "jira:LOAN-1482",
		Title:  "Stale decline handling",
		Status: "In Progress",
		URL:    "https://example.atlassian.net/rest/api/3/issue/10002",
	}
	if got != want {
		t.Fatalf("Resolve = %+v, want %+v", got, want)
	}
}

// TestResolve_FailureTable exercises every row of 04's failure table that
// applies to Resolve: NotFound, Unauthorized (401 and 403 both), and
// Unavailable (5xx and a timed-out/canceled context).
func TestResolve_FailureTable(t *testing.T) {
	tests := []struct {
		name         string
		arrange      func(server *jiratest.Server, key string)
		ctx          func() context.Context
		wantSentinel error
	}{
		{
			name:         "404 not found",
			arrange:      func(s *jiratest.Server, key string) { s.SeedNotFound(key) },
			wantSentinel: provider.ErrNotFound,
		},
		{
			name:         "401 unauthorized",
			arrange:      func(s *jiratest.Server, key string) { s.ForceStatus(key, http.StatusUnauthorized) },
			wantSentinel: provider.ErrUnauthorized,
		},
		{
			name:         "403 forbidden",
			arrange:      func(s *jiratest.Server, key string) { s.ForceStatus(key, http.StatusForbidden) },
			wantSentinel: provider.ErrUnauthorized,
		},
		{
			name:         "500 internal server error",
			arrange:      func(s *jiratest.Server, key string) { s.ForceStatus(key, http.StatusInternalServerError) },
			wantSentinel: provider.ErrUnavailable,
		},
		{
			name:         "503 service unavailable",
			arrange:      func(s *jiratest.Server, key string) { s.ForceStatus(key, http.StatusServiceUnavailable) },
			wantSentinel: provider.ErrUnavailable,
		},
		{
			name:    "context already canceled reads as unavailable/timeout",
			arrange: func(s *jiratest.Server, key string) {},
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantSentinel: provider.ErrUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := jiratest.NewServer(testRollupField)
			t.Cleanup(server.Close)
			tt.arrange(server, "FAIL-1")

			a := newAdapter(t, server, nil)
			ctx := context.Background()
			if tt.ctx != nil {
				ctx = tt.ctx()
			}
			_, err := a.Resolve(ctx, provider.StoryRef("jira:FAIL-1"))
			if err == nil {
				t.Fatalf("Resolve error = nil, want %v", tt.wantSentinel)
			}
			if !errors.Is(err, tt.wantSentinel) {
				t.Fatalf("Resolve error = %v, want errors.Is(err, %v)", err, tt.wantSentinel)
			}
		})
	}
}

// TestPublishRollup_FailureTable exercises the same failure taxonomy
// through PublishRollup's read-before-write GET.
func TestPublishRollup_FailureTable(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		wantSentinel error
	}{
		{"401 unauthorized", http.StatusUnauthorized, provider.ErrUnauthorized},
		{"403 forbidden", http.StatusForbidden, provider.ErrUnauthorized},
		{"500 internal server error", http.StatusInternalServerError, provider.ErrUnavailable},
		{"503 service unavailable", http.StatusServiceUnavailable, provider.ErrUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := jiratest.NewServer(testRollupField)
			t.Cleanup(server.Close)
			server.ForceStatus("FAIL-2", tt.status)

			a := newAdapter(t, server, nil)
			roll := provider.Rollup{
				Story:  "jira:FAIL-2",
				Commit: "abc123",
				Criteria: []provider.CriterionStatus{
					{ID: "ac-1", Status: "pending"},
				},
			}
			err := a.PublishRollup(context.Background(), roll)
			if err == nil {
				t.Fatalf("PublishRollup error = nil, want %v", tt.wantSentinel)
			}
			if !errors.Is(err, tt.wantSentinel) {
				t.Fatalf("PublishRollup error = %v, want errors.Is(err, %v)", err, tt.wantSentinel)
			}
		})
	}

	t.Run("issue does not exist (404) fails the publish", func(t *testing.T) {
		server := jiratest.NewServer(testRollupField)
		t.Cleanup(server.Close)
		server.SeedNotFound("MISSING-1")

		a := newAdapter(t, server, nil)
		roll := provider.Rollup{Story: "jira:MISSING-1", Commit: "c1", Criteria: []provider.CriterionStatus{{ID: "ac-1", Status: "pending"}}}
		err := a.PublishRollup(context.Background(), roll)
		if !errors.Is(err, provider.ErrNotFound) {
			t.Fatalf("PublishRollup error = %v, want errors.Is(err, ErrNotFound)", err)
		}
	})
}

// TestPublishRollup_Idempotency_HTTPLevel proves republishing the same
// (story, commit) at the HTTP level leaves the machine field in one state
// and does not duplicate the comment, distinct from the abstract
// contract-suite assertion in contract_test.go.
func TestPublishRollup_Idempotency_HTTPLevel(t *testing.T) {
	server := jiratest.NewServer(testRollupField)
	t.Cleanup(server.Close)
	a := newAdapter(t, server, func(string) string { return "" })

	roll := provider.Rollup{
		Story:  "jira:IDEM-1",
		Commit: "commit-a",
		Criteria: []provider.CriterionStatus{
			{ID: "ac-1", Status: "evidenced", Summary: "covered"},
		},
		Eligible: true,
	}
	ctx := context.Background()
	if err := a.PublishRollup(ctx, roll); err != nil {
		t.Fatalf("first PublishRollup error = %v", err)
	}
	if err := a.PublishRollup(ctx, roll); err != nil {
		t.Fatalf("second PublishRollup (same story+commit) error = %v", err)
	}

	if got := server.PublishedCommitCount("IDEM-1"); got != 1 {
		t.Fatalf("PublishedCommitCount = %d after two identical publishes, want 1", got)
	}
	if got := server.CommentCount("IDEM-1"); got != 1 {
		t.Fatalf("CommentCount = %d after two identical publishes, want 1 (only the first publish fires — I-26 — the second is unchanged)", got)
	}

	field, ok := server.FieldValue("IDEM-1")
	if !ok {
		t.Fatal("FieldValue not set after publish")
	}
	payload, err := decodeRollupPayload(field)
	if err != nil {
		t.Fatalf("decoding field: %v", err)
	}
	if payload.Commit != "commit-a" || !payload.Eligible {
		t.Fatalf("field payload = %+v, want commit=commit-a eligible=true", payload)
	}

	// A different commit is a distinct record.
	roll2 := roll
	roll2.Commit = "commit-b"
	if err := a.PublishRollup(ctx, roll2); err != nil {
		t.Fatalf("PublishRollup (new commit) error = %v", err)
	}
	if got := server.PublishedCommitCount("IDEM-1"); got != 2 {
		t.Fatalf("PublishedCommitCount = %d after publishing a new commit, want 2", got)
	}
}

// TestPublishRollup_CommentOnlyOnChange_HTTPLevel proves the human comment
// fires on the first publish (I-26) and again only when an AC status
// actually changes, at the HTTP level (inspecting the mock server's
// recorded comment bodies directly).
func TestPublishRollup_CommentOnlyOnChange_HTTPLevel(t *testing.T) {
	server := jiratest.NewServer(testRollupField)
	t.Cleanup(server.Close)
	a := newAdapter(t, server, func(string) string { return "" })

	ctx := context.Background()
	base := provider.Rollup{
		Story:  "jira:CMT-1",
		Commit: "c1",
		Criteria: []provider.CriterionStatus{
			{ID: "ac-1", Status: "pending", Summary: "no evidence yet"},
		},
		Eligible: false,
	}
	if err := a.PublishRollup(ctx, base); err != nil {
		t.Fatalf("first PublishRollup error = %v", err)
	}
	if got := server.CommentCount("CMT-1"); got != 1 {
		t.Fatalf("CommentCount after first publish = %d, want 1 (I-26: first publish always fires)", got)
	}

	unchanged := base
	unchanged.Commit = "c2"
	if err := a.PublishRollup(ctx, unchanged); err != nil {
		t.Fatalf("unchanged republish error = %v", err)
	}
	if got := server.CommentCount("CMT-1"); got != 1 {
		t.Fatalf("CommentCount after unchanged republish = %d, want 1 (no comment on unchanged statuses)", got)
	}

	changed := unchanged
	changed.Commit = "c3"
	changed.Criteria = []provider.CriterionStatus{
		{ID: "ac-1", Status: "evidenced", Summary: "now covered"},
	}
	changed.Eligible = true
	if err := a.PublishRollup(ctx, changed); err != nil {
		t.Fatalf("changed republish error = %v", err)
	}
	if got := server.CommentCount("CMT-1"); got != 2 {
		t.Fatalf("CommentCount after changed republish = %d, want 2", got)
	}

	comments := server.Comments("CMT-1")
	if len(comments) != 2 {
		t.Fatalf("len(Comments) = %d, want 2", len(comments))
	}
	if !strings.Contains(comments[0], "ac-1") || !strings.Contains(comments[0], "pending") {
		t.Fatalf("first comment %q does not mention the initial ac-1/pending state", comments[0])
	}
	if !strings.Contains(comments[1], "ac-1") || !strings.Contains(comments[1], "evidenced") {
		t.Fatalf("second comment %q does not mention the changed ac-1/evidenced state", comments[1])
	}
}

// TestPublishRollup_CommentIncludesCILink proves the human comment
// includes an MR/pipeline link when CI env vars are present (04 §Jira
// adapter: "plus a link to the MR/pipeline"), and omits it when absent.
func TestPublishRollup_CommentIncludesCILink(t *testing.T) {
	t.Run("link present when CI env is set", func(t *testing.T) {
		server := jiratest.NewServer(testRollupField)
		t.Cleanup(server.Close)
		env := map[string]string{
			"CI_PROJECT_URL":       "https://gitlab.example/group/proj",
			"CI_MERGE_REQUEST_IID": "42",
		}
		a := newAdapter(t, server, func(k string) string { return env[k] })

		roll := provider.Rollup{Story: "jira:LINK-1", Commit: "c1", Criteria: []provider.CriterionStatus{{ID: "ac-1", Status: "pending"}}}
		if err := a.PublishRollup(context.Background(), roll); err != nil {
			t.Fatalf("PublishRollup error = %v", err)
		}
		comments := server.Comments("LINK-1")
		if len(comments) != 1 || !strings.Contains(comments[0], "https://gitlab.example/group/proj/-/merge_requests/42") {
			t.Fatalf("comments = %v, want one comment containing the MR URL", comments)
		}
	})

	t.Run("link omitted when CI env is absent", func(t *testing.T) {
		server := jiratest.NewServer(testRollupField)
		t.Cleanup(server.Close)
		a := newAdapter(t, server, func(string) string { return "" })

		roll := provider.Rollup{Story: "jira:LINK-2", Commit: "c1", Criteria: []provider.CriterionStatus{{ID: "ac-1", Status: "pending"}}}
		if err := a.PublishRollup(context.Background(), roll); err != nil {
			t.Fatalf("PublishRollup error = %v", err)
		}
		comments := server.Comments("LINK-2")
		if len(comments) != 1 || strings.Contains(comments[0], "MR/pipeline") {
			t.Fatalf("comments = %v, want one comment with no MR/pipeline line", comments)
		}
	})
}

// TestCILink is a table-driven unit test of the MR/pipeline link
// preference order this package's ciLink implements indirectly (exercised
// here through the comment it produces, since ciLink itself is
// unexported).
func TestCILink(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string // substring expected in the comment, or "" for no link line
	}{
		{
			name: "gitlab MR preferred over pipeline",
			env: map[string]string{
				"CI_PROJECT_URL":       "https://gitlab.example/g/p",
				"CI_MERGE_REQUEST_IID": "7",
				"CI_PIPELINE_URL":      "https://gitlab.example/g/p/-/pipelines/99",
			},
			want: "https://gitlab.example/g/p/-/merge_requests/7",
		},
		{
			name: "gitlab pipeline when no MR",
			env: map[string]string{
				"CI_PIPELINE_URL": "https://gitlab.example/g/p/-/pipelines/99",
			},
			want: "https://gitlab.example/g/p/-/pipelines/99",
		},
		{
			name: "github actions run URL",
			env: map[string]string{
				"GITHUB_SERVER_URL": "https://github.com",
				"GITHUB_REPOSITORY": "OWNER/verdi",
				"GITHUB_RUN_ID":     "123456",
			},
			want: "https://github.com/OWNER/verdi/actions/runs/123456",
		},
		{
			name: "nothing present",
			env:  map[string]string{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := jiratest.NewServer(testRollupField)
			t.Cleanup(server.Close)
			a := newAdapter(t, server, func(k string) string { return tt.env[k] })

			roll := provider.Rollup{Story: "jira:CILINK-1", Commit: "c1", Criteria: []provider.CriterionStatus{{ID: "ac-1", Status: "pending"}}}
			if err := a.PublishRollup(context.Background(), roll); err != nil {
				t.Fatalf("PublishRollup error = %v", err)
			}
			comments := server.Comments("CILINK-1")
			if len(comments) != 1 {
				t.Fatalf("len(Comments) = %d, want 1", len(comments))
			}
			if tt.want == "" {
				if strings.Contains(comments[0], "MR/pipeline") {
					t.Fatalf("comment = %q, want no MR/pipeline line", comments[0])
				}
				return
			}
			if !strings.Contains(comments[0], tt.want) {
				t.Fatalf("comment = %q, want it to contain %q", comments[0], tt.want)
			}
		})
	}
}

// TestNew_DefaultsGetenvAndClient proves New fills in Config's zero-value
// fields (HTTPClient, Getenv) rather than leaving the Adapter unusable. It
// points at a closed local loopback port (connection refused immediately,
// no DNS lookup, no live network — CLAUDE.md) rather than a real host: a
// nil HTTPClient/Getenv would panic on first use, so an ordinary wrapped
// connection error rather than a nil-pointer panic proves both were
// defaulted.
func TestNew_DefaultsGetenvAndClient(t *testing.T) {
	a := jira.New(jira.Config{BaseURL: "http://127.0.0.1:1", RollupField: "customfield_1"})
	_, err := a.Resolve(context.Background(), "jira:X-1")
	if !errors.Is(err, provider.ErrUnavailable) {
		t.Fatalf("Resolve against a closed local port = %v, want errors.Is(err, ErrUnavailable)", err)
	}
}
