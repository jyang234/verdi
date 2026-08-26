package forge_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/forge/github"
	"github.com/jyang234/verdi/internal/forge/gitlab"
)

const (
	candidateA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	candidateB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// TestForgeApprovalContract_Static is the exact static AC-1 evidence producer.
func TestForgeApprovalContract_Static(t *testing.T) {
	t.Run("provider-neutral snapshot validates and sorts facts", func(t *testing.T) {
		approvals := []forge.Approval{
			approvalFixture("review-2", candidateB),
			approvalFixture("review-1", candidateA),
		}

		got, err := forge.NewApprovalSnapshot(
			"github",
			"acme/widgets",
			"17",
			candidateA,
			time.Date(2026, 8, 26, 12, 30, 0, 123, time.FixedZone("EDT", -4*60*60)),
			approvals,
		)
		if err != nil {
			t.Fatalf("NewApprovalSnapshot: %v", err)
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
		if got.Forge != "github" || got.Repository != "acme/widgets" || got.ChangeID != "17" || got.CandidateSHA != candidateA {
			t.Fatalf("identity = %+v", got)
		}
		if got.ObservedAt != "2026-08-26T16:30:00.000000123Z" {
			t.Fatalf("observed_at = %q", got.ObservedAt)
		}
		if len(got.ProviderSnapshotID) != len("sha256:")+64 || !strings.HasPrefix(got.ProviderSnapshotID, "sha256:") {
			t.Fatalf("provider_snapshot_id = %q", got.ProviderSnapshotID)
		}
		if got.Approvals == nil || len(got.Approvals) != 2 {
			t.Fatalf("approvals = %#v, want non-null length 2", got.Approvals)
		}
		if got.Approvals[0].ApprovalID != "review-1" || got.Approvals[1].ApprovalID != "review-2" {
			t.Fatalf("approval order = %#v", got.Approvals)
		}
		if got.Approvals[1].CandidateSHA != candidateB {
			t.Fatalf("wrong-head fact was lost: %#v", got.Approvals[1])
		}
	})

	t.Run("zero approvals still proves observation identity", func(t *testing.T) {
		got, err := forge.NewApprovalSnapshot("gitlab", "42", "9", candidateA, time.Date(2026, 8, 26, 16, 0, 0, 0, time.UTC), nil)
		if err != nil {
			t.Fatalf("NewApprovalSnapshot: %v", err)
		}
		if got.Approvals == nil || len(got.Approvals) != 0 {
			t.Fatalf("approvals = %#v, want non-null empty", got.Approvals)
		}
		if got.ProviderSnapshotID == "" || got.ObservedAt == "" {
			t.Fatalf("empty snapshot lacks identity: %+v", got)
		}
	})

	t.Run("validation fails closed", func(t *testing.T) {
		base, err := forge.NewApprovalSnapshot(
			"github", "acme/widgets", "17", candidateA,
			time.Date(2026, 8, 26, 16, 30, 0, 0, time.UTC),
			[]forge.Approval{approvalFixture("review-1", candidateA)},
		)
		if err != nil {
			t.Fatalf("fixture: %v", err)
		}

		tests := []struct {
			name   string
			mutate func(*forge.ApprovalSnapshot)
		}{
			{"missing forge", func(s *forge.ApprovalSnapshot) { s.Forge = "" }},
			{"missing repository", func(s *forge.ApprovalSnapshot) { s.Repository = "" }},
			{"missing change", func(s *forge.ApprovalSnapshot) { s.ChangeID = "" }},
			{"short candidate", func(s *forge.ApprovalSnapshot) { s.CandidateSHA = "abc" }},
			{"non-UTC observation", func(s *forge.ApprovalSnapshot) { s.ObservedAt = "2026-08-26T12:30:00-04:00" }},
			{"missing snapshot id", func(s *forge.ApprovalSnapshot) { s.ProviderSnapshotID = "" }},
			{"snapshot id does not match facts", func(s *forge.ApprovalSnapshot) { s.CandidateSHA = candidateB }},
			{"null approvals", func(s *forge.ApprovalSnapshot) { s.Approvals = nil }},
			{"duplicate approval id", func(s *forge.ApprovalSnapshot) { s.Approvals = append(s.Approvals, s.Approvals[0]) }},
			{"unknown state", func(s *forge.ApprovalSnapshot) { s.Approvals[0].State = forge.ApprovalState("pending") }},
			{"missing actor scheme", func(s *forge.ApprovalSnapshot) { s.Approvals[0].Actor.Scheme = "" }},
			{"missing stable actor", func(s *forge.ApprovalSnapshot) { s.Approvals[0].Actor.Subject = "" }},
			{"null witnesses", func(s *forge.ApprovalSnapshot) { s.Approvals[0].ProviderWitnesses = nil }},
			{"missing approval ref", func(s *forge.ApprovalSnapshot) { s.Approvals[0].ApprovalRef = "" }},
			{"non-UTC approval time", func(s *forge.ApprovalSnapshot) { s.Approvals[0].ApprovedAt = "2026-08-26T11:00:00-04:00" }},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := base
				got.Approvals = append([]forge.Approval(nil), base.Approvals...)
				got.Approvals[0].ProviderWitnesses = append([]forge.ProviderWitness(nil), base.Approvals[0].ProviderWitnesses...)
				tt.mutate(&got)
				if err := got.Validate(); err == nil {
					t.Fatalf("Validate(%s): want error, got nil", tt.name)
				}
			})
		}
	})

	t.Run("provider actor contract accepts only built-in canonical identities", func(t *testing.T) {
		valid := []struct {
			name      string
			forgeName string
			scheme    string
			subject   string
		}{
			{"github", "github", "github-user-id", "101"},
			{"gitlab", "gitlab", "gitlab-user-id", "9223372036854775807"},
		}
		for _, tt := range valid {
			t.Run("valid "+tt.name, func(t *testing.T) {
				approval := approvalFixture("review-1", candidateA)
				approval.Actor = forge.ProviderActor{Scheme: tt.scheme, Subject: tt.subject}
				if _, err := forge.NewApprovalSnapshot(tt.forgeName, "acme/widgets", "17", candidateA, fixedClock(), []forge.Approval{approval}); err != nil {
					t.Fatalf("NewApprovalSnapshot: %v", err)
				}
			})
		}

		schemes := []struct {
			name      string
			forgeName string
			scheme    string
		}{
			{"unknown forge", "bitbucket", "github-user-id"},
			{"unknown scheme", "github", "provider-user"},
			{"github with gitlab scheme", "github", "gitlab-user-id"},
			{"gitlab with github scheme", "gitlab", "github-user-id"},
		}
		for _, tt := range schemes {
			t.Run(tt.name, func(t *testing.T) {
				approval := approvalFixture("review-1", candidateA)
				approval.Actor.Scheme = tt.scheme
				_, err := forge.NewApprovalSnapshot(tt.forgeName, "acme/widgets", "17", candidateA, fixedClock(), []forge.Approval{approval})
				if err == nil || !strings.Contains(err.Error(), "actor.scheme") {
					t.Fatalf("NewApprovalSnapshot error = %v, want actor.scheme error", err)
				}
			})
		}

		for _, subject := range []string{"", "+1", "-1", "0", "01", " 1", "1 ", "9223372036854775808", "octocat", "Jane Doe"} {
			t.Run("invalid subject "+fmt.Sprintf("%q", subject), func(t *testing.T) {
				approval := approvalFixture("review-1", candidateA)
				approval.Actor.Subject = subject
				_, err := forge.NewApprovalSnapshot("github", "acme/widgets", "17", candidateA, fixedClock(), []forge.Approval{approval})
				if err == nil || !strings.Contains(err.Error(), "actor.subject") {
					t.Fatalf("NewApprovalSnapshot error = %v, want actor.subject error", err)
				}
			})
		}
	})

	t.Run("direct validation cannot bypass provider actor contract", func(t *testing.T) {
		snapshot, err := forge.NewApprovalSnapshot(
			"github", "acme/widgets", "17", candidateA, fixedClock(),
			[]forge.Approval{approvalFixture("review-1", candidateA)},
		)
		if err != nil {
			t.Fatalf("fixture: %v", err)
		}
		snapshot.Approvals[0].Actor.Subject = "renamed-login"
		if err := snapshot.Validate(); err == nil || !strings.Contains(err.Error(), "actor.subject") {
			t.Fatalf("Validate error = %v, want actor.subject error", err)
		}
	})

	t.Run("unknown approval state cannot decode", func(t *testing.T) {
		var state forge.ApprovalState
		if err := json.Unmarshal([]byte(`"pending"`), &state); err == nil {
			t.Fatal("json.Unmarshal unknown state: want error, got nil")
		}
	})

	t.Run("fake returns independent provider facts", func(t *testing.T) {
		seed, err := forge.NewApprovalSnapshot(
			"github", "acme/widgets", "17", candidateA,
			time.Date(2026, 8, 26, 16, 30, 0, 0, time.UTC),
			[]forge.Approval{approvalFixture("review-1", candidateA)},
		)
		if err != nil {
			t.Fatalf("fixture: %v", err)
		}
		f := fake.New()
		f.SeedApprovalSnapshot("17", seed)
		first, err := f.ListApprovals(context.Background(), "17")
		if err != nil {
			t.Fatalf("ListApprovals: %v", err)
		}
		first.Approvals[0].Actor.Subject = "mutated"
		second, err := f.ListApprovals(context.Background(), "17")
		if err != nil {
			t.Fatalf("ListApprovals second: %v", err)
		}
		if second.Approvals[0].Actor.Subject != "101" {
			t.Fatalf("fake returned aliased facts: %+v", second.Approvals[0])
		}
	})
}

// TestForgeApprovalContract_Behavioral is the exact behavioral AC-1 evidence producer.
func TestForgeApprovalContract_Behavioral(t *testing.T) {
	t.Run("github drains pages and retains active and dismissed head facts", func(t *testing.T) {
		var server *httptest.Server
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/repos/acme/widgets/pulls/17":
				writeJSON(t, w, `{"head":{"sha":"`+candidateA+`"}}`)
			case "/repos/acme/widgets/pulls/17/reviews":
				if r.URL.Query().Get("per_page") != "100" && r.URL.Query().Get("page") == "" {
					t.Errorf("first reviews query = %q", r.URL.RawQuery)
				}
				if r.URL.Query().Get("page") == "2" {
					writeJSON(t, w, `[`+githubReviewJSON(100, "PRR_active", "APPROVED", candidateA, 901, "renamed-login", "2026-08-26T11:00:00-04:00")+`]`)
					return
				}
				w.Header().Set("Link", "<"+server.URL+r.URL.Path+"?page=2>; rel=\"next\"")
				writeJSON(t, w, `[`+githubReviewJSON(200, "PRR_dismissed", "DISMISSED", candidateB, 902, "display-only", "2026-08-25T15:00:00Z")+`]`)
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		times := []time.Time{
			time.Date(2026, 8, 26, 12, 30, 0, 5, time.FixedZone("EDT", -4*60*60)),
			time.Date(2026, 8, 26, 17, 30, 0, 5, time.UTC),
		}
		var clockCall atomic.Int32
		a := github.New(github.Config{
			BaseURL: server.URL, Owner: "acme", Repo: "widgets", HTTPClient: server.Client(),
			Clock: func() time.Time { return times[int(clockCall.Add(1))-1] },
		})

		first, err := a.ListApprovals(context.Background(), "17")
		if err != nil {
			t.Fatalf("ListApprovals: %v", err)
		}
		second, err := a.ListApprovals(context.Background(), "17")
		if err != nil {
			t.Fatalf("ListApprovals second: %v", err)
		}
		if first.CandidateSHA != candidateA || first.ObservedAt != "2026-08-26T16:30:00.000000005Z" {
			t.Fatalf("snapshot binding = %+v", first)
		}
		if first.ProviderSnapshotID != second.ProviderSnapshotID || first.ObservedAt == second.ObservedAt {
			t.Fatalf("snapshot identity depends on observation time: first=%+v second=%+v", first, second)
		}
		if len(first.Approvals) != 2 || first.Approvals[0].ApprovalID != "100" || first.Approvals[1].ApprovalID != "200" {
			t.Fatalf("approvals = %+v", first.Approvals)
		}
		if first.Approvals[0].State != forge.ApprovalActive || first.Approvals[1].State != forge.ApprovalDismissed {
			t.Fatalf("states = %+v", first.Approvals)
		}
		if first.Approvals[0].Actor.Subject != "901" || first.Approvals[0].Actor.Subject == "renamed-login" {
			t.Fatalf("actor identity used display value: %+v", first.Approvals[0].Actor)
		}
		if first.Approvals[1].CandidateSHA != candidateB {
			t.Fatalf("dismissed wrong-head row lost candidate binding: %+v", first.Approvals[1])
		}
	})

	t.Run("github rejects head change during approval collection", func(t *testing.T) {
		var headCalls atomic.Int32
		var approvalsCalls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/repos/acme/widgets/pulls/17":
				candidate := candidateA
				if headCalls.Add(1) == 2 {
					candidate = candidateB
				}
				writeJSON(t, w, `{"head":{"sha":"`+candidate+`"}}`)
			case "/repos/acme/widgets/pulls/17/reviews":
				approvalsCalls.Add(1)
				writeJSON(t, w, `[]`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		a := github.New(github.Config{BaseURL: server.URL, Owner: "acme", Repo: "widgets", HTTPClient: server.Client(), Clock: fixedClock})
		_, err := a.ListApprovals(context.Background(), "17")
		if err == nil || !strings.Contains(err.Error(), "head changed during approval collection") {
			t.Fatalf("ListApprovals error = %v, want head-change operational error (head calls %d, approval calls %d)", err, headCalls.Load(), approvalsCalls.Load())
		}
		if headCalls.Load() != 2 || approvalsCalls.Load() != 1 {
			t.Fatalf("request counts: head=%d approvals=%d, want 2 and 1", headCalls.Load(), approvalsCalls.Load())
		}
	})

	t.Run("github rejects duplicate incomplete unknown and trailing facts", func(t *testing.T) {
		tests := []struct {
			name string
			body string
		}{
			{"duplicate ids", `[` + githubReviewJSON(100, "PRR_1", "APPROVED", candidateA, 901, "one", "2026-08-26T15:00:00Z") + `,` + githubReviewJSON(100, "PRR_2", "APPROVED", candidateA, 902, "two", "2026-08-26T15:01:00Z") + `]`},
			{"missing stable actor", `[{"id":100,"node_id":"PRR_1","state":"APPROVED","submitted_at":"2026-08-26T15:00:00Z","commit_id":"` + candidateA + `","user":{"id":0,"login":"name-only"}}]`},
			{"missing immutable ref", `[{"id":100,"node_id":"","state":"APPROVED","submitted_at":"2026-08-26T15:00:00Z","commit_id":"` + candidateA + `","user":{"id":901,"login":"one"}}]`},
			{"unknown state", `[{"id":100,"node_id":"PRR_1","state":"MYSTERY","submitted_at":"2026-08-26T15:00:00Z","commit_id":"` + candidateA + `","user":{"id":901,"login":"one"}}]`},
			{"unknown field", `[{"id":100,"node_id":"PRR_1","state":"APPROVED","submitted_at":"2026-08-26T15:00:00Z","commit_id":"` + candidateA + `","user":{"id":901,"login":"one"},"mystery":true}]`},
			{"trailing data", `[] true`},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				a, closeServer := githubFixture(t, tt.body)
				defer closeServer()
				if _, err := a.ListApprovals(context.Background(), "17"); err == nil {
					t.Fatalf("ListApprovals(%s): want error, got nil", tt.name)
				}
			})
		}
	})

	t.Run("github ignores closed known non-approval review states", func(t *testing.T) {
		a, closeServer := githubFixture(t, `[`+githubReviewJSON(100, "PRR_1", "COMMENTED", candidateA, 901, "one", "2026-08-26T15:00:00Z")+`]`)
		defer closeServer()
		got, err := a.ListApprovals(context.Background(), "17")
		if err != nil {
			t.Fatalf("ListApprovals: %v", err)
		}
		if got.Approvals == nil || len(got.Approvals) != 0 {
			t.Fatalf("non-approval reviews = %+v", got.Approvals)
		}
	})

	t.Run("github rejects malformed claimed continuation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/repos/acme/widgets/pulls/17":
				writeJSON(t, w, `{"head":{"sha":"`+candidateA+`"}}`)
			case "/repos/acme/widgets/pulls/17/reviews":
				w.Header().Set("Link", `not-a-link; rel="next"`)
				writeJSON(t, w, `[]`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()
		a := github.New(github.Config{BaseURL: server.URL, Owner: "acme", Repo: "widgets", HTTPClient: server.Client(), Clock: fixedClock})
		if _, err := a.ListApprovals(context.Background(), "17"); err == nil {
			t.Fatal("ListApprovals with malformed claimed continuation: want error, got nil")
		}
	})

	t.Run("github rejects multiple distinct next continuations", func(t *testing.T) {
		var reviewsCalls atomic.Int32
		var server *httptest.Server
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/repos/acme/widgets/pulls/17":
				writeJSON(t, w, `{"head":{"sha":"`+candidateA+`"}}`)
			case "/repos/acme/widgets/pulls/17/reviews":
				reviewsCalls.Add(1)
				if r.URL.Query().Get("page") == "" {
					w.Header().Set("Link", "<"+server.URL+r.URL.Path+"?page=2>; rel=\"next\", <"+server.URL+r.URL.Path+"?page=3>; rel=\"next\"")
				}
				writeJSON(t, w, `[]`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		a := github.New(github.Config{BaseURL: server.URL, Owner: "acme", Repo: "widgets", HTTPClient: server.Client(), Clock: fixedClock})
		_, err := a.ListApprovals(context.Background(), "17")
		if err == nil || !strings.Contains(err.Error(), "multiple distinct") {
			t.Fatalf("ListApprovals error = %v, want multiple-distinct-next error (review calls %d)", err, reviewsCalls.Load())
		}
		if reviewsCalls.Load() != 1 {
			t.Fatalf("review calls = %d, want 1", reviewsCalls.Load())
		}
	})

	t.Run("github rejects multi-page approval cycle", func(t *testing.T) {
		var reviewsCalls atomic.Int32
		var server *httptest.Server
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/repos/acme/widgets/pulls/17":
				writeJSON(t, w, `{"head":{"sha":"`+candidateA+`"}}`)
			case "/repos/acme/widgets/pulls/17/reviews":
				call := reviewsCalls.Add(1)
				if call > 2 {
					http.Error(w, "unexpected pagination revisit", http.StatusInternalServerError)
					return
				}
				if r.URL.Query().Get("page") == "2" {
					w.Header().Set("Link", "<"+server.URL+r.URL.Path+"?per_page=100>; rel=\"next\"")
				} else {
					w.Header().Set("Link", "<"+server.URL+r.URL.Path+"?page=2>; rel=\"next\"")
				}
				writeJSON(t, w, `[]`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		a := github.New(github.Config{BaseURL: server.URL, Owner: "acme", Repo: "widgets", HTTPClient: server.Client(), Clock: fixedClock})
		_, err := a.ListApprovals(context.Background(), "17")
		if err == nil || !strings.Contains(err.Error(), "approval pagination cycle detected") {
			t.Fatalf("ListApprovals error = %v, want multi-page cycle error (review calls %d)", err, reviewsCalls.Load())
		}
		if reviewsCalls.Load() != 2 {
			t.Fatalf("review calls = %d, want 2", reviewsCalls.Load())
		}
	})

	t.Run("gitlab current set is deterministic and removal is absence", func(t *testing.T) {
		var approvalsCall atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/projects/42/merge_requests/9":
				writeJSON(t, w, `{"sha":"`+candidateA+`"}`)
			case "/projects/42/merge_requests/9/approvals":
				if approvalsCall.Add(1) == 1 {
					writeJSON(t, w, `{"approved_by":[{"user":{"id":8,"username":"display-eight"},"approved_at":"2026-08-26T15:02:00Z"},{"user":{"id":7,"username":"renamed-seven"},"approved_at":"2026-08-26T11:01:00-04:00"}]}`)
					return
				}
				writeJSON(t, w, `{"approved_by":[{"user":{"id":7,"username":"different-display"},"approved_at":"2026-08-26T15:01:00Z"}]}`)
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		a := gitlab.New(gitlab.Config{
			BaseURL: server.URL, ProjectID: "42", HTTPClient: server.Client(),
			Clock: func() time.Time { return time.Date(2026, 8, 26, 16, 30, 0, 0, time.UTC) },
		})
		first, err := a.ListApprovals(context.Background(), "9")
		if err != nil {
			t.Fatalf("ListApprovals first: %v", err)
		}
		second, err := a.ListApprovals(context.Background(), "9")
		if err != nil {
			t.Fatalf("ListApprovals second: %v", err)
		}
		if len(first.Approvals) != 2 || len(second.Approvals) != 1 {
			t.Fatalf("current sets: first=%+v second=%+v", first.Approvals, second.Approvals)
		}
		if first.Approvals[0].ApprovalID > first.Approvals[1].ApprovalID {
			t.Fatalf("approvals are not deterministically ID-ordered: %+v", first.Approvals)
		}
		var actorSeven *forge.Approval
		for i := range first.Approvals {
			if first.Approvals[i].Actor.Subject == "7" {
				actorSeven = &first.Approvals[i]
			}
		}
		if actorSeven == nil || actorSeven.State != forge.ApprovalActive {
			t.Fatalf("stable actor 7 fact missing: %+v", first.Approvals)
		}
		if first.Approvals[0].ApprovalID == "" || first.Approvals[0].ApprovalID != first.Approvals[0].ApprovalRef {
			t.Fatalf("derived immutable identity = %+v", first.Approvals[0])
		}
		if first.ProviderSnapshotID == second.ProviderSnapshotID {
			t.Fatalf("removed approval did not change snapshot identity: %q", first.ProviderSnapshotID)
		}
		for _, approval := range second.Approvals {
			if approval.State == forge.ApprovalRevoked || approval.Actor.Subject == "8" {
				t.Fatalf("removed approval was fabricated as history: %+v", second.Approvals)
			}
		}
	})

	t.Run("gitlab rejects head change during approval collection", func(t *testing.T) {
		var headCalls atomic.Int32
		var approvalsCalls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/projects/42/merge_requests/9":
				candidate := candidateA
				if headCalls.Add(1) == 2 {
					candidate = candidateB
				}
				writeJSON(t, w, `{"sha":"`+candidate+`"}`)
			case "/projects/42/merge_requests/9/approvals":
				approvalsCalls.Add(1)
				writeJSON(t, w, `{"approved_by":[]}`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		a := gitlab.New(gitlab.Config{BaseURL: server.URL, ProjectID: "42", HTTPClient: server.Client(), Clock: fixedClock})
		_, err := a.ListApprovals(context.Background(), "9")
		if err == nil || !strings.Contains(err.Error(), "head changed during approval collection") {
			t.Fatalf("ListApprovals error = %v, want head-change operational error (head calls %d, approval calls %d)", err, headCalls.Load(), approvalsCalls.Load())
		}
		if headCalls.Load() != 2 || approvalsCalls.Load() != 1 {
			t.Fatalf("request counts: head=%d approvals=%d, want 2 and 1", headCalls.Load(), approvalsCalls.Load())
		}
	})

	t.Run("gitlab rejects continuation claim", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/projects/42/merge_requests/9":
				writeJSON(t, w, `{"sha":"`+candidateA+`"}`)
			case "/projects/42/merge_requests/9/approvals":
				w.Header().Set("X-Next-Page", "2")
				writeJSON(t, w, `{"approved_by":[]}`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()
		a := gitlab.New(gitlab.Config{BaseURL: server.URL, ProjectID: "42", HTTPClient: server.Client(), Clock: fixedClock})
		if _, err := a.ListApprovals(context.Background(), "9"); err == nil {
			t.Fatal("ListApprovals with continuation header: want error, got nil")
		}
	})

	t.Run("gitlab rejects incomplete unknown and trailing facts", func(t *testing.T) {
		tests := []struct {
			name string
			body string
		}{
			{"missing stable actor", `{"approved_by":[{"user":{"id":0,"username":"name-only"},"approved_at":"2026-08-26T15:00:00Z"}]}`},
			{"missing approval time", `{"approved_by":[{"user":{"id":7,"username":"seven"},"approved_at":""}]}`},
			{"unknown field", `{"approved_by":[],"mystery":true}`},
			{"trailing data", `{"approved_by":[]} true`},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				a, closeServer := gitlabFixture(t, tt.body, "")
				defer closeServer()
				if _, err := a.ListApprovals(context.Background(), "9"); err == nil {
					t.Fatalf("ListApprovals(%s): want error, got nil", tt.name)
				}
			})
		}
	})
}

func approvalFixture(id, candidate string) forge.Approval {
	return forge.Approval{
		ApprovalID:        id,
		ApprovalRef:       "provider-ref-" + id,
		State:             forge.ApprovalActive,
		ApprovedAt:        "2026-08-26T15:00:00Z",
		UpdatedAt:         "2026-08-26T15:00:00Z",
		CandidateSHA:      candidate,
		Actor:             forge.ProviderActor{Scheme: "github-user-id", Subject: "101"},
		ProviderWitnesses: []forge.ProviderWitness{{Name: "review_id", Value: id}},
	}
}

func githubReviewJSON(id int, ref, state, commit string, userID int, login, submittedAt string) string {
	return fmt.Sprintf(`{"id":%d,"node_id":%q,"state":%q,"submitted_at":%q,"commit_id":%q,"user":{"id":%d,"login":%q}}`, id, ref, state, submittedAt, commit, userID, login)
}

func githubFixture(t *testing.T, reviewsBody string) (*github.Adapter, func()) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/widgets/pulls/17":
			writeJSON(t, w, `{"head":{"sha":"`+candidateA+`"}}`)
		case "/repos/acme/widgets/pulls/17/reviews":
			writeJSON(t, w, reviewsBody)
		default:
			http.NotFound(w, r)
		}
	}))
	a := github.New(github.Config{BaseURL: server.URL, Owner: "acme", Repo: "widgets", HTTPClient: server.Client(), Clock: fixedClock})
	return a, server.Close
}

func gitlabFixture(t *testing.T, approvalsBody, continuation string) (*gitlab.Adapter, func()) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/42/merge_requests/9":
			writeJSON(t, w, `{"sha":"`+candidateA+`"}`)
		case "/projects/42/merge_requests/9/approvals":
			if continuation != "" {
				w.Header().Set("X-Next-Page", continuation)
			}
			writeJSON(t, w, approvalsBody)
		default:
			http.NotFound(w, r)
		}
	}))
	a := gitlab.New(gitlab.Config{BaseURL: server.URL, ProjectID: "42", HTTPClient: server.Client(), Clock: fixedClock})
	return a, server.Close
}

func fixedClock() time.Time {
	return time.Date(2026, 8, 26, 16, 30, 0, 0, time.UTC)
}

func writeJSON(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte(body)); err != nil {
		t.Errorf("write response: %v", err)
	}
}
