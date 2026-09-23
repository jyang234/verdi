package forge_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/forge/github"
	"github.com/jyang234/verdi/internal/forge/gitlab"
)

// --- fixture bodies for a healthy GitHub environment-review call:
// run 555, attempt 1, environment "close", gated job "close". ---

const (
	envReviewRunURL     = "https://github.com/acme/widgets/actions/runs/555"
	envReviewRunHealthy = `{"id":555,"run_attempt":1,"head_sha":"` + candidateA + `","html_url":"` + envReviewRunURL + `"}`
	envReviewEnvHealthy = `{"id":9,"name":"close"}`
)

func envReviewJobsBody(jobs ...string) string {
	return `{"total_count":` + strconv.Itoa(len(jobs)) + `,"jobs":[` + strings.Join(jobs, ",") + `]}`
}

func envReviewJobJSON(name, createdAt, startedAt string) string {
	fields := []string{`"name":"` + name + `"`}
	if createdAt != "" {
		fields = append(fields, `"created_at":"`+createdAt+`"`)
	}
	if startedAt != "" {
		fields = append(fields, `"started_at":"`+startedAt+`"`)
	}
	return "{" + strings.Join(fields, ",") + "}"
}

func envReviewHistoryEntryJSON(state string, userID int64, envID int64, envName string) string {
	return fmt.Sprintf(`{"state":%q,"comment":"","environments":[{"id":%d,"name":%q}],"user":{"id":%d}}`, state, envID, envName, userID)
}

func envReviewQuery() forge.EnvironmentReviewQuery {
	return forge.EnvironmentReviewQuery{RunID: "555", RunAttempt: 1, EnvironmentName: "close", GatedJobName: "close"}
}

// environmentReviewServer serves the 4 fixed routes an EnvironmentReview
// call reads (run attempt, jobs, environment, review history), keyed by
// run 555 attempt 1 / environment "close" — the healthy fixtures above use
// these exact ids.
func environmentReviewServer(t *testing.T, runBody, jobsBody, envBody, historyBody string) (*github.Adapter, func()) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/widgets/actions/runs/555/attempts/1":
			writeJSON(t, w, runBody)
		case "/repos/acme/widgets/actions/runs/555/attempts/1/jobs":
			writeJSON(t, w, jobsBody)
		case "/repos/acme/widgets/environments/close":
			writeJSON(t, w, envBody)
		case "/repos/acme/widgets/actions/runs/555/approvals":
			writeJSON(t, w, historyBody)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}))
	a := github.New(github.Config{BaseURL: server.URL, Owner: "acme", Repo: "widgets", HTTPClient: server.Client(), Clock: fixedClock})
	return a, server.Close
}

func hasWitness(witnesses []forge.ProviderWitness, name string) bool {
	for _, w := range witnesses {
		if w.Name == name {
			return true
		}
	}
	return false
}

func witnessValue(witnesses []forge.ProviderWitness, name string) (string, bool) {
	for _, w := range witnesses {
		if w.Name == name {
			return w.Value, true
		}
	}
	return "", false
}

func disclosuresContain(disclosures []string, substr string) bool {
	for _, d := range disclosures {
		if strings.Contains(d, substr) {
			return true
		}
	}
	return false
}

// TestEnvironmentReviewApprovalContract_Static is the exact v2 ac-4 static
// obligation's producer: it proves NewEnvironmentReviewFacts and
// NormalizeEnvironmentReview's pure mapping without any HTTP involved.
func TestEnvironmentReviewApprovalContract_Static(t *testing.T) {
	t.Run("unknown provider state cannot decode", func(t *testing.T) {
		var state forge.EnvironmentReviewState
		if err := json.Unmarshal([]byte(`"commented"`), &state); err == nil {
			t.Fatal("json.Unmarshal unknown state: want error, got nil")
		}
	})

	t.Run("known provider states decode", func(t *testing.T) {
		for _, raw := range []string{"approved", "rejected", "pending"} {
			var state forge.EnvironmentReviewState
			if err := json.Unmarshal([]byte(`"`+raw+`"`), &state); err != nil {
				t.Fatalf("json.Unmarshal %q: %v", raw, err)
			}
			if string(state) != raw {
				t.Fatalf("decoded state = %q, want %q", state, raw)
			}
		}
	})

	baseSupported := func() forge.EnvironmentReviewFacts {
		return forge.EnvironmentReviewFacts{
			Supported: true, Repository: "acme/widgets", RunID: "555", RunAttempt: 1,
			RunHeadSHA: candidateA, RunURL: envReviewRunURL,
			EnvironmentID: "9", EnvironmentName: "close",
			Reviews: []forge.EnvironmentReviewRow{},
		}
	}

	t.Run("supported facts require every composite identity component", func(t *testing.T) {
		tests := []struct {
			name   string
			mutate func(*forge.EnvironmentReviewFacts)
		}{
			{"missing repository", func(f *forge.EnvironmentReviewFacts) { f.Repository = "" }},
			{"missing run id", func(f *forge.EnvironmentReviewFacts) { f.RunID = "" }},
			{"zero run attempt", func(f *forge.EnvironmentReviewFacts) { f.RunAttempt = 0 }},
			{"negative run attempt", func(f *forge.EnvironmentReviewFacts) { f.RunAttempt = -1 }},
			{"missing run head sha", func(f *forge.EnvironmentReviewFacts) { f.RunHeadSHA = "" }},
			{"missing run url", func(f *forge.EnvironmentReviewFacts) { f.RunURL = "" }},
			{"missing environment id", func(f *forge.EnvironmentReviewFacts) { f.EnvironmentID = "" }},
			{"missing environment name", func(f *forge.EnvironmentReviewFacts) { f.EnvironmentName = "" }},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				draft := baseSupported()
				tt.mutate(&draft)
				if _, err := forge.NewEnvironmentReviewFacts(draft, fixedClock()); err == nil {
					t.Fatalf("NewEnvironmentReviewFacts(%s): want error, got nil", tt.name)
				}
			})
		}
	})

	t.Run("unsupported facts must carry no run job environment or review data", func(t *testing.T) {
		draft := forge.EnvironmentReviewFacts{Supported: false, UnsupportedReason: "gitlab: unsupported", Repository: "42", RunID: "1"}
		if _, err := forge.NewEnvironmentReviewFacts(draft, fixedClock()); err == nil {
			t.Fatal("NewEnvironmentReviewFacts with unsupported facts carrying run data: want error, got nil")
		}
	})

	t.Run("unsupported facts require a disclosed reason", func(t *testing.T) {
		draft := forge.EnvironmentReviewFacts{Supported: false, Repository: "42"}
		if _, err := forge.NewEnvironmentReviewFacts(draft, fixedClock()); err == nil {
			t.Fatal("NewEnvironmentReviewFacts with no unsupported_reason: want error, got nil")
		}
	})

	t.Run("direct validation cannot bypass the provider state contract", func(t *testing.T) {
		draft := baseSupported()
		draft.Reviews = []forge.EnvironmentReviewRow{{
			ReviewerActor: forge.ProviderActor{Scheme: "github-user-id", Subject: "901"},
			ProviderState: forge.EnvironmentReviewState("mystery"),
		}}
		if _, err := forge.NewEnvironmentReviewFacts(draft, fixedClock()); err == nil {
			t.Fatal("NewEnvironmentReviewFacts with an out-of-vocabulary provider state: want error, got nil")
		}
	})

	t.Run("duplicate reviewer state pair rejected", func(t *testing.T) {
		draft := baseSupported()
		row := forge.EnvironmentReviewRow{ReviewerActor: forge.ProviderActor{Scheme: "github-user-id", Subject: "901"}, ProviderState: forge.EnvironmentReviewApproved}
		draft.Reviews = []forge.EnvironmentReviewRow{row, row}
		if _, err := forge.NewEnvironmentReviewFacts(draft, fixedClock()); err == nil {
			t.Fatal("NewEnvironmentReviewFacts with a duplicate (reviewer, state) pair: want error, got nil")
		}
	})

	approvedRow := func(subject string) forge.EnvironmentReviewRow {
		return forge.EnvironmentReviewRow{ReviewerActor: forge.ProviderActor{Scheme: "github-user-id", Subject: subject}, ProviderState: forge.EnvironmentReviewApproved}
	}

	t.Run("normalize: unsupported forge yields no rows with a disclosure", func(t *testing.T) {
		facts, err := forge.NewEnvironmentReviewFacts(forge.EnvironmentReviewFacts{Supported: false, UnsupportedReason: "gitlab: unsupported", Repository: "42"}, fixedClock())
		if err != nil {
			t.Fatalf("fixture: %v", err)
		}
		rows, disclosures := forge.NormalizeEnvironmentReview(facts)
		if len(rows) != 0 {
			t.Fatalf("rows = %+v, want none", rows)
		}
		if !disclosuresContain(disclosures, "unsupported-forge") {
			t.Fatalf("disclosures = %v, want an unsupported-forge witness", disclosures)
		}
	})

	t.Run("normalize: any rerun yields no rows with a disclosure regardless of reviews", func(t *testing.T) {
		draft := baseSupported()
		draft.RunAttempt = 2
		draft.Reviews = []forge.EnvironmentReviewRow{approvedRow("901")}
		facts, err := forge.NewEnvironmentReviewFacts(draft, fixedClock())
		if err != nil {
			t.Fatalf("fixture: %v", err)
		}
		rows, disclosures := forge.NormalizeEnvironmentReview(facts)
		if len(rows) != 0 {
			t.Fatalf("rows = %+v, want none for a rerun even with an approved review present", rows)
		}
		if !disclosuresContain(disclosures, "rerun-not-honored") {
			t.Fatalf("disclosures = %v, want a rerun-not-honored witness", disclosures)
		}
	})

	t.Run("normalize: rejected pending and absent reviews yield no rows", func(t *testing.T) {
		tests := []struct {
			name    string
			reviews []forge.EnvironmentReviewRow
		}{
			{"rejected", []forge.EnvironmentReviewRow{{ReviewerActor: forge.ProviderActor{Scheme: "github-user-id", Subject: "901"}, ProviderState: forge.EnvironmentReviewRejected}}},
			{"pending", []forge.EnvironmentReviewRow{{ReviewerActor: forge.ProviderActor{Scheme: "github-user-id", Subject: "901"}, ProviderState: forge.EnvironmentReviewPending}}},
			{"absent", nil},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				draft := baseSupported()
				draft.Reviews = tt.reviews
				draft.GatedJobFound = true
				draft.GatedJobCreatedAt = "2026-08-26T15:00:00Z"
				facts, err := forge.NewEnvironmentReviewFacts(draft, fixedClock())
				if err != nil {
					t.Fatalf("fixture: %v", err)
				}
				rows, _ := forge.NormalizeEnvironmentReview(facts)
				if len(rows) != 0 {
					t.Fatalf("rows = %+v, want none", rows)
				}
			})
		}
	})

	t.Run("normalize: missing creation stamp fails closed with a disclosure, never a row", func(t *testing.T) {
		tests := []struct {
			name          string
			gatedJobFound bool
			wantSubstr    string
		}{
			{"job found, no creation stamp", true, "gated job creation stamp unavailable"},
			{"job not found", false, "gated job not found"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				draft := baseSupported()
				draft.Reviews = []forge.EnvironmentReviewRow{approvedRow("901")}
				draft.GatedJobFound = tt.gatedJobFound
				facts, err := forge.NewEnvironmentReviewFacts(draft, fixedClock())
				if err != nil {
					t.Fatalf("fixture: %v", err)
				}
				rows, disclosures := forge.NormalizeEnvironmentReview(facts)
				if len(rows) != 0 {
					t.Fatalf("rows = %+v, want none (fail closed)", rows)
				}
				if !disclosuresContain(disclosures, "creation-stamp-unavailable") || !disclosuresContain(disclosures, tt.wantSubstr) {
					t.Fatalf("disclosures = %v, want creation-stamp-unavailable containing %q", disclosures, tt.wantSubstr)
				}
			})
		}
	})

	t.Run("normalize: one approved review at attempt 1 with a creation stamp produces exactly one fully disclosed row", func(t *testing.T) {
		selfReview := true
		draft := baseSupported()
		draft.Reviews = []forge.EnvironmentReviewRow{approvedRow("901")}
		draft.GatedJobFound = true
		draft.GatedJobCreatedAt = "2026-08-26T09:00:00Z"
		draft.GatedJobStartedAt = "2026-08-26T15:00:00Z"
		draft.EnvironmentPreventSelfReview = &selfReview
		facts, err := forge.NewEnvironmentReviewFacts(draft, fixedClock())
		if err != nil {
			t.Fatalf("fixture: %v", err)
		}

		rows, disclosures := forge.NormalizeEnvironmentReview(facts)
		if disclosures != nil {
			t.Fatalf("disclosures = %v, want nil for a clean approved row", disclosures)
		}
		if len(rows) != 1 {
			t.Fatalf("rows = %+v, want exactly 1", rows)
		}
		row := rows[0]

		wantID := "github-environment-review:acme/widgets:555:1:9:901"
		if row.ApprovalID != wantID {
			t.Fatalf("ApprovalID = %q, want %q", row.ApprovalID, wantID)
		}
		if !strings.Contains(row.ApprovalRef, envReviewRunURL) || !strings.Contains(row.ApprovalRef, "close") {
			t.Fatalf("ApprovalRef = %q, want it to name the run URL and environment", row.ApprovalRef)
		}
		if row.State != forge.ApprovalActive {
			t.Fatalf("State = %q, want active", row.State)
		}
		if row.ApprovedAt != "2026-08-26T09:00:00Z" || row.UpdatedAt != row.ApprovedAt {
			t.Fatalf("ApprovedAt/UpdatedAt = %q/%q, want the creation stamp for both", row.ApprovedAt, row.UpdatedAt)
		}
		if row.ApprovedAt == draft.GatedJobStartedAt {
			t.Fatalf("ApprovedAt equals the job's start stamp, want the creation stamp (a conservative lower bound)")
		}
		if row.CandidateSHA != candidateA {
			t.Fatalf("CandidateSHA = %q, want the run head %q", row.CandidateSHA, candidateA)
		}
		if row.Actor != (forge.ProviderActor{Scheme: "github-user-id", Subject: "901"}) {
			t.Fatalf("Actor = %+v", row.Actor)
		}

		for _, name := range []string{
			"provider_state", "actor_user_id", "run_id", "run_attempt", "run_head_sha", "run_url",
			"environment_id", "environment_name", "gated_job_created_at", "gated_job_started_at",
			"approval_id_derivation", "approved_at_derivation", "state_derivation", "environment_prevent_self_review",
		} {
			if !hasWitness(row.ProviderWitnesses, name) {
				t.Errorf("missing derived-field disclosure %q in %+v", name, row.ProviderWitnesses)
			}
		}
		if v, _ := witnessValue(row.ProviderWitnesses, "provider_state"); v != "approved" {
			t.Errorf("provider_state witness = %q, want %q", v, "approved")
		}
		if v, _ := witnessValue(row.ProviderWitnesses, "gated_job_started_at"); v != draft.GatedJobStartedAt {
			t.Errorf("gated_job_started_at witness = %q, want %q", v, draft.GatedJobStartedAt)
		}
		if v, _ := witnessValue(row.ProviderWitnesses, "environment_prevent_self_review"); v != "true" {
			t.Errorf("environment_prevent_self_review witness = %q, want %q", v, "true")
		}
	})

	t.Run("normalize: self-review setting absent when the provider does not report it", func(t *testing.T) {
		draft := baseSupported()
		draft.Reviews = []forge.EnvironmentReviewRow{approvedRow("901")}
		draft.GatedJobFound = true
		draft.GatedJobCreatedAt = "2026-08-26T09:00:00Z"
		facts, err := forge.NewEnvironmentReviewFacts(draft, fixedClock())
		if err != nil {
			t.Fatalf("fixture: %v", err)
		}
		rows, _ := forge.NormalizeEnvironmentReview(facts)
		if len(rows) != 1 {
			t.Fatalf("rows = %+v, want exactly 1", rows)
		}
		if hasWitness(rows[0].ProviderWitnesses, "environment_prevent_self_review") {
			t.Fatalf("environment_prevent_self_review witness present, want absent when the provider never reported it: %+v", rows[0].ProviderWitnesses)
		}
	})

	t.Run("normalize: deterministic across repeated calls", func(t *testing.T) {
		draft := baseSupported()
		draft.Reviews = []forge.EnvironmentReviewRow{approvedRow("901"), approvedRow("902")}
		draft.GatedJobFound = true
		draft.GatedJobCreatedAt = "2026-08-26T09:00:00Z"
		facts, err := forge.NewEnvironmentReviewFacts(draft, fixedClock())
		if err != nil {
			t.Fatalf("fixture: %v", err)
		}
		first, _ := forge.NormalizeEnvironmentReview(facts)
		second, _ := forge.NormalizeEnvironmentReview(facts)
		if len(first) != 2 || len(second) != 2 {
			t.Fatalf("rows = %+v / %+v, want 2 each", first, second)
		}
		if first[0].ApprovalID != second[0].ApprovalID || first[1].ApprovalID != second[1].ApprovalID {
			t.Fatalf("normalization is not deterministic: %+v vs %+v", first, second)
		}
		if first[0].ApprovalID >= first[1].ApprovalID {
			t.Fatalf("rows are not sorted by ApprovalID: %+v", first)
		}
	})
}

// TestEnvironmentReviewApprovalContract_Behavioral is the exact v2 ac-4
// behavioral obligation's producer: github/gitlab/fake driven through
// their real HTTP or in-memory surface.
func TestEnvironmentReviewApprovalContract_Behavioral(t *testing.T) {
	t.Run("github happy path produces facts and one active row", func(t *testing.T) {
		jobs := envReviewJobsBody(envReviewJobJSON("close", "2026-08-26T09:00:00Z", "2026-08-26T15:00:00Z"))
		history := "[" + envReviewHistoryEntryJSON("approved", 901, 9, "close") + "]"
		a, closeServer := environmentReviewServer(t, envReviewRunHealthy, jobs, envReviewEnvHealthy, history)
		defer closeServer()

		facts, err := a.EnvironmentReview(context.Background(), envReviewQuery())
		if err != nil {
			t.Fatalf("EnvironmentReview: %v", err)
		}
		if !facts.Supported {
			t.Fatalf("Supported = false, want true")
		}
		if facts.Repository != "acme/widgets" || facts.RunID != "555" || facts.RunAttempt != 1 {
			t.Fatalf("identity = %+v", facts)
		}
		if facts.RunHeadSHA != candidateA || facts.RunURL != envReviewRunURL {
			t.Fatalf("run binding = %+v", facts)
		}
		if facts.EnvironmentID != "9" || facts.EnvironmentName != "close" {
			t.Fatalf("environment = %+v", facts)
		}
		if !facts.GatedJobFound || facts.GatedJobCreatedAt != "2026-08-26T09:00:00Z" || facts.GatedJobStartedAt != "2026-08-26T15:00:00Z" {
			t.Fatalf("gated job stamps = %+v", facts)
		}
		if len(facts.Reviews) != 1 || facts.Reviews[0].ProviderState != forge.EnvironmentReviewApproved {
			t.Fatalf("reviews = %+v", facts.Reviews)
		}
		if facts.ProviderSnapshotID == "" || facts.ObservedAt == "" {
			t.Fatalf("facts lack observation identity: %+v", facts)
		}

		rows, disclosures := forge.NormalizeEnvironmentReview(facts)
		if disclosures != nil {
			t.Fatalf("disclosures = %v, want nil", disclosures)
		}
		if len(rows) != 1 || rows[0].ApprovalID != "github-environment-review:acme/widgets:555:1:9:901" {
			t.Fatalf("rows = %+v", rows)
		}
	})

	t.Run("github drains pages for jobs and review history", func(t *testing.T) {
		var jobsCalls, historyCalls int
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/repos/acme/widgets/actions/runs/555/attempts/1":
				writeJSON(t, w, envReviewRunHealthy)
			case "/repos/acme/widgets/actions/runs/555/attempts/1/jobs":
				jobsCalls++
				if r.URL.Query().Get("page") == "2" {
					writeJSON(t, w, envReviewJobsBody(envReviewJobJSON("close", "2026-08-26T09:00:00Z", "2026-08-26T15:00:00Z")))
					return
				}
				w.Header().Set("Link", `<`+urlForPage(r)+`?page=2>; rel="next"`)
				writeJSON(t, w, envReviewJobsBody(envReviewJobJSON("other-job", "2026-08-26T08:00:00Z", "2026-08-26T08:01:00Z")))
			case "/repos/acme/widgets/environments/close":
				writeJSON(t, w, envReviewEnvHealthy)
			case "/repos/acme/widgets/actions/runs/555/approvals":
				historyCalls++
				if r.URL.Query().Get("page") == "2" {
					writeJSON(t, w, "["+envReviewHistoryEntryJSON("approved", 901, 9, "close")+"]")
					return
				}
				w.Header().Set("Link", `<`+urlForPage(r)+`?page=2>; rel="next"`)
				writeJSON(t, w, "["+envReviewHistoryEntryJSON("rejected", 700, 9, "close")+"]")
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		a := github.New(github.Config{BaseURL: server.URL, Owner: "acme", Repo: "widgets", HTTPClient: server.Client(), Clock: fixedClock})
		facts, err := a.EnvironmentReview(context.Background(), envReviewQuery())
		if err != nil {
			t.Fatalf("EnvironmentReview: %v", err)
		}
		if jobsCalls < 2 || historyCalls < 2 {
			t.Fatalf("jobsCalls=%d historyCalls=%d, want both drained across at least 2 pages", jobsCalls, historyCalls)
		}
		if !facts.GatedJobFound || facts.GatedJobCreatedAt != "2026-08-26T09:00:00Z" {
			t.Fatalf("gated job (found on page 2) = %+v", facts)
		}
		if len(facts.Reviews) != 2 {
			t.Fatalf("reviews (drained across 2 pages) = %+v", facts.Reviews)
		}
		rows, _ := forge.NormalizeEnvironmentReview(facts)
		if len(rows) != 1 {
			t.Fatalf("rows = %+v, want exactly the one approved reviewer", rows)
		}
	})

	t.Run("github adverse review states and cancellation", func(t *testing.T) {
		tests := []struct {
			name    string
			jobs    string
			history string
			wantSub string
		}{
			{"rejected review", envReviewJobsBody(envReviewJobJSON("close", "2026-08-26T09:00:00Z", "")), "[" + envReviewHistoryEntryJSON("rejected", 901, 9, "close") + "]", ""},
			{"pending review", envReviewJobsBody(envReviewJobJSON("close", "2026-08-26T09:00:00Z", "")), "[" + envReviewHistoryEntryJSON("pending", 901, 9, "close") + "]", ""},
			{"absent review", envReviewJobsBody(envReviewJobJSON("close", "2026-08-26T09:00:00Z", "")), "[]", ""},
			{"cancelled run: gated job never created", envReviewJobsBody(), "[" + envReviewHistoryEntryJSON("approved", 901, 9, "close") + "]", "gated job not found"},
			{"missing creation stamp", envReviewJobsBody(envReviewJobJSON("close", "", "2026-08-26T15:00:00Z")), "[" + envReviewHistoryEntryJSON("approved", 901, 9, "close") + "]", "gated job creation stamp unavailable"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				a, closeServer := environmentReviewServer(t, envReviewRunHealthy, tt.jobs, envReviewEnvHealthy, tt.history)
				defer closeServer()
				facts, err := a.EnvironmentReview(context.Background(), envReviewQuery())
				if err != nil {
					t.Fatalf("EnvironmentReview: %v", err)
				}
				rows, disclosures := forge.NormalizeEnvironmentReview(facts)
				if len(rows) != 0 {
					t.Fatalf("rows = %+v, want none", rows)
				}
				if tt.wantSub != "" && !disclosuresContain(disclosures, tt.wantSub) {
					t.Fatalf("disclosures = %v, want a witness containing %q", disclosures, tt.wantSub)
				}
			})
		}
	})

	t.Run("github rerun (attempt 2) yields no rows even when the run's review history still shows an approval", func(t *testing.T) {
		runBody := `{"id":555,"run_attempt":2,"head_sha":"` + candidateA + `","html_url":"` + envReviewRunURL + `"}`
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/repos/acme/widgets/actions/runs/555/attempts/2":
				writeJSON(t, w, runBody)
			case "/repos/acme/widgets/actions/runs/555/attempts/2/jobs":
				writeJSON(t, w, envReviewJobsBody(envReviewJobJSON("close", "2026-08-26T09:00:00Z", "2026-08-26T09:05:00Z")))
			case "/repos/acme/widgets/environments/close":
				writeJSON(t, w, envReviewEnvHealthy)
			case "/repos/acme/widgets/actions/runs/555/approvals":
				writeJSON(t, w, "["+envReviewHistoryEntryJSON("approved", 901, 9, "close")+"]")
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		a := github.New(github.Config{BaseURL: server.URL, Owner: "acme", Repo: "widgets", HTTPClient: server.Client(), Clock: fixedClock})
		query := envReviewQuery()
		query.RunAttempt = 2
		facts, err := a.EnvironmentReview(context.Background(), query)
		if err != nil {
			t.Fatalf("EnvironmentReview: %v", err)
		}
		if facts.RunAttempt != 2 || len(facts.Reviews) != 1 {
			t.Fatalf("facts = %+v, want attempt 2 with the approval fact still present", facts)
		}
		rows, disclosures := forge.NormalizeEnvironmentReview(facts)
		if len(rows) != 0 {
			t.Fatalf("rows = %+v, want none for a rerun", rows)
		}
		if !disclosuresContain(disclosures, "rerun-not-honored") {
			t.Fatalf("disclosures = %v, want rerun-not-honored", disclosures)
		}
	})

	t.Run("github discloses self-review setting when present and omits it when absent", func(t *testing.T) {
		history := "[" + envReviewHistoryEntryJSON("approved", 901, 9, "close") + "]"
		jobs := envReviewJobsBody(envReviewJobJSON("close", "2026-08-26T09:00:00Z", ""))

		t.Run("present true", func(t *testing.T) {
			envBody := `{"id":9,"name":"close","protection_rules":[{"id":1,"node_id":"n1","type":"required_reviewers","prevent_self_review":true,"reviewers":[]}]}`
			a, closeServer := environmentReviewServer(t, envReviewRunHealthy, jobs, envBody, history)
			defer closeServer()
			facts, err := a.EnvironmentReview(context.Background(), envReviewQuery())
			if err != nil {
				t.Fatalf("EnvironmentReview: %v", err)
			}
			if facts.EnvironmentPreventSelfReview == nil || !*facts.EnvironmentPreventSelfReview {
				t.Fatalf("EnvironmentPreventSelfReview = %v, want true", facts.EnvironmentPreventSelfReview)
			}
		})
		t.Run("present false", func(t *testing.T) {
			envBody := `{"id":9,"name":"close","protection_rules":[{"id":1,"node_id":"n1","type":"required_reviewers","prevent_self_review":false,"reviewers":[]}]}`
			a, closeServer := environmentReviewServer(t, envReviewRunHealthy, jobs, envBody, history)
			defer closeServer()
			facts, err := a.EnvironmentReview(context.Background(), envReviewQuery())
			if err != nil {
				t.Fatalf("EnvironmentReview: %v", err)
			}
			if facts.EnvironmentPreventSelfReview == nil || *facts.EnvironmentPreventSelfReview {
				t.Fatalf("EnvironmentPreventSelfReview = %v, want false", facts.EnvironmentPreventSelfReview)
			}
		})
		t.Run("absent", func(t *testing.T) {
			a, closeServer := environmentReviewServer(t, envReviewRunHealthy, jobs, envReviewEnvHealthy, history)
			defer closeServer()
			facts, err := a.EnvironmentReview(context.Background(), envReviewQuery())
			if err != nil {
				t.Fatalf("EnvironmentReview: %v", err)
			}
			if facts.EnvironmentPreventSelfReview != nil {
				t.Fatalf("EnvironmentPreventSelfReview = %v, want nil (not reported)", facts.EnvironmentPreventSelfReview)
			}
		})
	})

	t.Run("github rejects duplicate unknown trailing and unknown-state facts", func(t *testing.T) {
		healthyJobs := envReviewJobsBody(envReviewJobJSON("close", "2026-08-26T09:00:00Z", ""))
		healthyHistory := "[" + envReviewHistoryEntryJSON("approved", 901, 9, "close") + "]"

		tests := []struct {
			name                                string
			runBody, jobsBody, envBody, history string
		}{
			{"run: unknown field", `{"id":555,"run_attempt":1,"head_sha":"` + candidateA + `","html_url":"` + envReviewRunURL + `","mystery":true}`, healthyJobs, envReviewEnvHealthy, healthyHistory},
			{"run: trailing data", envReviewRunHealthy + " true", healthyJobs, envReviewEnvHealthy, healthyHistory},
			{"jobs: unknown field on a job", envReviewRunHealthy, `{"total_count":1,"jobs":[{"name":"close","created_at":"2026-08-26T09:00:00Z","mystery":true}]}`, envReviewEnvHealthy, healthyHistory},
			{"jobs: trailing data", envReviewRunHealthy, healthyJobs + " true", envReviewEnvHealthy, healthyHistory},
			{"environment: unknown field", envReviewRunHealthy, healthyJobs, `{"id":9,"name":"close","mystery":true}`, healthyHistory},
			{"environment: trailing data", envReviewRunHealthy, healthyJobs, envReviewEnvHealthy + " true", healthyHistory},
			{"history: unknown field", envReviewRunHealthy, healthyJobs, envReviewEnvHealthy, `[{"state":"approved","comment":"","environments":[{"id":9,"name":"close"}],"user":{"id":901},"mystery":true}]`},
			{"history: trailing data", envReviewRunHealthy, healthyJobs, envReviewEnvHealthy, healthyHistory + " true"},
			{"history: unknown provider state", envReviewRunHealthy, healthyJobs, envReviewEnvHealthy, `[{"state":"commented","comment":"","environments":[{"id":9,"name":"close"}],"user":{"id":901}}]`},
			{"history: attempt field github does not supply", envReviewRunHealthy, healthyJobs, envReviewEnvHealthy, `[{"state":"approved","comment":"","environments":[{"id":9,"name":"close"}],"user":{"id":901},"attempt":1}]`},
			{"history: duplicate reviewer", envReviewRunHealthy, healthyJobs, envReviewEnvHealthy, "[" + envReviewHistoryEntryJSON("approved", 901, 9, "close") + "," + envReviewHistoryEntryJSON("approved", 901, 9, "close") + "]"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				a, closeServer := environmentReviewServer(t, tt.runBody, tt.jobsBody, tt.envBody, tt.history)
				defer closeServer()
				if _, err := a.EnvironmentReview(context.Background(), envReviewQuery()); err == nil {
					t.Fatalf("EnvironmentReview(%s): want error, got nil", tt.name)
				}
			})
		}
	})

	t.Run("github rejects an ambiguous jobs pagination continuation", func(t *testing.T) {
		var server *httptest.Server
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/repos/acme/widgets/actions/runs/555/attempts/1":
				writeJSON(t, w, envReviewRunHealthy)
			case "/repos/acme/widgets/actions/runs/555/attempts/1/jobs":
				if r.URL.Query().Get("page") == "" {
					w.Header().Set("Link", "<"+server.URL+r.URL.Path+"?page=2>; rel=\"next\", <"+server.URL+r.URL.Path+"?page=3>; rel=\"next\"")
				}
				writeJSON(t, w, envReviewJobsBody())
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		a := github.New(github.Config{BaseURL: server.URL, Owner: "acme", Repo: "widgets", HTTPClient: server.Client(), Clock: fixedClock})
		_, err := a.EnvironmentReview(context.Background(), envReviewQuery())
		if err == nil || !strings.Contains(err.Error(), "multiple distinct") {
			t.Fatalf("EnvironmentReview error = %v, want multiple-distinct-next error", err)
		}
	})

	t.Run("github rejects a review history pagination cycle", func(t *testing.T) {
		var calls int
		var server *httptest.Server
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/repos/acme/widgets/actions/runs/555/attempts/1":
				writeJSON(t, w, envReviewRunHealthy)
			case "/repos/acme/widgets/actions/runs/555/attempts/1/jobs":
				writeJSON(t, w, envReviewJobsBody(envReviewJobJSON("close", "2026-08-26T09:00:00Z", "")))
			case "/repos/acme/widgets/environments/close":
				writeJSON(t, w, envReviewEnvHealthy)
			case "/repos/acme/widgets/actions/runs/555/approvals":
				calls++
				if calls > 2 {
					http.Error(w, "unexpected pagination revisit", http.StatusInternalServerError)
					return
				}
				if r.URL.Query().Get("page") == "2" {
					w.Header().Set("Link", "<"+server.URL+r.URL.Path+"?per_page=100>; rel=\"next\"")
				} else {
					w.Header().Set("Link", "<"+server.URL+r.URL.Path+"?page=2>; rel=\"next\"")
				}
				writeJSON(t, w, "[]")
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		a := github.New(github.Config{BaseURL: server.URL, Owner: "acme", Repo: "widgets", HTTPClient: server.Client(), Clock: fixedClock})
		_, err := a.EnvironmentReview(context.Background(), envReviewQuery())
		if err == nil || !strings.Contains(err.Error(), "pagination cycle detected") {
			t.Fatalf("EnvironmentReview error = %v, want pagination cycle error (calls=%d)", err, calls)
		}
	})

	t.Run("gitlab environment review is unsupported with a disclosure and never an error", func(t *testing.T) {
		a := gitlab.New(gitlab.Config{BaseURL: "http://unused.invalid", ProjectID: "42", Clock: fixedClock})
		facts, err := a.EnvironmentReview(context.Background(), envReviewQuery())
		if err != nil {
			t.Fatalf("EnvironmentReview: %v, want nil error (never breaks a close)", err)
		}
		if facts.Supported {
			t.Fatalf("Supported = true, want false (unsupported source)")
		}
		if facts.UnsupportedReason == "" {
			t.Fatal("UnsupportedReason is empty, want a disclosed reason")
		}
		rows, disclosures := forge.NormalizeEnvironmentReview(facts)
		if len(rows) != 0 {
			t.Fatalf("rows = %+v, want none", rows)
		}
		if len(disclosures) == 0 {
			t.Fatal("disclosures empty, want a witness naming the unsupported source")
		}
	})

	t.Run("fake seeds and returns independent environment review facts", func(t *testing.T) {
		draft := forge.EnvironmentReviewFacts{
			Supported: true, Repository: "acme/widgets", RunID: "555", RunAttempt: 1,
			RunHeadSHA: candidateA, RunURL: envReviewRunURL, EnvironmentID: "9", EnvironmentName: "close",
			GatedJobFound: true, GatedJobCreatedAt: "2026-08-26T09:00:00Z",
			Reviews: []forge.EnvironmentReviewRow{{ReviewerActor: forge.ProviderActor{Scheme: "github-user-id", Subject: "901"}, ProviderState: forge.EnvironmentReviewApproved}},
		}
		seed, err := forge.NewEnvironmentReviewFacts(draft, fixedClock())
		if err != nil {
			t.Fatalf("fixture: %v", err)
		}
		f := fake.New()
		query := envReviewQuery()
		f.SeedEnvironmentReviewFacts(query, seed)

		first, err := f.EnvironmentReview(context.Background(), query)
		if err != nil {
			t.Fatalf("EnvironmentReview: %v", err)
		}
		first.Reviews[0].ProviderState = forge.EnvironmentReviewRejected
		second, err := f.EnvironmentReview(context.Background(), query)
		if err != nil {
			t.Fatalf("EnvironmentReview second: %v", err)
		}
		if second.Reviews[0].ProviderState != forge.EnvironmentReviewApproved {
			t.Fatalf("fake returned aliased facts: %+v", second.Reviews[0])
		}
	})

	t.Run("fake errors on an unseeded environment review query", func(t *testing.T) {
		f := fake.New()
		if _, err := f.EnvironmentReview(context.Background(), envReviewQuery()); err == nil {
			t.Fatal("EnvironmentReview unseeded: want error, got nil")
		}
	})
}

func urlForPage(r *http.Request) string {
	return "http://" + r.Host + r.URL.Path
}
