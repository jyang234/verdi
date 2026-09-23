package lifecyclecountersign

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jyang234/verdi/internal/countersign"
	"github.com/jyang234/verdi/internal/forge"
	forgefake "github.com/jyang234/verdi/internal/forge/fake"
	forgegithub "github.com/jyang234/verdi/internal/forge/github"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
)

// The canned GitHub observation every environment-review case below starts
// from: the dispatch-only close workflow's run 5551, first attempt, whose
// gated `close` job is in progress, and whose `close` environment (id
// 161088068) the owner (GitHub user 900, also the pull request's author)
// approved. GitHub refuses an author's approval of their own pull request,
// so the pull request carries no review at all.
const (
	closeRunID       int64 = 5551
	otherCloseRunID  int64 = 5550
	closeEnvID       int64 = 161088068
	closeRunJobID    int64 = 7001
	closeOwnerID     int64 = 900
	closeReviewerID  int64 = 101
	closeRunAttempt1       = "1"
	closeRunPath           = ".github/workflows/close.yml@refs/heads/close/candidate"
)

// closeRunApprovalID is the ac-4 composite identity of the owner's approval
// of run 5551's first attempt.
var closeRunApprovalID = fmt.Sprintf("github-environment-review:acme/widgets:%d:1:%d:%d", closeRunID, closeEnvID, closeOwnerID)

// githubReviewEntry is one entry of GitHub's review history for a run.
type githubReviewEntry struct {
	State   string
	EnvID   int64
	EnvName string
	UserID  int64
}

// githubJob is one job of the queried run attempt.
type githubJob struct {
	ID         int64
	Name       string
	Status     string
	Conclusion string
	CreatedAt  string
	StartedAt  string
}

// githubCloseRun is one canned GitHub observation: the pull request the
// countersign reads, the review history, jobs, and run facts of a workflow
// run, and the CI environment the process runs in.
type githubCloseRun struct {
	PRHead        string
	HistoryRunID  int64
	History       []githubReviewEntry
	LatestAttempt int
	RunHead       string
	Event         string
	Path          string
	Status        string
	Conclusion    string
	Jobs          []githubJob
	Env           map[string]string
}

func closeStamp(offset time.Duration) string {
	return lifecycleNow().Add(offset).UTC().Format(time.RFC3339Nano)
}

// approvedCloseRun is the proving observation: the owner's approved review
// of the close run's first attempt, dispatched five minutes ago and started
// a minute later, whose head is the pull request's head and local HEAD.
func approvedCloseRun() githubCloseRun {
	return githubCloseRun{
		PRHead:        lifecycleCandidateSHA,
		HistoryRunID:  closeRunID,
		History:       []githubReviewEntry{{State: "approved", EnvID: closeEnvID, EnvName: "close", UserID: closeOwnerID}},
		LatestAttempt: 1,
		RunHead:       lifecycleCandidateSHA,
		Event:         "workflow_dispatch",
		Path:          closeRunPath,
		Status:        "in_progress",
		Jobs: []githubJob{{
			ID: closeRunJobID, Name: "close", Status: "in_progress",
			CreatedAt: closeStamp(-5 * time.Minute), StartedAt: closeStamp(-4 * time.Minute),
		}},
		Env: map[string]string{"GITHUB_RUN_ID": strconv.FormatInt(closeRunID, 10), "GITHUB_RUN_ATTEMPT": closeRunAttempt1},
	}
}

// githubCloseServer serves one githubCloseRun over GitHub's REST shapes and
// records every request it receives. It answers only GET; any other method
// is a write attempt and is counted.
type githubCloseServer struct {
	*httptest.Server
	mu       sync.Mutex
	paths    []string
	mutating int
}

var (
	closeRunApprovalsPath = regexp.MustCompile(`^/repos/acme/widgets/actions/runs/(\d+)/approvals$`)
	closeRunJobsPath      = regexp.MustCompile(`^/repos/acme/widgets/actions/runs/(\d+)/attempts/(\d+)/jobs$`)
	closeRunPathRe        = regexp.MustCompile(`^/repos/acme/widgets/actions/runs/(\d+)$`)
)

func newGitHubCloseServer(t *testing.T, run githubCloseRun) *githubCloseServer {
	t.Helper()
	server := &githubCloseServer{}
	server.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.mu.Lock()
		server.paths = append(server.paths, r.Method+" "+r.URL.Path)
		if r.Method != http.MethodGet {
			server.mutating++
		}
		server.mu.Unlock()
		if r.Method != http.MethodGet {
			http.Error(w, "read-only fixture", http.StatusMethodNotAllowed)
			return
		}
		body, ok := run.respond(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	return server
}

func (s *githubCloseServer) requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string{}, s.paths...)
}

func (s *githubCloseServer) writes() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mutating
}

// environmentReviewReads returns the requests that read a workflow run or
// an environment: the environment-review source's only endpoints.
func (s *githubCloseServer) environmentReviewReads() []string {
	var out []string
	for _, request := range s.requests() {
		if strings.Contains(request, "/actions/") || strings.Contains(request, "/environments/") {
			out = append(out, request)
		}
	}
	return out
}

func mustJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// respond returns the canned GitHub response body for path.
func (run githubCloseRun) respond(path string) ([]byte, bool) {
	switch path {
	case "/repos/acme/widgets/pulls":
		return []byte(`[{"number":17,"title":"close candidate","head":{"ref":"feature/candidate"}}]`), true
	case "/repos/acme/widgets/pulls/17":
		return mustJSON(map[string]any{"number": 17, "head": map[string]any{"sha": run.PRHead}, "user": map[string]any{"id": closeOwnerID}}), true
	case "/repos/acme/widgets/pulls/17/reviews":
		return []byte(`[]`), true
	case "/repos/acme/widgets/environments/close":
		return mustJSON(map[string]any{
			"id": closeEnvID, "name": "close",
			"protection_rules": []any{map[string]any{"id": 3, "type": "required_reviewers", "prevent_self_review": false, "reviewers": []any{}}},
		}), true
	}
	if m := closeRunApprovalsPath.FindStringSubmatch(path); m != nil {
		entries := []any{}
		if m[1] == strconv.FormatInt(run.HistoryRunID, 10) {
			for _, entry := range run.History {
				entries = append(entries, map[string]any{
					"state":        entry.State,
					"comment":      "",
					"environments": []any{map[string]any{"id": entry.EnvID, "name": entry.EnvName}},
					"user":         map[string]any{"id": entry.UserID, "login": fmt.Sprintf("u%d", entry.UserID)},
				})
			}
		}
		return mustJSON(entries), true
	}
	if m := closeRunJobsPath.FindStringSubmatch(path); m != nil {
		jobs := []any{}
		for _, job := range run.Jobs {
			object := map[string]any{"id": job.ID, "name": job.Name, "status": job.Status, "conclusion": nullable(job.Conclusion), "started_at": job.StartedAt}
			if job.CreatedAt != "" {
				object["created_at"] = job.CreatedAt
			}
			jobs = append(jobs, object)
		}
		return mustJSON(map[string]any{"total_count": len(jobs), "jobs": jobs}), true
	}
	if m := closeRunPathRe.FindStringSubmatch(path); m != nil {
		id, _ := strconv.ParseInt(m[1], 10, 64)
		return mustJSON(map[string]any{
			"id": id, "run_attempt": run.LatestAttempt, "head_sha": run.RunHead,
			"html_url": fmt.Sprintf("https://github.com/acme/widgets/actions/runs/%d", id),
			"event":    run.Event, "path": run.Path, "status": nullable(run.Status), "conclusion": nullable(run.Conclusion),
		}), true
	}
	return nil, false
}

// recordingForge records every forge port call the resolver makes.
// PostComment is the port's only write; the others are reads.
type recordingForge struct {
	forge.Forge
	mu    sync.Mutex
	calls []string
}

func (f *recordingForge) record(method string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, method)
}

func (f *recordingForge) called(method string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, call := range f.calls {
		if call == method {
			return true
		}
	}
	return false
}

func (f *recordingForge) ListApprovals(ctx context.Context, changeID string) (forge.ApprovalSnapshot, error) {
	f.record("ListApprovals")
	return f.Forge.ListApprovals(ctx, changeID)
}

func (f *recordingForge) EnvironmentReview(ctx context.Context, query forge.EnvironmentReviewQuery) (forge.EnvironmentReviewFacts, error) {
	f.record("EnvironmentReview")
	return f.Forge.EnvironmentReview(ctx, query)
}

func (f *recordingForge) FetchEvidenceBundle(ctx context.Context, ref, commit string) (forge.DerivedTree, error) {
	f.record("FetchEvidenceBundle")
	return f.Forge.FetchEvidenceBundle(ctx, ref, commit)
}

func (f *recordingForge) GeneratedAttribute() string {
	f.record("GeneratedAttribute")
	return f.Forge.GeneratedAttribute()
}

func (f *recordingForge) CIContext(ctx context.Context) (forge.CIInfo, error) {
	f.record("CIContext")
	return f.Forge.CIContext(ctx)
}

func (f *recordingForge) ListOpenMRs(ctx context.Context, targetBranch string) ([]forge.OpenMR, error) {
	f.record("ListOpenMRs")
	return f.Forge.ListOpenMRs(ctx, targetBranch)
}

func (f *recordingForge) FetchFileAtRef(ctx context.Context, ref, path string) ([]byte, error) {
	f.record("FetchFileAtRef")
	return f.Forge.FetchFileAtRef(ctx, ref, path)
}

func (f *recordingForge) ListComments(ctx context.Context, mrID string) ([]forge.Comment, error) {
	f.record("ListComments")
	return f.Forge.ListComments(ctx, mrID)
}

func (f *recordingForge) PostComment(ctx context.Context, mrID, body string, target *forge.CommentTarget) (forge.Comment, error) {
	f.record("PostComment")
	return f.Forge.PostComment(ctx, mrID, body, target)
}

func (f *recordingForge) GetThreadResolution(ctx context.Context, mrID string) ([]forge.ThreadResolution, error) {
	f.record("GetThreadResolution")
	return f.Forge.GetThreadResolution(ctx, mrID)
}

// githubCloseRunFixture resolves the story countersign through the real
// GitHub adapter over run's canned responses, under source's selected
// profile.
func githubCloseRunFixture(t *testing.T, run githubCloseRun, source fstest.MapFS) (Resolver, Request, *githubCloseServer, *recordingForge) {
	t.Helper()
	server := newGitHubCloseServer(t, run)
	adapter := forgegithub.New(forgegithub.Config{
		BaseURL: server.URL, Owner: "acme", Repo: "widgets", Token: "test-token",
		Getenv: func(key string) string { return run.Env[key] },
		Clock:  lifecycleNow,
	})
	recording := &recordingForge{Forge: adapter}
	mdl := &model.Model{Lifecycle: map[string]model.Lifecycle{
		"story": {Transitions: []model.Transition{{Verb: "close", Obligations: []model.Obligation{{Scheme: "attestation", Kind: "countersign", Count: 1}}}}},
	}}
	manifest := &store.Manifest{Countersign: &store.CountersignConfig{
		TrustSource: "forge-live", FreshnessPolicyID: "forge-current",
		MaximumObservationAgeSeconds: 300, MaximumApprovalAgeSeconds: 3600,
	}}
	resolver := Resolver{Forge: recording, Clock: lifecycleNow}
	request := Request{
		Root: "/candidate", Manifest: manifest, Model: mdl, TargetClass: "story",
		DefaultBranch: "main", SourceBranch: "feature/candidate", LocalCandidateSHA: lifecycleCandidateSHA,
		AcceptedBranch: "main", AcceptedCommit: lifecycleAcceptedCommit, AcceptedProfileSource: source,
	}
	return resolver, request, server, recording
}

// soloCollapseWitness is the kernel's solo role-collapse disclosure the
// countersign record must carry whenever the owner's own review counts.
func soloCollapseWitness(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf(`kernel-separation-probe:author-as-approver:collapse-permitted:kernel_disclosure="solo-role-collapse":author_principal_id=%q:roles=["author" "story-review"]`, lifecyclePrincipalID(t, "900"))
}

func recordApproval(record *countersign.Record, approvalID string) (countersign.ApprovalRecord, bool) {
	for _, approval := range record.Approvals {
		if approval.ApprovalID == approvalID {
			return approval, true
		}
	}
	return countersign.ApprovalRecord{}, false
}

func providerWitness(approval countersign.ApprovalRecord, name string) string {
	for _, witness := range approval.ProviderWitnesses {
		if witness.Name == name {
			return witness.Value
		}
	}
	return ""
}

// TestSoloEnvironmentReviewCountersign_Behavioral is the exact behavioral
// producer of spec/vatc-forge-countersign-v2 ac-4
// (go-test:internal/lifecyclecountersign:TestSoloEnvironmentReviewCountersign_Behavioral).
// It drives the lifecycle resolver over the governance kernel with solo,
// team, and high-assurance profiles and the real GitHub adapter over canned
// pull-request, run, attempt-specific job, environment, and review-history
// responses. Only under a solo profile whose rules let the owner fill both
// the author and approver roles does the owner's approved review of the
// dispatch-only close run's first attempt prove the countersign, with the
// kernel's solo role-collapse disclosure in the witnesses; every other case
// yields no approval with its witness, and no case writes or requests an
// approval.
func TestSoloEnvironmentReviewCountersign_Behavioral(t *testing.T) {
	solo := lifecycleSoloAcceptedSource
	soloRefusingCollapse := func() fstest.MapFS {
		return kernelProbeSource("solo",
			kernelProbeMappings(kernelProbeMapping("author", "900"), kernelProbeMapping("story-review", "900")),
			"\n  - {transitions: [close], left_role: author, right_role: story-review, relation: different-principal}")
	}
	team := func() fstest.MapFS { return lifecycleAcceptedSource(`"101", "900"`) }
	otherHead := strings.Repeat("c", 40)

	for _, tc := range []struct {
		name string
		// source is the accepted tree's selected profile.
		source func() fstest.MapFS
		// mutate changes the proving observation.
		mutate func(*githubCloseRun)
		// maximumApprovalAge overrides the configured ceiling when nonzero.
		maximumApprovalAge int64
		// wantProven is whether the countersign is proven by the review.
		wantProven bool
		// wantRequested is whether the environment review is read at all.
		wantRequested bool
		// wantWitnesses must each appear in the record's witnesses.
		wantWitnesses []string
	}{
		{
			name:   "solo: the owner's approved review of the close run's first attempt proves",
			source: solo, wantProven: true, wantRequested: true,
			wantWitnesses: []string{
				fmt.Sprintf("approval-candidate:approval_id=%q:match=true", closeRunApprovalID),
				fmt.Sprintf("environment-review:candidate-binding: approval_id=%q run_head_sha=%q change_head_sha=%q local_candidate_sha=%q match=true", closeRunApprovalID, lifecycleCandidateSHA, lifecycleCandidateSHA, lifecycleCandidateSHA),
				fmt.Sprintf("environment-review:observation: run_id=%d run_attempt=1 environment=close gated_job=close", closeRunID),
			},
		},
		{
			name:   "solo: an approval beside another reviewer's pending review proves and carries the pending disclosure",
			source: solo,
			mutate: func(run *githubCloseRun) {
				run.History = append(run.History, githubReviewEntry{State: "pending", EnvID: closeEnvID, EnvName: "close", UserID: closeReviewerID})
			},
			wantProven: true, wantRequested: true,
			wantWitnesses: []string{fmt.Sprintf("environment-review:review-not-approved: run_id=%d run_attempt=1 environment_id=%d reviewer=%d state=pending", closeRunID, closeEnvID, closeReviewerID)},
		},
		{
			name:   "team: the author's own approved review is never requested or counted",
			source: team, wantRequested: false,
		},
		{
			name:   "team: an independent story reviewer's approved review is never requested or counted",
			source: team,
			mutate: func(run *githubCloseRun) {
				run.History = []githubReviewEntry{{State: "approved", EnvID: closeEnvID, EnvName: "close", UserID: closeReviewerID}}
			},
			wantRequested: false,
		},
		{
			name:   "high-assurance: an independent story reviewer's approved review is never requested or counted",
			source: lifecycleHighAssuranceAcceptedSource,
			mutate: func(run *githubCloseRun) {
				run.History = []githubReviewEntry{{State: "approved", EnvID: closeEnvID, EnvName: "close", UserID: closeReviewerID}}
			},
			wantRequested: false,
		},
		{
			name:   "solo whose own rules refuse the collapse: the review is never requested or counted",
			source: soloRefusingCollapse, wantRequested: false,
			wantWitnesses: []string{`kernel-separation-probe:author-as-approver:separation-required:kernel_answer="refused":reason="distinctness-violated"`},
		},
		{
			name:          "solo: a rejected review is no approval",
			source:        solo,
			mutate:        func(run *githubCloseRun) { run.History[0].State = "rejected" },
			wantRequested: true,
			wantWitnesses: []string{"environment-review:no-approved-review:", "environment-review:review-not-approved:", "state=rejected"},
		},
		{
			name:          "solo: a pending review is no approval",
			source:        solo,
			mutate:        func(run *githubCloseRun) { run.History[0].State = "pending" },
			wantRequested: true,
			wantWitnesses: []string{"environment-review:no-approved-review:", "state=pending"},
		},
		{
			name:          "solo: an absent review is no approval",
			source:        solo,
			mutate:        func(run *githubCloseRun) { run.History = nil },
			wantRequested: true,
			wantWitnesses: []string{"environment-review:no-approved-review:", "reviews=0"},
		},
		{
			name:   "solo: a bypassed protection rule (the job runs, no review entry) is no approval",
			source: solo,
			mutate: func(run *githubCloseRun) {
				run.History = nil
				run.Jobs[0].StartedAt = closeStamp(-5 * time.Minute)
			},
			wantRequested: true,
			wantWitnesses: []string{"environment-review:no-approved-review:"},
		},
		{
			name:   "solo: an approval of another environment only is no approval",
			source: solo,
			mutate: func(run *githubCloseRun) {
				run.History = []githubReviewEntry{{State: "approved", EnvID: 999, EnvName: "production", UserID: closeOwnerID}}
			},
			wantRequested: true,
			wantWitnesses: []string{"environment-review:no-approved-review:"},
		},
		{
			name:   "solo: a cancelled run is no approval",
			source: solo,
			mutate: func(run *githubCloseRun) {
				run.Status, run.Conclusion = "completed", "cancelled"
				run.Jobs[0].Status, run.Jobs[0].Conclusion = "completed", "cancelled"
			},
			wantRequested: true,
			wantWitnesses: []string{"environment-review:run-not-eligible:", "conclusion=cancelled"},
		},
		{
			name:          "solo: another run's approved review is no approval for the current run",
			source:        solo,
			mutate:        func(run *githubCloseRun) { run.HistoryRunID = otherCloseRunID },
			wantRequested: true,
			wantWitnesses: []string{"environment-review:no-approved-review:", fmt.Sprintf("run_id=%d", closeRunID)},
		},
		{
			name:   "solo: a rerun attempt's review is no approval",
			source: solo,
			mutate: func(run *githubCloseRun) {
				run.Env["GITHUB_RUN_ATTEMPT"] = "2"
				run.LatestAttempt = 2
			},
			wantRequested: true,
			wantWitnesses: []string{"environment-review:rerun-not-honored:", "run_attempt=2"},
		},
		{
			name:          "solo: the first attempt's review is no approval once the run was rerun",
			source:        solo,
			mutate:        func(run *githubCloseRun) { run.LatestAttempt = 2 },
			wantRequested: true,
			wantWitnesses: []string{"environment-review:rerun-not-honored:", "latest_run_attempt=2"},
		},
		{
			name:          "solo: a run head other than the change head and local HEAD does not prove",
			source:        solo,
			mutate:        func(run *githubCloseRun) { run.RunHead = otherHead },
			wantRequested: true,
			wantWitnesses: []string{
				fmt.Sprintf("approval-candidate:approval_id=%q:match=false", closeRunApprovalID),
				fmt.Sprintf("environment-review:candidate-binding: approval_id=%q run_head_sha=%q change_head_sha=%q local_candidate_sha=%q match=false", closeRunApprovalID, otherHead, lifecycleCandidateSHA, lifecycleCandidateSHA),
			},
		},
		{
			name:          "solo: a run head equal to the change head but not the local commit does not prove",
			source:        solo,
			mutate:        func(run *githubCloseRun) { run.RunHead, run.PRHead = otherHead, otherHead },
			wantRequested: true,
			wantWitnesses: []string{
				fmt.Sprintf("candidate-sha-mismatch:snapshot=%q:local=%q", otherHead, lifecycleCandidateSHA),
				fmt.Sprintf("environment-review:candidate-binding: approval_id=%q run_head_sha=%q change_head_sha=%q local_candidate_sha=%q match=false", closeRunApprovalID, otherHead, otherHead, lifecycleCandidateSHA),
			},
		},
		{
			name:   "solo: a delayed dispatch dates the approval by the job's creation, older than the configured maximum",
			source: solo,
			mutate: func(run *githubCloseRun) {
				run.Jobs[0].CreatedAt = closeStamp(-2 * time.Hour)
				run.Jobs[0].StartedAt = closeStamp(-time.Minute)
			},
			wantRequested: true,
			wantWitnesses: []string{fmt.Sprintf("approval-freshness:approval_id=%q:approved_at=%q:updated_at=%q:evaluated_at=%q:maximum_age_seconds=3600:verdict=%q", closeRunApprovalID, closeStamp(-2*time.Hour), closeStamp(-2*time.Hour), closeStamp(0), countersign.VerdictViolated)},
		},
		{
			name:   "solo: the same delayed dispatch proves under a ceiling above its age",
			source: solo,
			mutate: func(run *githubCloseRun) {
				run.Jobs[0].CreatedAt = closeStamp(-2 * time.Hour)
				run.Jobs[0].StartedAt = closeStamp(-time.Minute)
			},
			maximumApprovalAge: 3 * 3600, wantProven: true, wantRequested: true,
		},
		{
			name:          "solo: a missing creation stamp is no approval",
			source:        solo,
			mutate:        func(run *githubCloseRun) { run.Jobs[0].CreatedAt = "" },
			wantRequested: true,
			wantWitnesses: []string{"environment-review:creation-stamp-unavailable:"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := approvedCloseRun()
			if tc.mutate != nil {
				tc.mutate(&run)
			}
			resolver, request, server, recording := githubCloseRunFixture(t, run, tc.source())
			if tc.maximumApprovalAge != 0 {
				request.Manifest.Countersign.MaximumApprovalAgeSeconds = tc.maximumApprovalAge
			}

			result, err := resolver.Resolve(context.Background(), request)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if result.Record == nil {
				t.Fatalf("result = %+v, want a canonical countersign record", result)
			}
			record := result.Record

			// The resolver never writes or requests an approval (co-3).
			if server.writes() != 0 || recording.called("PostComment") {
				t.Fatalf("the resolver wrote to the forge: requests=%v calls=%v", server.requests(), recording.calls)
			}

			// Environment reviews are read only under a collapse-permitting
			// solo profile, and only for the current run.
			reads := server.environmentReviewReads()
			if tc.wantRequested {
				attempt := run.Env["GITHUB_RUN_ATTEMPT"]
				for _, want := range []string{
					fmt.Sprintf("GET /repos/acme/widgets/actions/runs/%d/approvals", closeRunID),
					fmt.Sprintf("GET /repos/acme/widgets/actions/runs/%d/attempts/%s/jobs", closeRunID, attempt),
					"GET /repos/acme/widgets/environments/close",
					fmt.Sprintf("GET /repos/acme/widgets/actions/runs/%d", closeRunID),
				} {
					if !containsExact(reads, want) {
						t.Errorf("environment-review reads = %v, want %q", reads, want)
					}
				}
				for _, read := range reads {
					if strings.Contains(read, strconv.FormatInt(otherCloseRunID, 10)) {
						t.Errorf("the resolver read another run: %q", read)
					}
				}
			} else if len(reads) != 0 || recording.called("EnvironmentReview") {
				t.Fatalf("environment review requested although the profile requires separation: reads=%v calls=%v", reads, recording.calls)
			}

			for _, approval := range record.Approvals {
				if strings.HasPrefix(approval.ApprovalID, "github-environment-review:") && !tc.wantRequested {
					t.Fatalf("an environment review was counted although never requested: %+v", approval)
				}
			}
			eligible := false
			for _, id := range record.Reduction.EligibleApprovalIDs {
				if id == closeRunApprovalID {
					eligible = true
				}
			}

			if tc.wantProven {
				if result.Verdict != countersign.VerdictProven || !eligible || len(record.Reduction.EligibleApprovalIDs) != 1 {
					t.Fatalf("verdict=%q eligible=%v, want the owner's review alone to prove; witnesses=%v", result.Verdict, record.Reduction.EligibleApprovalIDs, record.Witnesses)
				}
				if record.Obligation.SeparationRule != countersign.SeparationNone {
					t.Fatalf("separation rule = %q, want %q", record.Obligation.SeparationRule, countersign.SeparationNone)
				}
				approval, ok := recordApproval(record, closeRunApprovalID)
				if !ok {
					t.Fatalf("approvals = %+v, want %s", record.Approvals, closeRunApprovalID)
				}
				created := run.Jobs[0].CreatedAt
				if approval.State != forge.ApprovalActive || approval.CandidateSHA != lifecycleCandidateSHA || approval.ApprovedAt != created || approval.UpdatedAt != created {
					t.Fatalf("approval row = %+v, want active, the run head, and the gated job's creation stamp %s", approval, created)
				}
				if providerWitness(approval, "provider_state") != "approved" || providerWitness(approval, "approved_at_derivation") == "" || providerWitness(approval, "gated_job_started_at") != run.Jobs[0].StartedAt {
					t.Fatalf("approval provider witnesses = %+v, want the provider state, the derivation disclosures, and the job's start", approval.ProviderWitnesses)
				}
				if !containsExact(record.Witnesses, soloCollapseWitness(t)) {
					t.Fatalf("record witnesses = %v, want the kernel's solo role-collapse disclosure %s", record.Witnesses, soloCollapseWitness(t))
				}
			} else {
				if result.Verdict == countersign.VerdictProven || eligible {
					t.Fatalf("verdict=%q eligible=%v, want no approval; witnesses=%v", result.Verdict, record.Reduction.EligibleApprovalIDs, record.Witnesses)
				}
			}
			for _, want := range tc.wantWitnesses {
				if !containsLifecycleWitness(record.Witnesses, want) {
					t.Errorf("record witnesses = %v, want one containing %s", record.Witnesses, want)
				}
			}
		})
	}

	t.Run("solo: a forge that answers for another run than the current one is refused", func(t *testing.T) {
		resolver, request, server, _ := githubCloseRunFixture(t, approvedCloseRun(), solo())
		resolver.Forge = otherRunForge{Forge: resolver.Forge}
		if _, err := resolver.Resolve(context.Background(), request); err == nil || !strings.Contains(err.Error(), "do not answer the query") {
			t.Fatalf("Resolve error = %v, want a refusal of facts for another run", err)
		}
		if server.writes() != 0 {
			t.Fatalf("the resolver wrote to the forge: %v", server.requests())
		}
	})
}

// otherRunForge answers an environment-review query with the facts of
// another run: a forge contract violation the resolver must refuse.
type otherRunForge struct {
	forge.Forge
}

func (f otherRunForge) EnvironmentReview(ctx context.Context, query forge.EnvironmentReviewQuery) (forge.EnvironmentReviewFacts, error) {
	query.RunID = strconv.FormatInt(otherCloseRunID, 10)
	return f.Forge.EnvironmentReview(ctx, query)
}

func containsExact(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// --- the current-run environment review's disclosure and error paths ------
//
// These drive Resolve over the shared forge fake, whose environment-review
// facts are seeded per exact query, under the solo profile that permits the
// collapse. The change itself carries no approval, so only an environment
// review could prove the countersign.

// fakeCloseRunFacts is the approved first-attempt observation of run 5551
// as facts, before the forge normalizes them.
func fakeCloseRunFacts() forge.EnvironmentReviewFacts {
	return forge.EnvironmentReviewFacts{
		Supported: true, Repository: "acme/widgets",
		RunID: strconv.FormatInt(closeRunID, 10), RunAttempt: 1, LatestRunAttempt: 1,
		RunHeadSHA: lifecycleCandidateSHA, RunURL: "https://github.com/acme/widgets/actions/runs/5551",
		RunEvent: "workflow_dispatch", RunWorkflowPath: ".github/workflows/close.yml", RunStatus: "in_progress",
		EnvironmentID: strconv.FormatInt(closeEnvID, 10), EnvironmentName: "close",
		GatedJobName: "close", GatedJobCount: 1, GatedJobID: strconv.FormatInt(closeRunJobID, 10), GatedJobStatus: "in_progress",
		GatedJobCreatedAt: closeStamp(-5 * time.Minute), GatedJobStartedAt: closeStamp(-4 * time.Minute),
		Reviews: []forge.EnvironmentReviewRow{{
			ReviewerActor: forge.ProviderActor{Scheme: "github-user-id", Subject: "900"},
			ProviderState: forge.EnvironmentReviewApproved, EnvironmentID: strconv.FormatInt(closeEnvID, 10),
		}},
	}
}

// closeRunQuery is the query the resolver must build for run 5551's first
// attempt.
func closeRunQuery() forge.EnvironmentReviewQuery {
	return forge.EnvironmentReviewQuery{RunID: strconv.FormatInt(closeRunID, 10), RunAttempt: 1, EnvironmentName: "close", GatedJobName: "close"}
}

// fakeCloseRunFixture is the solo story countersign over the forge fake: an
// open change authored by the owner with no approval, the CI context ci,
// and draft seeded (normalized) as the answer to closeRunQuery.
func fakeCloseRunFixture(t *testing.T, ci forge.CIInfo, draft forge.EnvironmentReviewFacts) (Resolver, Request, *recordingForge) {
	t.Helper()
	resolver, request := lifecycleFixture(t, "story", "900", "900")
	f := resolver.Forge.(*forgefake.Forge)
	snapshot, err := forge.NewApprovalSnapshot("github", "acme/widgets", "17", lifecycleCandidateSHA, forge.ProviderActor{Scheme: "github-user-id", Subject: "900"}, lifecycleNow(), []forge.Approval{})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	f.SetCIContext(ci)
	facts, err := forge.NewEnvironmentReviewFacts(draft, lifecycleNow())
	if err != nil {
		t.Fatalf("environment review facts: %v", err)
	}
	if err := f.SeedEnvironmentReviewFacts(closeRunQuery(), facts); err != nil {
		t.Fatalf("seed environment review facts: %v", err)
	}
	recording := &recordingForge{Forge: emptyChangeForge{Forge: f, snapshot: snapshot}}
	resolver.Forge = recording
	request.AcceptedProfileSource = lifecycleSoloAcceptedSource()
	return resolver, request, recording
}

// emptyChangeForge answers ListApprovals with snapshot as-is. The shared
// fake returns a seeded empty approval set as nil, which the snapshot
// contract refuses, so the change without approvals is served here.
type emptyChangeForge struct {
	forge.Forge
	snapshot forge.ApprovalSnapshot
}

func (f emptyChangeForge) ListApprovals(context.Context, string) (forge.ApprovalSnapshot, error) {
	return f.snapshot, nil
}

func closeRunCI() forge.CIInfo {
	return forge.CIInfo{Pipeline: strconv.FormatInt(closeRunID, 10), Job: "1", JobName: "close"}
}

// TestResolveEnvironmentReviewOverFakeProves is the fake-backed control for
// the cases below: the seeded approval proves the solo countersign.
func TestResolveEnvironmentReviewOverFakeProves(t *testing.T) {
	resolver, request, _ := fakeCloseRunFixture(t, closeRunCI(), fakeCloseRunFacts())
	result, err := resolver.Resolve(context.Background(), request)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result.Verdict != countersign.VerdictProven || result.Record == nil || !containsExact(result.Record.Reduction.EligibleApprovalIDs, closeRunApprovalID) {
		t.Fatalf("result = %+v, want the seeded environment review to prove", result)
	}
}

// TestResolveEnvironmentReviewCurrentRunUnresolved: when the CI context
// cannot name the current run exactly, no environment review is requested,
// the countersign is not proven, and the record discloses why; it is never
// an error that hides the rest of the countersign.
func TestResolveEnvironmentReviewCurrentRunUnresolved(t *testing.T) {
	for _, tc := range []struct {
		name     string
		pipeline string
		job      string
	}{
		{"outside CI", "", ""},
		{"run id missing beside an attempt", "", "1"},
		{"run id with a leading zero", "05551", "1"},
		{"run id with a sign", "+5551", "1"},
		{"run id not a number", "run-5551", "1"},
		{"run id zero", "0", "1"},
		{"attempt missing", "5551", ""},
		{"attempt zero", "5551", "0"},
		{"attempt with a leading zero", "5551", "01"},
		{"attempt not a number", "5551", "first"},
		{"attempt out of range", "5551", "99999999999999999999"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolver, request, recording := fakeCloseRunFixture(t, forge.CIInfo{Pipeline: tc.pipeline, Job: tc.job}, fakeCloseRunFacts())
			result, err := resolver.Resolve(context.Background(), request)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if result.Verdict == countersign.VerdictProven || result.Record == nil {
				t.Fatalf("result = %+v, want a non-proven record", result)
			}
			if recording.called("EnvironmentReview") {
				t.Fatal("an environment review was requested for an unresolved current run")
			}
			want := fmt.Sprintf("environment-review:current-run-unresolved: run_id=%q run_attempt=%q:", tc.pipeline, tc.job)
			if !containsLifecycleWitness(result.Record.Witnesses, want) {
				t.Fatalf("record witnesses = %v, want %s", result.Record.Witnesses, want)
			}
		})
	}
}

// unavailableEnvironmentReviewForge fails the environment-review read with
// err.
type unavailableEnvironmentReviewForge struct {
	forge.Forge
	err error
}

func (f unavailableEnvironmentReviewForge) EnvironmentReview(context.Context, forge.EnvironmentReviewQuery) (forge.EnvironmentReviewFacts, error) {
	return forge.EnvironmentReviewFacts{}, f.err
}

// ciContextErrorForge fails the CI-context read.
type ciContextErrorForge struct {
	forge.Forge
}

func (ciContextErrorForge) CIContext(context.Context) (forge.CIInfo, error) {
	return forge.CIInfo{}, context.Canceled
}

// TestResolveEnvironmentReviewUnavailableIsDisclosed: an unavailable forge
// read is a disclosed missing approval, never an error and never a pass.
func TestResolveEnvironmentReviewUnavailableIsDisclosed(t *testing.T) {
	resolver, request, _ := fakeCloseRunFixture(t, closeRunCI(), fakeCloseRunFacts())
	resolver.Forge = unavailableEnvironmentReviewForge{Forge: resolver.Forge, err: fmt.Errorf("github: GET environments/close: status 403: %w", forge.ErrUnavailable)}
	result, err := resolver.Resolve(context.Background(), request)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result.Verdict == countersign.VerdictProven || result.Record == nil {
		t.Fatalf("result = %+v, want a non-proven record", result)
	}
	if !containsLifecycleWitness(result.Record.Witnesses, fmt.Sprintf("environment-review:unavailable: run_id=%d run_attempt=1 environment=close gated_job=close:", closeRunID)) {
		t.Fatalf("record witnesses = %v, want the unavailable disclosure", result.Record.Witnesses)
	}
}

// TestResolveEnvironmentReviewContractViolationsAreOperational: a failed CI
// context read, a forge error that is not unavailability, and facts that
// break the facts contract are operational errors, never a verdict.
func TestResolveEnvironmentReviewContractViolationsAreOperational(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Resolver)
		want   string
	}{
		{"CI context read fails", func(r *Resolver) { r.Forge = ciContextErrorForge{Forge: r.Forge} }, "read the current run from the CI context"},
		{"forge read fails without unavailability", func(r *Resolver) {
			r.Forge = unavailableEnvironmentReviewForge{Forge: r.Forge, err: fmt.Errorf("github: decode run: unknown workflow run status %q", "exploded")}
		}, "read the environment review of"},
		{"facts break the facts contract", func(r *Resolver) {
			facts, err := forge.NewEnvironmentReviewFacts(fakeCloseRunFacts(), lifecycleNow())
			if err != nil {
				t.Fatalf("facts: %v", err)
			}
			// Changed after the provider snapshot identity was derived.
			facts.RunHeadSHA = strings.Repeat("d", 40)
			r.Forge = fixedFactsForge{Forge: r.Forge, facts: facts}
		}, fmt.Sprintf("environment review of run_id=%d run_attempt=1", closeRunID)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolver, request, _ := fakeCloseRunFixture(t, closeRunCI(), fakeCloseRunFacts())
			tc.mutate(&resolver)
			if _, err := resolver.Resolve(context.Background(), request); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Resolve error = %v, want an operational error containing %q", err, tc.want)
			}
		})
	}
}

// fixedFactsForge answers every environment-review query with facts as-is,
// so a test can hand the resolver facts the fake's own seeding contract
// would refuse.
type fixedFactsForge struct {
	forge.Forge
	facts forge.EnvironmentReviewFacts
}

func (f fixedFactsForge) EnvironmentReview(context.Context, forge.EnvironmentReviewQuery) (forge.EnvironmentReviewFacts, error) {
	return f.facts, nil
}

// TestResolveEnvironmentReviewRepositoryMismatchIsNoApproval: a review of
// another repository's run is disclosed and never counted.
func TestResolveEnvironmentReviewRepositoryMismatchIsNoApproval(t *testing.T) {
	draft := fakeCloseRunFacts()
	draft.Repository = "acme/other"
	resolver, request, _ := fakeCloseRunFixture(t, closeRunCI(), draft)
	result, err := resolver.Resolve(context.Background(), request)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result.Verdict == countersign.VerdictProven || result.Record == nil || len(result.Record.Approvals) != 0 {
		t.Fatalf("result = %+v, want no approval", result)
	}
	if !containsLifecycleWitness(result.Record.Witnesses, "environment-review:repository-mismatch:") {
		t.Fatalf("record witnesses = %v, want the repository-mismatch disclosure", result.Record.Witnesses)
	}
}

// TestResolveEnvironmentReviewUnsupportedForgeIsDisclosed: a forge without
// environment reviews (GitLab) yields its disclosure and no approval.
func TestResolveEnvironmentReviewUnsupportedForgeIsDisclosed(t *testing.T) {
	resolver, request, _ := fakeCloseRunFixture(t, closeRunCI(), fakeCloseRunFacts())
	unsupported, err := forge.NewEnvironmentReviewFacts(forge.EnvironmentReviewFacts{
		Repository: "acme/widgets", UnsupportedReason: "test: this forge has no environment reviews",
	}, lifecycleNow())
	if err != nil {
		t.Fatalf("unsupported facts: %v", err)
	}
	// Served as-is: the shared fake returns a seeded empty review list as
	// nil, which the facts contract refuses.
	resolver.Forge = fixedFactsForge{Forge: resolver.Forge, facts: unsupported}
	result, err := resolver.Resolve(context.Background(), request)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result.Verdict == countersign.VerdictProven || result.Record == nil || len(result.Record.Approvals) != 0 {
		t.Fatalf("result = %+v, want no approval", result)
	}
	for _, want := range []string{"environment-review:unsupported-forge:", "environment-review:observation:"} {
		if !containsLifecycleWitness(result.Record.Witnesses, want) {
			t.Fatalf("record witnesses = %v, want %s", result.Record.Witnesses, want)
		}
	}
}

// TestResolveEnvironmentReviewLeavesTeamRecordsUnchanged: under a team
// profile the record is byte-identical whether or not the process runs in
// a close run whose review the owner approved, because the review is never
// requested.
func TestResolveEnvironmentReviewLeavesTeamRecordsUnchanged(t *testing.T) {
	digest := func(ci forge.CIInfo, seed bool) string {
		t.Helper()
		resolver, request := lifecycleFixture(t, "story", "101", "900")
		f := resolver.Forge.(*forgefake.Forge)
		f.SetCIContext(ci)
		if seed {
			facts, err := forge.NewEnvironmentReviewFacts(fakeCloseRunFacts(), lifecycleNow())
			if err != nil {
				t.Fatalf("facts: %v", err)
			}
			if err := f.SeedEnvironmentReviewFacts(closeRunQuery(), facts); err != nil {
				t.Fatalf("seed: %v", err)
			}
		}
		result, err := resolver.Resolve(context.Background(), request)
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if result.Record == nil || result.Verdict != countersign.VerdictProven {
			t.Fatalf("result = %+v, want the independent review to prove", result)
		}
		return result.Record.Digest
	}
	outside := digest(forge.CIInfo{}, false)
	inCloseRun := digest(closeRunCI(), true)
	if outside != inCloseRun {
		t.Fatalf("team record digest changed inside an approved close run: %s != %s", inCloseRun, outside)
	}
}

// --- pure helpers ----------------------------------------------------------

func TestCurrentRunQuery(t *testing.T) {
	t.Run("a canonical run id and attempt name the close environment and gated job", func(t *testing.T) {
		query, unresolved := currentRunQuery(forge.CIInfo{Pipeline: "5551", Job: "2", JobName: "not-the-gated-job"})
		want := forge.EnvironmentReviewQuery{RunID: "5551", RunAttempt: 2, EnvironmentName: CloseEnvironmentName, GatedJobName: CloseGatedJobName}
		if unresolved != "" || query != want {
			t.Fatalf("currentRunQuery = %+v %q, want %+v", query, unresolved, want)
		}
	})
	for _, tc := range []struct {
		name, pipeline, job, reason string
	}{
		{"no run id", "", "1", "names no current run"},
		{"non-canonical run id", "05551", "1", "run id is not a canonical"},
		{"negative run id", "-5551", "1", "run id is not a canonical"},
		{"no attempt", "5551", "", "run attempt (GITHUB_RUN_ATTEMPT) is missing"},
		{"non-canonical attempt", "5551", "+1", "run attempt (GITHUB_RUN_ATTEMPT) is missing"},
		{"attempt beyond int", "5551", "9223372036854775808", "run attempt (GITHUB_RUN_ATTEMPT) is missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			query, unresolved := currentRunQuery(forge.CIInfo{Pipeline: tc.pipeline, Job: tc.job})
			if query != (forge.EnvironmentReviewQuery{}) {
				t.Fatalf("query = %+v, want none", query)
			}
			prefix := fmt.Sprintf("environment-review:current-run-unresolved: run_id=%q run_attempt=%q: ", tc.pipeline, tc.job)
			if !strings.HasPrefix(unresolved, prefix) || !strings.Contains(unresolved, tc.reason) {
				t.Fatalf("disclosure = %q, want prefix %q naming %q", unresolved, prefix, tc.reason)
			}
		})
	}
}

func TestCanonicalPositive(t *testing.T) {
	for _, tc := range []struct {
		value   string
		bitSize int
		want    int64
		ok      bool
	}{
		{"1", 0, 1, true},
		{"5551", 64, 5551, true},
		{"9223372036854775807", 64, 9223372036854775807, true},
		{"", 64, 0, false},
		{"0", 64, 0, false},
		{"-1", 64, -1, false},
		{"01", 64, 1, false},
		{"+1", 64, 1, false},
		{" 1", 64, 0, false},
		{"1x", 64, 0, false},
		{"9223372036854775808", 64, 0, false},
	} {
		t.Run(fmt.Sprintf("%q/%d", tc.value, tc.bitSize), func(t *testing.T) {
			got, ok := canonicalPositive(tc.value, tc.bitSize)
			if ok != tc.ok || (ok && got != tc.want) {
				t.Fatalf("canonicalPositive(%q, %d) = %d %v, want %d %v", tc.value, tc.bitSize, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestAnswersQuery(t *testing.T) {
	facts := fakeCloseRunFacts()
	if !answersQuery(facts, closeRunQuery()) {
		t.Fatal("the facts of the queried run do not answer the query")
	}
	upper := closeRunQuery()
	upper.EnvironmentName = "CLOSE"
	if !answersQuery(facts, upper) {
		t.Fatal("GitHub environment names are case-insensitive, but a differently cased query was refused")
	}
	for _, tc := range []struct {
		name   string
		mutate func(*forge.EnvironmentReviewQuery)
	}{
		{"another run", func(q *forge.EnvironmentReviewQuery) { q.RunID = "5550" }},
		{"another attempt", func(q *forge.EnvironmentReviewQuery) { q.RunAttempt = 2 }},
		{"another environment", func(q *forge.EnvironmentReviewQuery) { q.EnvironmentName = "production" }},
		{"another gated job", func(q *forge.EnvironmentReviewQuery) { q.GatedJobName = "Close" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			query := closeRunQuery()
			tc.mutate(&query)
			if answersQuery(facts, query) {
				t.Fatalf("facts for %+v answered query %+v", closeRunQuery(), query)
			}
		})
	}
}

func TestEarliestStamp(t *testing.T) {
	early, late := "2026-08-26T16:59:59.5Z", "2026-08-26T17:00:00Z"
	for _, pair := range [][2]string{{early, late}, {late, early}} {
		got, err := earliestStamp(pair[0], pair[1])
		if err != nil || got.Format(time.RFC3339Nano) != early {
			t.Fatalf("earliestStamp(%s, %s) = %v %v, want %s", pair[0], pair[1], got, err, early)
		}
	}
	for _, pair := range [][2]string{{"yesterday", late}, {early, "later"}} {
		if _, err := earliestStamp(pair[0], pair[1]); err == nil {
			t.Fatalf("earliestStamp(%s, %s) accepted a malformed stamp", pair[0], pair[1])
		}
	}
}
