package forge_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/forge/github"
)

// The environment-review scenario every adapter-driven test starts from is
// GitHub's own published examples (published_fixtures_test.go) with only the
// values a scenario binds changed: the run becomes a workflow_dispatch run
// of the close workflow in progress, the job and environment are renamed
// "close", and the review history approves that environment. Every other
// published member and value is kept. The job schema requires created_at but
// the published job example omits it, so it is the one member a scenario job
// adds (the verbatim example, without it, is exercised too).
const (
	erOwner        = "octo-org"
	erRepo         = "octo-repo"
	erRunID        = "30433642" // published workflow run id
	erRunIDNum     = 30433642
	erEnvID        = "161088068" // published environment id, also the review history's
	erEnvIDNum     = 161088068
	erHeadSHA      = "acb5820ced9479c074f688cc328bf03f341a511d"                    // published run head_sha
	erJobHeadSHA   = "f83a356604ae3c5d03e1b46ef4d1ca77d64a90b0"                    // published job head_sha, never the candidate
	erRunURL       = "https://github.com/octo-org/octo-repo/actions/runs/30433642" // published run html_url
	erJobID        = "399444496"                                                   // published job id
	erReviewer     = "1"                                                           // published review history user (octocat)
	erReviewerNum  = 1
	erCreatedAt    = "2020-01-20T17:30:00Z"             // the added created_at: 12m40s before the published start
	erStartedAt    = "2020-01-20T17:42:40Z"             // published job started_at
	erWorkflowPath = ".github/workflows/close.yml@main" // the published path shape ".github/workflows/build.yml@main"
)

func erQuery() forge.EnvironmentReviewQuery {
	return forge.EnvironmentReviewQuery{RunID: erRunID, RunAttempt: 1, EnvironmentName: "close", GatedJobName: "close"}
}

// publishedRunObject is a fresh copy of GitHub's published workflow run.
func publishedRunObject(t *testing.T) map[string]any {
	t.Helper()
	return asObject(t, publishedValue(t, "github/workflow-run.json"))
}

// publishedJobObject is a fresh copy of GitHub's published workflow job.
func publishedJobObject(t *testing.T) map[string]any {
	t.Helper()
	page := asObject(t, publishedValue(t, "github/job-paginated.json"))
	return asObject(t, asArray(t, page["jobs"])[0])
}

// publishedEnvironmentObject is a fresh copy of GitHub's published environment.
func publishedEnvironmentObject(t *testing.T) map[string]any {
	t.Helper()
	return asObject(t, publishedValue(t, "github/environment.json"))
}

// publishedHistoryEntryObject is a fresh copy of GitHub's published review
// history entry.
func publishedHistoryEntryObject(t *testing.T) map[string]any {
	t.Helper()
	return asObject(t, asArray(t, publishedValue(t, "github/environment-approvals-items.json"))[0])
}

// scenarioRun is the published run bound to a workflow_dispatch run of the
// close workflow that is in progress.
func scenarioRun(t *testing.T) map[string]any {
	t.Helper()
	run := publishedRunObject(t)
	setMember(t, run, "event", "workflow_dispatch")
	setMember(t, run, "path", erWorkflowPath)
	setMember(t, run, "status", "in_progress")
	return run
}

// scenarioJob is the published job renamed name with the added created_at
// stamp; its published status (completed) and conclusion (success) stand.
func scenarioJob(t *testing.T, name string) map[string]any {
	t.Helper()
	job := publishedJobObject(t)
	setMember(t, job, "name", name)
	job["created_at"] = erCreatedAt
	return job
}

// scenarioEnvironment is the published environment renamed "close".
func scenarioEnvironment(t *testing.T) map[string]any {
	t.Helper()
	env := publishedEnvironmentObject(t)
	setMember(t, env, "name", "close")
	setMember(t, env, "url", "https://api.github.com/repos/octo-org/octo-repo/environments/close")
	setMember(t, env, "html_url", "https://github.com/octo-org/octo-repo/deployments/activity_log?environments_filter=close")
	return env
}

// historyEntry is the published review history entry bound to one state,
// one environment, and one reviewer.
func historyEntry(t *testing.T, state string, envID int64, envName string, userID int64) map[string]any {
	t.Helper()
	entry := publishedHistoryEntryObject(t)
	setMember(t, entry, "state", state)
	env := asObject(t, asArray(t, entry["environments"])[0])
	setMember(t, env, "id", envID)
	setMember(t, env, "name", envName)
	setMember(t, env, "url", "https://api.github.com/repos/octo-org/octo-repo/environments/"+envName)
	setMember(t, env, "html_url", "https://github.com/octo-org/octo-repo/deployments/activity_log?environments_filter="+envName)
	setMember(t, asObject(t, entry["user"]), "id", userID)
	return entry
}

// erScenario serves one environment-review call's four GitHub routes from
// published-shape fixtures and records every request it answers. Any other
// request fails the test, so an adapter reading an undocumented or wrong
// route (the run-level jobs list, the attempt endpoint for the run) cannot
// pass.
type erScenario struct {
	t            *testing.T
	attempt      int
	run          map[string]any
	jobPages     [][]map[string]any
	env          map[string]any
	historyPages [][]map[string]any

	// Declared departures from the published member set.
	jobAllowAdded, jobAllowRemoved, envAllowRemoved []string

	// Raw bodies served verbatim instead of the structured fixtures, for
	// adverse shapes (trailing data, null pages, wrong types).
	rawRun, rawEnv      string
	rawJobs, rawHistory []string

	mu       sync.Mutex
	requests []string
}

func newERScenario(t *testing.T) *erScenario {
	t.Helper()
	return &erScenario{
		t:             t,
		attempt:       1,
		run:           scenarioRun(t),
		jobPages:      [][]map[string]any{{scenarioJob(t, "close")}},
		env:           scenarioEnvironment(t),
		historyPages:  [][]map[string]any{{historyEntry(t, "approved", erEnvIDNum, "close", erReviewerNum)}},
		jobAllowAdded: []string{".created_at"},
	}
}

// requestLog returns the routes answered so far, in order.
func (s *erScenario) requestLog() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.requests...)
}

func (s *erScenario) record(route string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, route)
}

// adapter starts the scenario's server and returns a GitHub adapter on it.
func (s *erScenario) adapter() *github.Adapter {
	t := s.t
	t.Helper()
	base := "/repos/" + erOwner + "/" + erRepo
	runPath := base + "/actions/runs/" + erRunID
	jobsPath := fmt.Sprintf("%s/attempts/%d/jobs", runPath, s.attempt)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case runPath:
			s.record("run")
			writeJSON(t, w, s.runBody())
		case jobsPath:
			s.record("jobs")
			s.writePage(w, r, s.jobPageBodies())
		case base + "/environments/close":
			s.record("environment")
			writeJSON(t, w, s.envBody())
		case runPath + "/approvals":
			s.record("history")
			s.writePage(w, r, s.historyPageBodies())
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return github.New(github.Config{BaseURL: server.URL, Owner: erOwner, Repo: erRepo, HTTPClient: server.Client(), Clock: fixedClock})
}

// writePage serves page ?page=n (the first page has no page parameter) and
// links the next page with GitHub's Link header.
func (s *erScenario) writePage(w http.ResponseWriter, r *http.Request, bodies []string) {
	count := len(bodies)
	index := 0
	if page := r.URL.Query().Get("page"); page != "" {
		n, err := strconv.Atoi(page)
		if err != nil || n < 1 || n > count {
			s.t.Errorf("unexpected page %q of %d", page, count)
			http.NotFound(w, r)
			return
		}
		index = n - 1
	}
	if index+1 < count {
		w.Header().Set("Link", fmt.Sprintf(`<http://%s%s?page=%d>; rel="next"`, r.Host, r.URL.Path, index+2))
	}
	writeJSON(s.t, w, bodies[index])
}

func (s *erScenario) runBody() string {
	if s.rawRun != "" {
		return s.rawRun
	}
	if problem := publishedShapeViolation(publishedRunObject(s.t), s.run, nil, nil); problem != "" {
		s.t.Errorf("workflow run fixture departs from GitHub's published example: %s", problem)
	}
	return encodeGeneric(s.t, s.run)
}

func (s *erScenario) envBody() string {
	if s.rawEnv != "" {
		return s.rawEnv
	}
	if problem := publishedShapeViolation(publishedEnvironmentObject(s.t), s.env, nil, s.envAllowRemoved); problem != "" {
		s.t.Errorf("environment fixture departs from GitHub's published example: %s", problem)
	}
	return encodeGeneric(s.t, s.env)
}

func (s *erScenario) jobPageBodies() []string {
	if s.rawJobs != nil {
		return s.rawJobs
	}
	total := 0
	for _, page := range s.jobPages {
		total += len(page)
	}
	published := publishedJobObject(s.t)
	bodies := make([]string, 0, len(s.jobPages))
	for _, page := range s.jobPages {
		jobs := make([]any, 0, len(page))
		for _, job := range page {
			if problem := publishedShapeViolation(published, job, s.jobAllowAdded, s.jobAllowRemoved); problem != "" {
				s.t.Errorf("workflow job fixture departs from GitHub's published example: %s", problem)
			}
			jobs = append(jobs, job)
		}
		bodies = append(bodies, encodeGeneric(s.t, map[string]any{"total_count": total, "jobs": jobs}))
	}
	return bodies
}

func (s *erScenario) historyPageBodies() []string {
	if s.rawHistory != nil {
		return s.rawHistory
	}
	published := publishedHistoryEntryObject(s.t)
	bodies := make([]string, 0, len(s.historyPages))
	for _, page := range s.historyPages {
		entries := make([]any, 0, len(page))
		for _, entry := range page {
			if problem := historyFixtureViolation(published, entry); problem != "" {
				s.t.Errorf("review history fixture: %s", problem)
			}
			entries = append(entries, entry)
		}
		bodies = append(bodies, encodeGeneric(s.t, entries))
	}
	return bodies
}

// historyFixtureViolation is the review-history fixture guard: an entry must
// carry exactly the members GitHub's published entry carries, and in
// particular no attempt member, because GitHub's review history supplies
// none (v2 ac-4 static falsifier: "relies on test fixtures that add an
// attempt field GitHub's review history does not supply").
func historyFixtureViolation(published, entry map[string]any) string {
	paths := map[string]struct{}{}
	jsonKeyPaths(entry, "", paths)
	for path := range paths {
		if strings.Contains(strings.ToLower(path), "attempt") {
			return "adds " + path + ", an attempt member GitHub's review history does not supply"
		}
	}
	if problem := publishedShapeViolation(published, entry, nil, nil); problem != "" {
		return "departs from GitHub's published entry: " + problem
	}
	return ""
}
