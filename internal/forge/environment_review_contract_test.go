package forge_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/forge/github"
	"github.com/jyang234/verdi/internal/forge/gitlab"
)

// normalizeEnvReview normalizes validated facts, failing the test on an
// operational error.
func normalizeEnvReview(t *testing.T, facts forge.EnvironmentReviewFacts) ([]forge.Approval, []string) {
	t.Helper()
	rows, disclosures, err := forge.NormalizeEnvironmentReview(facts)
	if err != nil {
		t.Fatalf("NormalizeEnvironmentReview: %v", err)
	}
	return rows, disclosures
}

// reviewScenario runs the GitHub adapter against a scenario and normalizes
// the result.
func reviewScenario(t *testing.T, s *erScenario, query forge.EnvironmentReviewQuery) (forge.EnvironmentReviewFacts, []forge.Approval, []string) {
	t.Helper()
	facts, err := s.adapter().EnvironmentReview(context.Background(), query)
	if err != nil {
		t.Fatalf("EnvironmentReview: %v", err)
	}
	rows, disclosures := normalizeEnvReview(t, facts)
	return facts, rows, disclosures
}

// disclosureKinds lists each disclosure's kind, the token after
// "environment-review:".
func disclosureKinds(disclosures []string) []string {
	kinds := make([]string, 0, len(disclosures))
	for _, d := range disclosures {
		kind := strings.TrimPrefix(d, "environment-review:")
		if i := strings.Index(kind, ":"); i >= 0 {
			kind = kind[:i]
		}
		kinds = append(kinds, kind)
	}
	return kinds
}

func disclosuresContain(disclosures []string, substr string) bool {
	for _, d := range disclosures {
		if strings.Contains(d, substr) {
			return true
		}
	}
	return false
}

func witnessMap(witnesses []forge.ProviderWitness) map[string]string {
	out := make(map[string]string, len(witnesses))
	for _, w := range witnesses {
		out[w.Name] = w.Value
	}
	return out
}

func approvalIDs(rows []forge.Approval) []string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ApprovalID)
	}
	return ids
}

// scenarioApprovalID is the composite identity v2 ac-4 prescribes for the
// published scenario's approval by reviewer.
func scenarioApprovalID(reviewer string) string {
	return "github-environment-review:" + erOwner + "/" + erRepo + ":" + erRunID + ":1:" + erEnvID + ":" + reviewer
}

func boolPtr(v bool) *bool { return &v }

// validFactsDraft is the provider-neutral facts value the published scenario
// produces, for tests of the facts contract itself.
func validFactsDraft() forge.EnvironmentReviewFacts {
	return forge.EnvironmentReviewFacts{
		Supported: true, Repository: erOwner + "/" + erRepo, RunID: erRunID, RunAttempt: 1, LatestRunAttempt: 1,
		RunHeadSHA: erHeadSHA, RunURL: erRunURL, RunEvent: "workflow_dispatch", RunWorkflowPath: erWorkflowPath,
		RunStatus: "in_progress", EnvironmentID: erEnvID, EnvironmentName: "close",
		EnvironmentPreventSelfReview: boolPtr(false),
		GatedJobName:                 "close", GatedJobCount: 1, GatedJobID: erJobID, GatedJobStatus: "completed",
		GatedJobConclusion: "success", GatedJobCreatedAt: erCreatedAt, GatedJobStartedAt: erStartedAt,
		Reviews: []forge.EnvironmentReviewRow{approvedReviewRow(erReviewer)},
	}
}

func approvedReviewRow(reviewer string) forge.EnvironmentReviewRow {
	return forge.EnvironmentReviewRow{
		ReviewerActor: forge.ProviderActor{Scheme: "github-user-id", Subject: reviewer},
		ProviderState: forge.EnvironmentReviewApproved, EnvironmentID: erEnvID,
	}
}

// The derived-field disclosures every environment-review row carries,
// verbatim (v2 dc-5: "each derived field is disclosed in provider_witnesses").
const (
	wantApprovalIDDerivation = "composite of repository, run id, run attempt, environment id, and reviewer id (github's review history carries no review id)"
	wantApprovedAtDerivation = "the gated job's creation stamp: a conservative lower bound, since github's review history carries no review time; the job's start stamp is recorded separately below, never as the approval instant"
	wantStateDerivation      = "github's approved review state normalized to the shared active state"
)

// scenarioWitnesses is the exact provider-witness set of the published
// scenario's row for reviewer "1": every operand and every derived-field
// disclosure, and nothing GitHub does not supply (no review id, no review
// time, no attempt of the review).
func scenarioWitnesses() map[string]string {
	return map[string]string{
		"actor_user_id":                   erReviewer,
		"approval_id_derivation":          wantApprovalIDDerivation,
		"approved_at_derivation":          wantApprovedAtDerivation,
		"environment_id":                  erEnvID,
		"environment_name":                "close",
		"environment_prevent_self_review": "false",
		"gated_job_conclusion":            "success",
		"gated_job_created_at":            erCreatedAt,
		"gated_job_id":                    erJobID,
		"gated_job_name":                  "close",
		"gated_job_started_at":            erStartedAt,
		"gated_job_status":                "completed",
		"provider_state":                  "approved",
		"run_attempt":                     "1",
		"run_event":                       "workflow_dispatch",
		"run_head_sha":                    erHeadSHA,
		"run_id":                          erRunID,
		"run_latest_attempt":              "1",
		"run_status":                      "in_progress",
		"run_url":                         erRunURL,
		"run_workflow_path":               erWorkflowPath,
		"state_derivation":                wantStateDerivation,
	}
}

// TestEnvironmentReviewApprovalContract_Static is the exact producer of the
// v2 ac-4 static obligation (go-test:internal/forge:
// TestEnvironmentReviewApprovalContract_Static). Its scope is the shared
// approval value, the GitHub environment-review decoder and normalizer, and
// their decoding boundaries over responses shaped exactly as GitHub
// publishes them, so it drives the real GitHub adapter through httptest over
// GitHub's published examples (environment_review_fixtures_test.go) as well
// as the facts contract and the pure mapping.
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

	t.Run("github decodes the verbatim published run jobs environment and review history examples", func(t *testing.T) {
		run := string(publishedFixture(t, "github/workflow-run.json"))
		jobs := string(publishedFixture(t, "github/job-paginated.json"))
		env := string(publishedFixture(t, "github/environment.json"))
		history := string(publishedFixture(t, "github/environment-approvals-items.json"))
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/repos/octo-org/octo-repo/actions/runs/30433642":
				writeJSON(t, w, run)
			case "/repos/octo-org/octo-repo/actions/runs/30433642/attempts/1/jobs":
				writeJSON(t, w, jobs)
			case "/repos/octo-org/octo-repo/environments/staging":
				writeJSON(t, w, env)
			case "/repos/octo-org/octo-repo/actions/runs/30433642/approvals":
				writeJSON(t, w, history)
			default:
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		a := github.New(github.Config{BaseURL: server.URL, Owner: erOwner, Repo: erRepo, HTTPClient: server.Client(), Clock: fixedClock})
		facts, err := a.EnvironmentReview(context.Background(), forge.EnvironmentReviewQuery{RunID: erRunID, RunAttempt: 1, EnvironmentName: "staging", GatedJobName: "build"})
		if err != nil {
			t.Fatalf("EnvironmentReview over the verbatim published examples: %v", err)
		}
		want := forge.EnvironmentReviewFacts{
			Supported: true, Repository: erOwner + "/" + erRepo, RunID: erRunID, RunAttempt: 1, LatestRunAttempt: 1,
			RunHeadSHA: erHeadSHA, RunURL: erRunURL, RunEvent: "push", RunWorkflowPath: ".github/workflows/build.yml@main",
			RunStatus: "queued", EnvironmentID: erEnvID, EnvironmentName: "staging", EnvironmentPreventSelfReview: boolPtr(false),
			GatedJobName: "build", GatedJobCount: 1, GatedJobID: erJobID, GatedJobStatus: "completed", GatedJobConclusion: "success",
			GatedJobStartedAt: erStartedAt,
			Reviews:           []forge.EnvironmentReviewRow{approvedReviewRow(erReviewer)},
		}
		got := facts
		got.ObservedAt, got.ProviderSnapshotID = "", ""
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("facts from the published examples =\n%+v\nwant\n%+v", got, want)
		}
		rows, disclosures := normalizeEnvReview(t, facts)
		if len(rows) != 0 {
			t.Fatalf("rows = %+v, want none: the published run is a push run of another workflow and its job carries no creation stamp", rows)
		}
		if kinds := disclosureKinds(disclosures); strings.Join(kinds, ",") != "not-workflow-dispatch,not-close-workflow,creation-stamp-unavailable" {
			t.Fatalf("disclosure kinds = %v (%v)", kinds, disclosures)
		}
	})

	t.Run("github ignores response members it does not model (open provider contract)", func(t *testing.T) {
		want, _, _ := reviewScenario(t, newERScenario(t), erQuery())
		extend := func(object map[string]any) map[string]any {
			object["future_member"] = map[string]any{"nested": []any{1, "two"}}
			return object
		}
		tests := []struct {
			name  string
			apply func(*erScenario)
		}{
			{"run", func(s *erScenario) { s.rawRun = encodeGeneric(t, extend(s.run)) }},
			{"jobs page and job", func(s *erScenario) {
				s.rawJobs = []string{encodeGeneric(t, extend(map[string]any{"total_count": 1, "jobs": []any{extend(s.jobPages[0][0])}}))}
			}},
			{"environment and protection rule", func(s *erScenario) {
				extend(asObject(t, asArray(t, s.env["protection_rules"])[1]))
				s.rawEnv = encodeGeneric(t, extend(s.env))
			}},
			{"review history entry and environment", func(s *erScenario) {
				entry := s.historyPages[0][0]
				extend(asObject(t, asArray(t, entry["environments"])[0]))
				s.rawHistory = []string{encodeGeneric(t, []any{extend(entry)})}
			}},
			{"review history attempt member GitHub does not supply is never read", func(s *erScenario) {
				entry := s.historyPages[0][0]
				entry["attempt"] = 2
				entry["run_attempt"] = 2
				s.rawHistory = []string{encodeGeneric(t, []any{entry})}
			}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				s := newERScenario(t)
				tt.apply(s)
				got, _, _ := reviewScenario(t, s, erQuery())
				if got.ProviderSnapshotID != want.ProviderSnapshotID {
					t.Fatalf("members GitHub may add changed the facts:\ngot  %+v\nwant %+v", got, want)
				}
			})
		}
	})

	t.Run("github rejects trailing data, mistyped members, and unknown vocabularies (co-1)", func(t *testing.T) {
		tests := []struct {
			name    string
			apply   func(*erScenario)
			wantErr string
		}{
			{"run: trailing data", func(s *erScenario) { s.rawRun = encodeGeneric(t, s.run) + " true" }, "trailing data"},
			{"jobs: trailing data", func(s *erScenario) {
				s.rawJobs = []string{encodeGeneric(t, map[string]any{"total_count": 1, "jobs": []any{s.jobPages[0][0]}}) + " {}"}
			}, "trailing data"},
			{"environment: trailing data", func(s *erScenario) { s.rawEnv = encodeGeneric(t, s.env) + " []" }, "trailing data"},
			{"history: trailing data", func(s *erScenario) {
				s.rawHistory = []string{encodeGeneric(t, []any{s.historyPages[0][0]}) + " null"}
			}, "trailing data"},
			{"run: id as a string", func(s *erScenario) {
				s.run["id"] = erRunID
				s.rawRun = encodeGeneric(t, s.run)
			}, "decode approval response"},
			{"history: unknown review state", func(s *erScenario) {
				s.historyPages[0][0]["state"] = "commented"
				s.rawHistory = []string{encodeGeneric(t, []any{s.historyPages[0][0]})}
			}, "unknown environment review state"},
			{"history: missing review state", func(s *erScenario) {
				delete(s.historyPages[0][0], "state")
				s.rawHistory = []string{encodeGeneric(t, []any{s.historyPages[0][0]})}
			}, "unknown state"},
			{"run: unknown status", func(s *erScenario) { s.run["status"] = "cancelling" }, "unknown workflow run status"},
			{"run: unknown conclusion", func(s *erScenario) {
				s.run["status"] = "completed"
				s.run["conclusion"] = "exploded"
			}, "unknown workflow run conclusion"},
			{"gated job: unknown status", func(s *erScenario) { s.jobPages[0][0]["status"] = "blocked" }, "unknown workflow job status"},
			{"gated job: unknown conclusion", func(s *erScenario) { s.jobPages[0][0]["conclusion"] = "exploded" }, "unknown workflow job conclusion"},
			{"gated job: malformed creation stamp", func(s *erScenario) { s.jobPages[0][0]["created_at"] = "yesterday" }, "created_at"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				s := newERScenario(t)
				tt.apply(s)
				facts, err := s.adapter().EnvironmentReview(context.Background(), erQuery())
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("EnvironmentReview = %+v, %v; want an error containing %q", facts, err, tt.wantErr)
				}
			})
		}
	})

	t.Run("github refuses missing provider ids (co-1)", func(t *testing.T) {
		tests := []struct {
			name    string
			apply   func(*erScenario)
			wantErr string
		}{
			{"run id", func(s *erScenario) { delete(s.run, "id"); s.rawRun = encodeGeneric(t, s.run) }, "reported id 0"},
			{"run attempt", func(s *erScenario) { delete(s.run, "run_attempt"); s.rawRun = encodeGeneric(t, s.run) }, "carries no run_attempt"},
			{"environment id", func(s *erScenario) { delete(s.env, "id"); s.rawEnv = encodeGeneric(t, s.env) }, "carries no stable id"},
			{"gated job id", func(s *erScenario) {
				delete(s.jobPages[0][0], "id")
				s.rawJobs = []string{encodeGeneric(t, map[string]any{"total_count": 1, "jobs": []any{s.jobPages[0][0]}})}
			}, "carries no stable id"},
			{"review entry environment id", func(s *erScenario) {
				delete(asObject(t, asArray(t, s.historyPages[0][0]["environments"])[0]), "id")
				s.rawHistory = []string{encodeGeneric(t, []any{s.historyPages[0][0]})}
			}, "environment with no stable id"},
			{"review entry environments", func(s *erScenario) {
				s.historyPages[0][0]["environments"] = []any{}
				s.rawHistory = []string{encodeGeneric(t, []any{s.historyPages[0][0]})}
			}, "names no environment"},
			{"reviewer id", func(s *erScenario) {
				delete(asObject(t, s.historyPages[0][0]["user"]), "id")
				s.rawHistory = []string{encodeGeneric(t, []any{s.historyPages[0][0]})}
			}, "no stable reviewer id"},
			{"run head sha", func(s *erScenario) { delete(s.run, "head_sha"); s.rawRun = encodeGeneric(t, s.run) }, "run_head_sha"},
			{"run url", func(s *erScenario) { delete(s.run, "html_url"); s.rawRun = encodeGeneric(t, s.run) }, "run_url"},
			{"run event", func(s *erScenario) { delete(s.run, "event"); s.rawRun = encodeGeneric(t, s.run) }, "run_event"},
			{"run workflow path", func(s *erScenario) { delete(s.run, "path"); s.rawRun = encodeGeneric(t, s.run) }, "run_workflow_path"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				s := newERScenario(t)
				tt.apply(s)
				facts, err := s.adapter().EnvironmentReview(context.Background(), erQuery())
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("EnvironmentReview = %+v, %v; want an error containing %q", facts, err, tt.wantErr)
				}
			})
		}
	})

	t.Run("github refuses null and inconsistent pages (m-8)", func(t *testing.T) {
		job := func(t *testing.T) map[string]any { return scenarioJob(t, "close") }
		tests := []struct {
			name    string
			apply   func(*erScenario)
			wantErr string
		}{
			{"null jobs page", func(s *erScenario) { s.rawJobs = []string{`null`} }, "total_count and a non-null jobs array"},
			{"null jobs member", func(s *erScenario) { s.rawJobs = []string{`{"total_count":0,"jobs":null}`} }, "total_count and a non-null jobs array"},
			{"missing total_count", func(s *erScenario) {
				s.rawJobs = []string{encodeGeneric(t, map[string]any{"jobs": []any{job(t)}})}
			}, "total_count and a non-null jobs array"},
			{"total_count larger than the jobs listed", func(s *erScenario) {
				s.rawJobs = []string{encodeGeneric(t, map[string]any{"total_count": 5, "jobs": []any{job(t)}})}
			}, "does not match"},
			{"pages disagree on total_count", func(s *erScenario) {
				s.rawJobs = []string{
					encodeGeneric(t, map[string]any{"total_count": 2, "jobs": []any{job(t)}}),
					encodeGeneric(t, map[string]any{"total_count": 3, "jobs": []any{scenarioJob(t, "build")}}),
				}
			}, "disagree on total_count"},
			{"null review history page", func(s *erScenario) { s.rawHistory = []string{`null`} }, "non-null array"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				s := newERScenario(t)
				tt.apply(s)
				facts, err := s.adapter().EnvironmentReview(context.Background(), erQuery())
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("EnvironmentReview = %+v, %v; want an error containing %q", facts, err, tt.wantErr)
				}
			})
		}
	})

	t.Run("github refuses ambiguous and cyclic pagination (co-1)", func(t *testing.T) {
		scenario := newERScenario(t)
		jobsBody := scenario.jobPageBodies()[0]
		historyBody := scenario.historyPageBodies()[0]
		tests := []struct {
			name    string
			link    func(server *httptest.Server, r *http.Request) (route, header string)
			wantErr string
		}{
			{"two distinct next pages of jobs", func(server *httptest.Server, r *http.Request) (string, string) {
				return "jobs", "<" + server.URL + r.URL.Path + "?page=2>; rel=\"next\", <" + server.URL + r.URL.Path + "?page=3>; rel=\"next\""
			}, "multiple distinct"},
			{"review history pages that cycle", func(server *httptest.Server, r *http.Request) (string, string) {
				if r.URL.Query().Get("page") == "2" {
					return "history", "<" + server.URL + r.URL.Path + "?per_page=100>; rel=\"next\""
				}
				return "history", "<" + server.URL + r.URL.Path + "?page=2>; rel=\"next\""
			}, "pagination cycle detected"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var server *httptest.Server
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					route, header := tt.link(server, r)
					switch r.URL.Path {
					case "/repos/octo-org/octo-repo/actions/runs/30433642/attempts/1/jobs":
						if route == "jobs" {
							w.Header().Set("Link", header)
						}
						writeJSON(t, w, jobsBody)
					case "/repos/octo-org/octo-repo/actions/runs/30433642/approvals":
						if route == "history" {
							w.Header().Set("Link", header)
						}
						writeJSON(t, w, historyBody)
					default:
						http.NotFound(w, r)
					}
				}))
				defer server.Close()
				a := github.New(github.Config{BaseURL: server.URL, Owner: erOwner, Repo: erRepo, HTTPClient: server.Client(), Clock: fixedClock})
				_, err := a.EnvironmentReview(context.Background(), erQuery())
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("EnvironmentReview error = %v, want %q", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("github refuses a non-canonical run id before any request (m-1)", func(t *testing.T) {
		for _, id := range []string{"+30433642", "030433642", " 30433642", "30433642 ", "3.0433642e7", "0", "-30433642", ""} {
			t.Run(fmt.Sprintf("%q", id), func(t *testing.T) {
				a := github.New(github.Config{BaseURL: "http://unused.invalid", Owner: erOwner, Repo: erRepo, Clock: fixedClock})
				query := erQuery()
				query.RunID = id
				if facts, err := a.EnvironmentReview(context.Background(), query); err == nil {
					t.Fatalf("EnvironmentReview(run id %q) = %+v, want a refusal", id, facts)
				}
			})
		}
	})

	t.Run("facts contract refuses incomplete, non-canonical, or unknown facts", func(t *testing.T) {
		tests := []struct {
			name   string
			mutate func(*forge.EnvironmentReviewFacts)
		}{
			{"missing repository", func(f *forge.EnvironmentReviewFacts) { f.Repository = "" }},
			{"missing run id", func(f *forge.EnvironmentReviewFacts) { f.RunID = "" }},
			{"run id with a plus sign (m-1)", func(f *forge.EnvironmentReviewFacts) { f.RunID = "+" + erRunID }},
			{"run id with a leading zero (m-1)", func(f *forge.EnvironmentReviewFacts) { f.RunID = "0" + erRunID }},
			{"zero run attempt", func(f *forge.EnvironmentReviewFacts) { f.RunAttempt = 0 }},
			{"zero latest run attempt", func(f *forge.EnvironmentReviewFacts) { f.LatestRunAttempt = 0 }},
			{"short run head sha", func(f *forge.EnvironmentReviewFacts) { f.RunHeadSHA = "acb5820" }},
			{"missing run url", func(f *forge.EnvironmentReviewFacts) { f.RunURL = "" }},
			{"missing run event", func(f *forge.EnvironmentReviewFacts) { f.RunEvent = "" }},
			{"missing run workflow path", func(f *forge.EnvironmentReviewFacts) { f.RunWorkflowPath = "" }},
			{"unknown run status", func(f *forge.EnvironmentReviewFacts) { f.RunStatus = "cancelling" }},
			{"unknown run conclusion", func(f *forge.EnvironmentReviewFacts) { f.RunConclusion = "exploded" }},
			{"missing environment id", func(f *forge.EnvironmentReviewFacts) { f.EnvironmentID = "" }},
			{"environment id with a leading zero (m-1)", func(f *forge.EnvironmentReviewFacts) { f.EnvironmentID = "0" + erEnvID }},
			{"missing environment name", func(f *forge.EnvironmentReviewFacts) { f.EnvironmentName = "" }},
			{"missing gated job name", func(f *forge.EnvironmentReviewFacts) { f.GatedJobName = "" }},
			{"negative gated job count", func(f *forge.EnvironmentReviewFacts) { f.GatedJobCount = -1 }},
			{"gated job stamps without exactly one gated job", func(f *forge.EnvironmentReviewFacts) {
				f.GatedJobCount, f.GatedJobID, f.GatedJobStatus, f.GatedJobConclusion = 0, "", "", ""
			}},
			{"gated job details with two gated jobs", func(f *forge.EnvironmentReviewFacts) { f.GatedJobCount = 2 }},
			{"missing gated job id", func(f *forge.EnvironmentReviewFacts) { f.GatedJobID = "" }},
			{"unknown gated job status", func(f *forge.EnvironmentReviewFacts) { f.GatedJobStatus = "blocked" }},
			{"unknown gated job conclusion", func(f *forge.EnvironmentReviewFacts) { f.GatedJobConclusion = "exploded" }},
			{"non-UTC creation stamp", func(f *forge.EnvironmentReviewFacts) { f.GatedJobCreatedAt = "2020-01-20T09:30:00-08:00" }},
			{"unknown review state", func(f *forge.EnvironmentReviewFacts) {
				f.Reviews[0].ProviderState = forge.EnvironmentReviewState("commented")
			}},
			{"review row for another environment (m-2)", func(f *forge.EnvironmentReviewFacts) { f.Reviews[0].EnvironmentID = "8" }},
			{"review row with a display-name reviewer", func(f *forge.EnvironmentReviewFacts) { f.Reviews[0].ReviewerActor.Subject = "octocat" }},
			{"repeated decision while the latest attempt is the first (co-1)", func(f *forge.EnvironmentReviewFacts) {
				f.Reviews = append(f.Reviews, f.Reviews[0])
			}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				draft := validFactsDraft()
				tt.mutate(&draft)
				if facts, err := forge.NewEnvironmentReviewFacts(draft, fixedClock()); err == nil {
					t.Fatalf("NewEnvironmentReviewFacts(%s) = %+v, want error", tt.name, facts)
				}
			})
		}
	})

	t.Run("unsupported facts carry a reason and nothing else", func(t *testing.T) {
		if _, err := forge.NewEnvironmentReviewFacts(forge.EnvironmentReviewFacts{Supported: false, UnsupportedReason: "gitlab: unsupported", Repository: "42"}, fixedClock()); err != nil {
			t.Fatalf("NewEnvironmentReviewFacts unsupported: %v", err)
		}
		tests := []struct {
			name  string
			draft forge.EnvironmentReviewFacts
		}{
			{"no reason", forge.EnvironmentReviewFacts{Supported: false, Repository: "42"}},
			{"run data", forge.EnvironmentReviewFacts{Supported: false, UnsupportedReason: "gitlab: unsupported", Repository: "42", RunID: "1"}},
			{"gated job data", forge.EnvironmentReviewFacts{Supported: false, UnsupportedReason: "gitlab: unsupported", Repository: "42", GatedJobCount: 1}},
			{"run conclusion", forge.EnvironmentReviewFacts{Supported: false, UnsupportedReason: "gitlab: unsupported", Repository: "42", RunConclusion: "success"}},
			{"review rows", forge.EnvironmentReviewFacts{Supported: false, UnsupportedReason: "gitlab: unsupported", Repository: "42", Reviews: []forge.EnvironmentReviewRow{approvedReviewRow("1")}}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if _, err := forge.NewEnvironmentReviewFacts(tt.draft, fixedClock()); err == nil {
					t.Fatalf("NewEnvironmentReviewFacts(%s): want error, got nil", tt.name)
				}
			})
		}
	})

	t.Run("normalize refuses facts that break the facts contract (m-3)", func(t *testing.T) {
		valid, err := forge.NewEnvironmentReviewFacts(validFactsDraft(), fixedClock())
		if err != nil {
			t.Fatalf("fixture: %v", err)
		}
		tampered := valid
		tampered.RunHeadSHA = candidateA
		nullReviews := valid
		nullReviews.Reviews = nil
		tests := []struct {
			name  string
			facts forge.EnvironmentReviewFacts
		}{
			{"null reviews", nullReviews},
			{"hand-built value never validated", forge.EnvironmentReviewFacts{
				Supported: true, RunAttempt: 1, LatestRunAttempt: 1, GatedJobCount: 1, GatedJobCreatedAt: "not-a-time",
				Reviews: []forge.EnvironmentReviewRow{approvedReviewRow("901")},
			}},
			{"zero value", forge.EnvironmentReviewFacts{}},
			{"validated facts changed after their digest", tampered},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				rows, disclosures, err := forge.NormalizeEnvironmentReview(tt.facts)
				if err == nil {
					t.Fatalf("NormalizeEnvironmentReview(%s) = rows %+v disclosures %v, want an error", tt.name, rows, disclosures)
				}
				if len(rows) != 0 {
					t.Fatalf("NormalizeEnvironmentReview(%s) produced rows %+v alongside its error", tt.name, rows)
				}
			})
		}
	})

	t.Run("github maps an approved review of an eligible close run to exactly one fully disclosed row", func(t *testing.T) {
		s := newERScenario(t)
		facts, rows, disclosures := reviewScenario(t, s, erQuery())
		if disclosures != nil {
			t.Fatalf("disclosures = %v, want none for an eligible approved review", disclosures)
		}
		if len(rows) != 1 {
			t.Fatalf("rows = %+v, want exactly one", rows)
		}
		row := rows[0]
		if row.ApprovalID != scenarioApprovalID(erReviewer) {
			t.Fatalf("ApprovalID = %q, want the complete composite %q", row.ApprovalID, scenarioApprovalID(erReviewer))
		}
		if row.ApprovalRef != erRunURL+" environment=close" {
			t.Fatalf("ApprovalRef = %q, want the run URL and environment name", row.ApprovalRef)
		}
		if row.State != forge.ApprovalActive {
			t.Fatalf("State = %q, want the shared active state", row.State)
		}
		if row.ApprovedAt != erCreatedAt || row.UpdatedAt != erCreatedAt {
			t.Fatalf("ApprovedAt/UpdatedAt = %q/%q, want the gated job's creation stamp %q, never its start %q", row.ApprovedAt, row.UpdatedAt, erCreatedAt, erStartedAt)
		}
		if row.CandidateSHA != erHeadSHA || row.CandidateSHA == erJobHeadSHA {
			t.Fatalf("CandidateSHA = %q, want the run's head commit %q", row.CandidateSHA, erHeadSHA)
		}
		if row.Actor != (forge.ProviderActor{Scheme: "github-user-id", Subject: erReviewer}) {
			t.Fatalf("Actor = %+v, want the reviewer's stable user id", row.Actor)
		}
		if got := witnessMap(row.ProviderWitnesses); !reflect.DeepEqual(got, scenarioWitnesses()) {
			t.Fatalf("provider witnesses =\n%v\nwant exactly\n%v", got, scenarioWitnesses())
		}
		names := make([]string, 0, len(row.ProviderWitnesses))
		for _, w := range row.ProviderWitnesses {
			names = append(names, w.Name)
		}
		if !sort.StringsAreSorted(names) {
			t.Fatalf("provider witnesses are not sorted by name: %v", names)
		}
		if _, err := forge.NewApprovalSnapshot("github", facts.Repository, "1347", facts.RunHeadSHA, forge.ProviderActor{Scheme: "github-user-id", Subject: "2"}, fixedClock(), rows); err != nil {
			t.Fatalf("the row does not satisfy the shared approval contract: %v", err)
		}
		if log := strings.Join(s.requestLog(), ","); log != "history,jobs,environment,run" {
			t.Fatalf("requests = %s, want the review history first, the queried attempt's jobs, the environment, and the run last", log)
		}
	})

	t.Run("github discloses the self-review setting only when GitHub reports it", func(t *testing.T) {
		tests := []struct {
			name  string
			apply func(*erScenario)
			want  string
		}{
			{"reported false (published)", func(*erScenario) {}, "false"},
			{"reported true", func(s *erScenario) {
				setMember(t, asObject(t, asArray(t, s.env["protection_rules"])[1]), "prevent_self_review", true)
			}, "true"},
			{"not reported", func(s *erScenario) {
				delete(asObject(t, asArray(t, s.env["protection_rules"])[1]), "prevent_self_review")
				s.envAllowRemoved = []string{".protection_rules[].prevent_self_review"}
			}, ""},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				s := newERScenario(t)
				tt.apply(s)
				_, rows, _ := reviewScenario(t, s, erQuery())
				if len(rows) != 1 {
					t.Fatalf("rows = %+v, want one", rows)
				}
				got, ok := witnessMap(rows[0].ProviderWitnesses)["environment_prevent_self_review"]
				if tt.want == "" && ok {
					t.Fatalf("environment_prevent_self_review = %q, want no witness when GitHub does not report the setting", got)
				}
				if tt.want != "" && got != tt.want {
					t.Fatalf("environment_prevent_self_review = %q (present %v), want %q", got, ok, tt.want)
				}
			})
		}
	})

	t.Run("github honors only a run whose first attempt is its latest (I-1, m-10)", func(t *testing.T) {
		tests := []struct {
			name    string
			apply   func(*erScenario)
			attempt int
		}{
			{"approved attempt 1 after a rerun began", func(s *erScenario) { setMember(t, s.run, "run_attempt", 2) }, 1},
			{"attempt 1 rejected, attempt 2 approved by the same reviewer", func(s *erScenario) {
				setMember(t, s.run, "run_attempt", 2)
				s.historyPages = [][]map[string]any{{historyEntry(t, "rejected", erEnvIDNum, "close", erReviewerNum), historyEntry(t, "approved", erEnvIDNum, "close", erReviewerNum)}}
			}, 1},
			{"the same owner approved attempts 1 and 2", func(s *erScenario) {
				setMember(t, s.run, "run_attempt", 2)
				s.historyPages = [][]map[string]any{{historyEntry(t, "approved", erEnvIDNum, "close", erReviewerNum), historyEntry(t, "approved", erEnvIDNum, "close", erReviewerNum)}}
			}, 1},
			{"the queried attempt is the rerun", func(s *erScenario) {
				setMember(t, s.run, "run_attempt", 2)
				s.attempt = 2
			}, 2},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				s := newERScenario(t)
				tt.apply(s)
				query := erQuery()
				query.RunAttempt = tt.attempt
				facts, rows, disclosures := reviewScenario(t, s, query)
				if facts.RunAttempt != tt.attempt || facts.LatestRunAttempt != 2 {
					t.Fatalf("attempts = queried %d latest %d, want %d and 2", facts.RunAttempt, facts.LatestRunAttempt, tt.attempt)
				}
				if len(rows) != 0 {
					t.Fatalf("rows = %+v, want none: a review cannot be tied to one attempt once the run is rerun", rows)
				}
				if !disclosuresContain(disclosures, "environment-review:rerun-not-honored:") || !disclosuresContain(disclosures, "latest_run_attempt=2") {
					t.Fatalf("disclosures = %v, want rerun-not-honored naming latest attempt 2", disclosures)
				}
			})
		}
	})

	t.Run("github honors only a workflow_dispatch run of the close workflow that is neither cancelled nor failed (I-3)", func(t *testing.T) {
		tests := []struct {
			name     string
			apply    func(*erScenario)
			wantKind string
		}{
			{"a bare close workflow path", func(s *erScenario) { setMember(t, s.run, "path", forge.CloseWorkflowPath) }, ""},
			{"a close workflow path read from a close branch", func(s *erScenario) {
				setMember(t, s.run, "path", forge.CloseWorkflowPath+"@refs/heads/close/spec-demo")
			}, ""},
			{"a run completed successfully", func(s *erScenario) {
				setMember(t, s.run, "status", "completed")
				setMember(t, s.run, "conclusion", "success")
			}, ""},
			{"a queued run", func(s *erScenario) { setMember(t, s.run, "status", "queued") }, ""},
			{"a push run", func(s *erScenario) { setMember(t, s.run, "event", "push") }, "not-workflow-dispatch"},
			{"a pull_request run", func(s *erScenario) { setMember(t, s.run, "event", "pull_request") }, "not-workflow-dispatch"},
			{"another workflow", func(s *erScenario) { setMember(t, s.run, "path", ".github/workflows/verify.yml@main") }, "not-close-workflow"},
			{"a reusable close workflow from another repository", func(s *erScenario) {
				setMember(t, s.run, "path", "octo-org/other/.github/workflows/close.yml@main")
			}, "not-close-workflow"},
			{"a close workflow path with an empty ref", func(s *erScenario) { setMember(t, s.run, "path", forge.CloseWorkflowPath+"@") }, "not-close-workflow"},
			{"a lookalike workflow file", func(s *erScenario) { setMember(t, s.run, "path", forge.CloseWorkflowPath+".bak") }, "not-close-workflow"},
			{"a cancelled run", func(s *erScenario) {
				setMember(t, s.run, "status", "completed")
				setMember(t, s.run, "conclusion", "cancelled")
			}, "run-not-eligible"},
			{"a failed run", func(s *erScenario) {
				setMember(t, s.run, "status", "completed")
				setMember(t, s.run, "conclusion", "failure")
			}, "run-not-eligible"},
			{"a timed-out run", func(s *erScenario) {
				setMember(t, s.run, "status", "completed")
				setMember(t, s.run, "conclusion", "timed_out")
			}, "run-not-eligible"},
			{"a run that failed to start", func(s *erScenario) {
				setMember(t, s.run, "status", "completed")
				setMember(t, s.run, "conclusion", "startup_failure")
			}, "run-not-eligible"},
			{"a run concluded neutral", func(s *erScenario) {
				setMember(t, s.run, "status", "completed")
				setMember(t, s.run, "conclusion", "neutral")
			}, "run-not-eligible"},
			{"a completed run with no conclusion", func(s *erScenario) { setMember(t, s.run, "status", "completed") }, "run-not-eligible"},
			{"a run with no status", func(s *erScenario) { setMember(t, s.run, "status", nil) }, "run-not-eligible"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				s := newERScenario(t)
				tt.apply(s)
				_, rows, disclosures := reviewScenario(t, s, erQuery())
				if tt.wantKind == "" {
					if len(rows) != 1 || disclosures != nil {
						t.Fatalf("rows = %v disclosures = %v, want one row and no disclosure", approvalIDs(rows), disclosures)
					}
					return
				}
				if len(rows) != 0 {
					t.Fatalf("rows = %v, want none", approvalIDs(rows))
				}
				if kinds := disclosureKinds(disclosures); strings.Join(kinds, ",") != tt.wantKind {
					t.Fatalf("disclosure kinds = %v (%v), want exactly %s", kinds, disclosures, tt.wantKind)
				}
			})
		}
	})

	t.Run("github honors only exactly one gated job in progress or completed successfully (I-3)", func(t *testing.T) {
		withJob := func(status string, conclusion any) func(*erScenario) {
			return func(s *erScenario) {
				setMember(t, s.jobPages[0][0], "status", status)
				setMember(t, s.jobPages[0][0], "conclusion", conclusion)
			}
		}
		tests := []struct {
			name     string
			apply    func(*erScenario)
			wantKind string
		}{
			{"in progress", withJob("in_progress", nil), ""},
			{"alongside other jobs", func(s *erScenario) {
				s.jobPages = [][]map[string]any{{scenarioJob(t, "verify"), s.jobPages[0][0], scenarioJob(t, "publish")}}
			}, ""},
			{"no job carries the name", func(s *erScenario) { s.jobPages = [][]map[string]any{{}} }, "gated-job-not-found"},
			{"only a differently cased name", func(s *erScenario) { setMember(t, s.jobPages[0][0], "name", "Close") }, "gated-job-not-found"},
			{"two jobs carry the name", func(s *erScenario) {
				second := scenarioJob(t, "close")
				setMember(t, second, "id", 399444497)
				s.jobPages = [][]map[string]any{{s.jobPages[0][0], second}}
			}, "gated-job-ambiguous"},
			{"queued", withJob("queued", nil), "gated-job-not-eligible"},
			{"waiting for review", withJob("waiting", nil), "gated-job-not-eligible"},
			{"completed with failure", withJob("completed", "failure"), "gated-job-not-eligible"},
			{"completed cancelled", withJob("completed", "cancelled"), "gated-job-not-eligible"},
			{"completed skipped", withJob("completed", "skipped"), "gated-job-not-eligible"},
			{"completed with no conclusion", withJob("completed", nil), "gated-job-not-eligible"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				s := newERScenario(t)
				tt.apply(s)
				_, rows, disclosures := reviewScenario(t, s, erQuery())
				if tt.wantKind == "" {
					if len(rows) != 1 || disclosures != nil {
						t.Fatalf("rows = %v disclosures = %v, want one row and no disclosure", approvalIDs(rows), disclosures)
					}
					return
				}
				if len(rows) != 0 {
					t.Fatalf("rows = %v, want none", approvalIDs(rows))
				}
				if kinds := disclosureKinds(disclosures); strings.Join(kinds, ",") != tt.wantKind {
					t.Fatalf("disclosure kinds = %v (%v), want exactly %s", kinds, disclosures, tt.wantKind)
				}
				if !disclosuresContain(disclosures, "gated_job=close") {
					t.Fatalf("disclosures = %v, want the gated job named", disclosures)
				}
			})
		}
	})

	t.Run("github yields no row, with the bypass disclosed, when no review approved the environment (I-3)", func(t *testing.T) {
		tests := []struct {
			name              string
			entries           func(t *testing.T) []map[string]any
			rejected, pending int
		}{
			{"rejected", func(t *testing.T) []map[string]any {
				return []map[string]any{historyEntry(t, "rejected", erEnvIDNum, "close", erReviewerNum)}
			}, 1, 0},
			{"pending", func(t *testing.T) []map[string]any {
				return []map[string]any{historyEntry(t, "pending", erEnvIDNum, "close", erReviewerNum)}
			}, 0, 1},
			{"absent, as after an administrator bypass", func(t *testing.T) []map[string]any { return []map[string]any{} }, 0, 0},
			{"approved only for another environment", func(t *testing.T) []map[string]any {
				return []map[string]any{historyEntry(t, "approved", 8, "staging", erReviewerNum)}
			}, 0, 0},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				s := newERScenario(t)
				s.historyPages = [][]map[string]any{tt.entries(t)}
				_, rows, disclosures := reviewScenario(t, s, erQuery())
				if len(rows) != 0 {
					t.Fatalf("rows = %v, want none", approvalIDs(rows))
				}
				want := fmt.Sprintf("rejected=%d pending=%d", tt.rejected, tt.pending)
				if !disclosuresContain(disclosures, "environment-review:no-approved-review:") || !disclosuresContain(disclosures, want) || !disclosuresContain(disclosures, "bypassed") {
					t.Fatalf("disclosures = %v, want no-approved-review with %s naming the bypass case", disclosures, want)
				}
			})
		}
	})

	t.Run("github never counts a review of another environment (m-2)", func(t *testing.T) {
		tests := []struct {
			name    string
			entries func(t *testing.T) []map[string]any
		}{
			{"another environment approved by another reviewer", func(t *testing.T) []map[string]any {
				return []map[string]any{historyEntry(t, "approved", 8, "staging", 2), historyEntry(t, "approved", erEnvIDNum, "close", erReviewerNum)}
			}},
			{"another environment id sharing the close name", func(t *testing.T) []map[string]any {
				return []map[string]any{historyEntry(t, "approved", 8, "close", 2), historyEntry(t, "approved", erEnvIDNum, "close", erReviewerNum)}
			}},
			{"the close environment under a former name", func(t *testing.T) []map[string]any {
				return []map[string]any{historyEntry(t, "approved", erEnvIDNum, "close-renamed", erReviewerNum)}
			}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				s := newERScenario(t)
				s.historyPages = [][]map[string]any{tt.entries(t)}
				_, rows, _ := reviewScenario(t, s, erQuery())
				if got := strings.Join(approvalIDs(rows), ","); got != scenarioApprovalID(erReviewer) {
					t.Fatalf("approval ids = %s, want only %s", got, scenarioApprovalID(erReviewer))
				}
			})
		}
	})

	t.Run("github cross-checks the run and environment it read", func(t *testing.T) {
		tests := []struct {
			name    string
			apply   func(*erScenario)
			wantErr string
		}{
			{"the run reports another id", func(s *erScenario) { setMember(t, s.run, "id", 30433643) }, "reported id 30433643"},
			{"the environment reports another name", func(s *erScenario) { setMember(t, s.env, "name", "staging") }, `reported name "staging"`},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				s := newERScenario(t)
				tt.apply(s)
				facts, err := s.adapter().EnvironmentReview(context.Background(), erQuery())
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("EnvironmentReview = %+v, %v; want an error containing %q", facts, err, tt.wantErr)
				}
			})
		}
		t.Run("the environment reports its name in another case", func(t *testing.T) {
			s := newERScenario(t)
			setMember(t, s.env, "name", "Close")
			facts, rows, _ := reviewScenario(t, s, erQuery())
			if facts.EnvironmentName != "Close" || len(rows) != 1 {
				t.Fatalf("facts = %+v rows = %v, want GitHub's case-insensitive name accepted", facts, approvalIDs(rows))
			}
		})
	})

	t.Run("github yields no row when the gated job's creation stamp is unavailable (SI-232)", func(t *testing.T) {
		s := newERScenario(t)
		s.jobPages = [][]map[string]any{{publishedJobObject(t)}}
		setMember(t, s.jobPages[0][0], "name", "close")
		facts, rows, disclosures := reviewScenario(t, s, erQuery())
		if facts.GatedJobCreatedAt != "" || facts.GatedJobStartedAt != erStartedAt {
			t.Fatalf("stamps = created %q started %q, want no creation stamp and the published start", facts.GatedJobCreatedAt, facts.GatedJobStartedAt)
		}
		if len(rows) != 0 {
			t.Fatalf("rows = %v, want none: an unavailable creation stamp never becomes an approval instant", approvalIDs(rows))
		}
		if kinds := disclosureKinds(disclosures); strings.Join(kinds, ",") != "creation-stamp-unavailable" {
			t.Fatalf("disclosure kinds = %v (%v)", kinds, disclosures)
		}
	})

	t.Run("github drains every page of jobs and review history", func(t *testing.T) {
		s := newERScenario(t)
		s.jobPages = [][]map[string]any{{scenarioJob(t, "verify")}, {scenarioJob(t, "close")}}
		s.historyPages = [][]map[string]any{
			{historyEntry(t, "approved", 8, "staging", 7)},
			{historyEntry(t, "approved", erEnvIDNum, "close", erReviewerNum)},
		}
		facts, rows, _ := reviewScenario(t, s, erQuery())
		if facts.GatedJobCount != 1 || facts.GatedJobCreatedAt != erCreatedAt {
			t.Fatalf("gated job found on page 2 = %+v", facts)
		}
		if got := strings.Join(approvalIDs(rows), ","); got != scenarioApprovalID(erReviewer) {
			t.Fatalf("approval ids = %s, want the approval found on history page 2", got)
		}
		if log := strings.Join(s.requestLog(), ","); log != "history,history,jobs,jobs,environment,run" {
			t.Fatalf("requests = %s, want both pages of history and jobs", log)
		}
	})

	t.Run("github rows are deterministic and sorted by approval id", func(t *testing.T) {
		s := newERScenario(t)
		s.historyPages = [][]map[string]any{{
			historyEntry(t, "approved", erEnvIDNum, "close", 9),
			historyEntry(t, "approved", erEnvIDNum, "close", erReviewerNum),
		}}
		facts, first, _ := reviewScenario(t, s, erQuery())
		second, _ := normalizeEnvReview(t, facts)
		want := []string{scenarioApprovalID(erReviewer), scenarioApprovalID("9")}
		if !reflect.DeepEqual(approvalIDs(first), want) || !reflect.DeepEqual(first, second) {
			t.Fatalf("rows = %v then %v, want %v both times", approvalIDs(first), approvalIDs(second), want)
		}
	})
}

// TestEnvironmentReviewApprovalContract_Behavioral covers the GitLab adapter
// and the fake behind the same port. It is not an obligation producer: the
// v2 ac-4 behavioral obligation's producer is
// go-test:internal/lifecyclecountersign:TestSoloEnvironmentReviewCountersign_Behavioral
// (lane L2c), which drives the resolver over these facts.
func TestEnvironmentReviewApprovalContract_Behavioral(t *testing.T) {
	t.Run("gitlab environment review is unsupported with a disclosure and never an error", func(t *testing.T) {
		a := gitlab.New(gitlab.Config{BaseURL: "http://unused.invalid", ProjectID: "42", Clock: fixedClock})
		facts, err := a.EnvironmentReview(context.Background(), erQuery())
		if err != nil {
			t.Fatalf("EnvironmentReview: %v, want nil error (never breaks a close)", err)
		}
		if facts.Supported {
			t.Fatalf("Supported = true, want false (unsupported source)")
		}
		if facts.UnsupportedReason == "" {
			t.Fatal("UnsupportedReason is empty, want a disclosed reason")
		}
		rows, disclosures := normalizeEnvReview(t, facts)
		if len(rows) != 0 {
			t.Fatalf("rows = %+v, want none", rows)
		}
		if kinds := disclosureKinds(disclosures); strings.Join(kinds, ",") != "unsupported-forge" {
			t.Fatalf("disclosures = %v, want one unsupported-forge witness", disclosures)
		}
	})

	t.Run("fake seeds and returns independent environment review facts", func(t *testing.T) {
		seed, err := forge.NewEnvironmentReviewFacts(validFactsDraft(), fixedClock())
		if err != nil {
			t.Fatalf("fixture: %v", err)
		}
		f := fake.New()
		if err := f.SeedEnvironmentReviewFacts(erQuery(), seed); err != nil {
			t.Fatalf("SeedEnvironmentReviewFacts: %v", err)
		}
		first, err := f.EnvironmentReview(context.Background(), erQuery())
		if err != nil {
			t.Fatalf("EnvironmentReview: %v", err)
		}
		if !reflect.DeepEqual(first, seed) {
			t.Fatalf("fake returned %+v, want the seeded facts %+v", first, seed)
		}
		first.Reviews[0].ProviderState = forge.EnvironmentReviewRejected
		*first.EnvironmentPreventSelfReview = true
		second, err := f.EnvironmentReview(context.Background(), erQuery())
		if err != nil {
			t.Fatalf("EnvironmentReview second: %v", err)
		}
		if second.Reviews[0].ProviderState != forge.EnvironmentReviewApproved || *second.EnvironmentPreventSelfReview {
			t.Fatalf("fake returned aliased facts: %+v", second)
		}
	})

	t.Run("fake refuses seeds a real adapter could not produce (m-3)", func(t *testing.T) {
		valid, err := forge.NewEnvironmentReviewFacts(validFactsDraft(), fixedClock())
		if err != nil {
			t.Fatalf("fixture: %v", err)
		}
		tampered := valid
		tampered.RunHeadSHA = candidateA
		query := func(edit func(*forge.EnvironmentReviewQuery)) forge.EnvironmentReviewQuery {
			q := erQuery()
			edit(&q)
			return q
		}
		tests := []struct {
			name  string
			query forge.EnvironmentReviewQuery
			facts forge.EnvironmentReviewFacts
		}{
			{"unvalidated facts", erQuery(), forge.EnvironmentReviewFacts{Supported: true, RunAttempt: 1}},
			{"facts changed after their digest", erQuery(), tampered},
			{"facts for another run", query(func(q *forge.EnvironmentReviewQuery) { q.RunID = "30433643" }), valid},
			{"facts for another attempt", query(func(q *forge.EnvironmentReviewQuery) { q.RunAttempt = 2 }), valid},
			{"facts for another environment", query(func(q *forge.EnvironmentReviewQuery) { q.EnvironmentName = "staging" }), valid},
			{"facts for another gated job", query(func(q *forge.EnvironmentReviewQuery) { q.GatedJobName = "verify" }), valid},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				f := fake.New()
				if err := f.SeedEnvironmentReviewFacts(tt.query, tt.facts); err == nil {
					t.Fatalf("SeedEnvironmentReviewFacts(%s): want error, got nil", tt.name)
				}
				if got, err := f.EnvironmentReview(context.Background(), tt.query); err == nil {
					t.Fatalf("EnvironmentReview after a refused seed returned %+v, want the unseeded error", got)
				}
			})
		}
	})

	t.Run("fake errors on an unseeded environment review query", func(t *testing.T) {
		if _, err := fake.New().EnvironmentReview(context.Background(), erQuery()); err == nil {
			t.Fatal("EnvironmentReview unseeded: want error, got nil")
		}
	})
}
