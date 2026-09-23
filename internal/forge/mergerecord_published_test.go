package forge_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/forge/github"
	"github.com/jyang234/verdi/internal/forge/gitlab"
)

// The merge-record adapters driven over the providers' own published
// examples (ruling R-W1-8 as plan R-PB-2 applies it): each published example
// decodes verbatim with every unmodeled member ignored, and each mapping rule
// is exercised by a response built from a published example with only the
// values the scenario binds changed (setMember) — never an invented member
// unless it is declared below.

// mrPublishedTestMerge is the published pull request's merge_commit_sha. The
// published pull request is open, so this is GitHub's test merge: asking
// about exactly this commit proves the test merge never becomes a merge
// commit.
const mrPublishedTestMerge = "e5bd3914e2e596debea16f433f57875b5b90bcd6"

// publishedPullSimpleObject is a fresh copy of the one pull request in
// GitHub's published "List pull requests associated with a commit" example.
func publishedPullSimpleObject(t *testing.T) map[string]any {
	t.Helper()
	return asObject(t, asArray(t, publishedValue(t, "github/pull-request-simple-items.json"))[0])
}

// publishedRepositoryObject is a fresh copy of GitHub's published "Get a
// repository" example.
func publishedRepositoryObject(t *testing.T) map[string]any {
	t.Helper()
	return asObject(t, publishedValue(t, "github/full-repository-default-response.json"))
}

// mrGitHubPull is the published pull request bound to one scenario.
func mrGitHubPull(t *testing.T, state string, mergedAt, mergeSHA any, baseRef string) map[string]any {
	t.Helper()
	pull := publishedPullSimpleObject(t)
	setMember(t, pull, "state", state)
	setMember(t, pull, "merged_at", mergedAt)
	setMember(t, pull, "merge_commit_sha", mergeSHA)
	setMember(t, asObject(t, pull["base"]), "ref", baseRef)
	return pull
}

// mrGitHubScenario serves "Get a repository" and one page of "List pull
// requests associated with a commit" for owner/repo octocat/Hello-World (the
// published repository's full_name) and records each route it answers.
type mrGitHubScenario struct {
	t              *testing.T
	commit         string
	rawRepo        string
	rawPulls       string
	repo           map[string]any
	pulls          []map[string]any
	mu             sync.Mutex
	requestedPaths []string
}

func (s *mrGitHubScenario) adapter() *github.Adapter {
	t := s.t
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requestedPaths = append(s.requestedPaths, r.URL.Path)
		s.mu.Unlock()
		switch r.URL.Path {
		case "/repos/octocat/Hello-World":
			writeJSON(t, w, s.repoBody())
		case "/repos/octocat/Hello-World/commits/" + s.commit + "/pulls":
			writeJSON(t, w, s.pullsBody())
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return github.New(github.Config{BaseURL: server.URL, Owner: "octocat", Repo: "Hello-World", HTTPClient: server.Client(), Clock: fixedClock})
}

func (s *mrGitHubScenario) repoBody() string {
	if s.rawRepo != "" {
		return s.rawRepo
	}
	if problem := publishedShapeViolation(publishedRepositoryObject(s.t), s.repo, nil, nil); problem != "" {
		s.t.Errorf("repository fixture departs from GitHub's published example: %s", problem)
	}
	return encodeGeneric(s.t, s.repo)
}

func (s *mrGitHubScenario) pullsBody() string {
	if s.rawPulls != "" {
		return s.rawPulls
	}
	published := publishedPullSimpleObject(s.t)
	page := make([]any, 0, len(s.pulls))
	for _, pull := range s.pulls {
		if problem := publishedShapeViolation(published, pull, nil, nil); problem != "" {
			s.t.Errorf("pull request fixture departs from GitHub's published example: %s", problem)
		}
		page = append(page, pull)
	}
	return encodeGeneric(s.t, page)
}

func TestMergeRecordsPublishedGitHub(t *testing.T) {
	t.Run("the published examples decode verbatim and the test merge is never a merge commit", func(t *testing.T) {
		s := &mrGitHubScenario{
			t: t, commit: mrPublishedTestMerge,
			rawRepo:  string(publishedFixture(t, "github/full-repository-default-response.json")),
			rawPulls: string(publishedFixture(t, "github/pull-request-simple-items.json")),
		}
		facts, err := s.adapter().MergeRecords(context.Background(), mrPublishedTestMerge)
		if err != nil {
			t.Fatalf("MergeRecords over the published examples: %v", err)
		}
		if got, want := s.requestedPaths, []string{"/repos/octocat/Hello-World", "/repos/octocat/Hello-World/commits/" + mrPublishedTestMerge + "/pulls"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("requests = %v, want %v", got, want)
		}
		if facts.DefaultBranch != "master" || facts.Repository != "octocat/Hello-World" || facts.Commit != mrPublishedTestMerge {
			t.Fatalf("facts = %+v, want the published default branch master for octocat/Hello-World", facts)
		}
		want := []forge.ChangeRequestMerge{{ChangeID: "1347", State: forge.ChangeRequestOpen, TargetBranch: "master"}}
		if !reflect.DeepEqual(facts.Changes, want) {
			t.Fatalf("changes = %+v, want the open published pull request with no merge commit or merge time %+v", facts.Changes, want)
		}
		proof, err := forge.ProveMergedIntoDefault(facts)
		if err != nil {
			t.Fatalf("ProveMergedIntoDefault: %v", err)
		}
		if proof.State != forge.MergeProofUnproven || proof.Reason != forge.MergeProofReasonNoMergeIntoDefault {
			t.Fatalf("proof = %+v, want unproven: an open pull request's test merge is never a merge", proof)
		}
	})

	tests := []struct {
		name       string
		pull       func(*testing.T) map[string]any
		want       forge.ChangeRequestMerge
		wantProven bool
		wantDetail string
	}{
		{
			name: "closed with merged_at is merged, and its merge commit is honored",
			pull: func(t *testing.T) map[string]any {
				return mrGitHubPull(t, "closed", "2011-01-26T19:01:12Z", mrCommit, "master")
			},
			want:       forge.ChangeRequestMerge{ChangeID: "1347", State: forge.ChangeRequestMerged, TargetBranch: "master", MergeCommitSHA: mrCommit, MergedAt: "2011-01-26T19:01:12Z"},
			wantProven: true,
		},
		{
			name: "merged_at is normalized to UTC",
			pull: func(t *testing.T) map[string]any {
				return mrGitHubPull(t, "closed", "2011-01-26T20:01:12.5+01:00", mrCommit, "master")
			},
			want:       forge.ChangeRequestMerge{ChangeID: "1347", State: forge.ChangeRequestMerged, TargetBranch: "master", MergeCommitSHA: mrCommit, MergedAt: "2011-01-26T19:01:12.5Z"},
			wantProven: true,
		},
		{
			name: "closed with a null merged_at is closed, and its stale merge_commit_sha is dropped",
			pull: func(t *testing.T) map[string]any {
				return mrGitHubPull(t, "closed", nil, mrCommit, "master")
			},
			want:       forge.ChangeRequestMerge{ChangeID: "1347", State: forge.ChangeRequestClosed, TargetBranch: "master"},
			wantDetail: "closed without merging",
		},
		{
			name: "open with the commit as its test merge is open, and the test merge is dropped",
			pull: func(t *testing.T) map[string]any {
				return mrGitHubPull(t, "open", nil, mrCommit, "master")
			},
			want:       forge.ChangeRequestMerge{ChangeID: "1347", State: forge.ChangeRequestOpen, TargetBranch: "master"},
			wantDetail: "test merge",
		},
		{
			name: "merged into another base branch",
			pull: func(t *testing.T) map[string]any {
				return mrGitHubPull(t, "closed", "2011-01-26T19:01:12Z", mrCommit, "release")
			},
			want:       forge.ChangeRequestMerge{ChangeID: "1347", State: forge.ChangeRequestMerged, TargetBranch: "release", MergeCommitSHA: mrCommit, MergedAt: "2011-01-26T19:01:12Z"},
			wantDetail: "merged into release, not the default branch master",
		},
		{
			name: "merged with a null merge_commit_sha",
			pull: func(t *testing.T) map[string]any {
				return mrGitHubPull(t, "closed", "2011-01-26T19:01:12Z", nil, "master")
			},
			want:       forge.ChangeRequestMerge{ChangeID: "1347", State: forge.ChangeRequestMerged, TargetBranch: "master", MergedAt: "2011-01-26T19:01:12Z"},
			wantDetail: "no forge-reported merge commit",
		},
		{
			name: "merged into the default branch with a different merge commit",
			pull: func(t *testing.T) map[string]any {
				return mrGitHubPull(t, "closed", "2011-01-26T19:01:12Z", mrOtherCommit, "master")
			},
			want:       forge.ChangeRequestMerge{ChangeID: "1347", State: forge.ChangeRequestMerged, TargetBranch: "master", MergeCommitSHA: mrOtherCommit, MergedAt: "2011-01-26T19:01:12Z"},
			wantDetail: "merge commit " + mrOtherCommit,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &mrGitHubScenario{t: t, commit: mrCommit, repo: publishedRepositoryObject(t), pulls: []map[string]any{tt.pull(t)}}
			facts, err := s.adapter().MergeRecords(context.Background(), mrCommit)
			if err != nil {
				t.Fatalf("MergeRecords: %v", err)
			}
			if !reflect.DeepEqual(facts.Changes, []forge.ChangeRequestMerge{tt.want}) {
				t.Fatalf("changes = %+v, want %+v", facts.Changes, tt.want)
			}
			proof, err := forge.ProveMergedIntoDefault(facts)
			if err != nil {
				t.Fatalf("ProveMergedIntoDefault: %v", err)
			}
			if got := proof.State == forge.MergeProofProven; got != tt.wantProven {
				t.Fatalf("proof = %+v, want proven=%v", proof, tt.wantProven)
			}
			if !strings.Contains(proof.Detail, tt.wantDetail) {
				t.Fatalf("proof detail = %q, want it to name %q", proof.Detail, tt.wantDetail)
			}
		})
	}

	t.Run("a published pull request in an unknown state fails closed", func(t *testing.T) {
		s := &mrGitHubScenario{t: t, commit: mrCommit, repo: publishedRepositoryObject(t), pulls: []map[string]any{
			mrGitHubPull(t, "merged", "2011-01-26T19:01:12Z", mrCommit, "master"),
		}}
		if facts, err := s.adapter().MergeRecords(context.Background(), mrCommit); err == nil || errors.Is(err, forge.ErrUnavailable) {
			t.Fatalf("MergeRecords = %+v, %v; want an operational error that is not unavailability", facts, err)
		}
	})

	t.Run("the default branch comes from the repository read", func(t *testing.T) {
		repo := publishedRepositoryObject(t)
		setMember(t, repo, "default_branch", "trunk")
		s := &mrGitHubScenario{t: t, commit: mrCommit, repo: repo, pulls: []map[string]any{
			mrGitHubPull(t, "closed", "2011-01-26T19:01:12Z", mrCommit, "master"),
		}}
		facts, err := s.adapter().MergeRecords(context.Background(), mrCommit)
		if err != nil {
			t.Fatalf("MergeRecords: %v", err)
		}
		proof, err := forge.ProveMergedIntoDefault(facts)
		if err != nil {
			t.Fatalf("ProveMergedIntoDefault: %v", err)
		}
		if facts.DefaultBranch != "trunk" || proof.State != forge.MergeProofUnproven {
			t.Fatalf("facts %+v proof %+v: a merge into master is not a merge into the default branch trunk", facts, proof)
		}
	})
}

// mrPublishedMRHead is the published GitLab merge request's head sha, also
// the sha of the published request URL.
const mrPublishedMRHead = "af5b13261899fb2c0db30abdd0af8b07cb44fdc5"

// mrGitLabMergedAt is the one member a merged GitLab scenario adds: the
// commits.md example (an opened merge request) omits merged_at, which the
// merge request attributes in merge_requests.md carry and a merged merge
// request reports. Every other published member and value is kept.
const mrGitLabMergedAt = "2018-03-26T17:40:00.000Z"

// publishedCommitMergeRequestObject is a fresh copy of the one merge request
// in GitLab's published "List merge requests associated with a commit"
// example.
func publishedCommitMergeRequestObject(t *testing.T) map[string]any {
	t.Helper()
	return asObject(t, asArray(t, publishedValue(t, "gitlab/commit-merge-requests.json"))[0])
}

// publishedProjectObject is a fresh copy of GitLab's published "Retrieve a
// project" example.
func publishedProjectObject(t *testing.T) map[string]any {
	t.Helper()
	return asObject(t, publishedValue(t, "gitlab/project.json"))
}

// mrGitLabMR is the published merge request bound to one scenario; a merged
// scenario adds merged_at (mrGitLabMergedAt's doc).
func mrGitLabMR(t *testing.T, state string, mergeSHA, squashSHA any, target string) map[string]any {
	t.Helper()
	mr := publishedCommitMergeRequestObject(t)
	setMember(t, mr, "state", state)
	setMember(t, mr, "merge_commit_sha", mergeSHA)
	setMember(t, mr, "squash_commit_sha", squashSHA)
	setMember(t, mr, "target_branch", target)
	if state == "merged" {
		mr["merged_at"] = mrGitLabMergedAt
	}
	return mr
}

// mrGitLabScenario serves "Retrieve a project" and one page of "List merge
// requests associated with a commit" for project 3 (the published project's
// id) and records each route it answers.
type mrGitLabScenario struct {
	t              *testing.T
	commit         string
	rawProject     string
	rawMRs         string
	project        map[string]any
	mrs            []map[string]any
	mu             sync.Mutex
	requestedPaths []string
}

func (s *mrGitLabScenario) adapter() *gitlab.Adapter {
	t := s.t
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requestedPaths = append(s.requestedPaths, r.URL.Path)
		s.mu.Unlock()
		switch r.URL.Path {
		case "/projects/3":
			writeJSON(t, w, s.projectBody())
		case "/projects/3/repository/commits/" + s.commit + "/merge_requests":
			writeJSON(t, w, s.mrsBody())
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return gitlab.New(gitlab.Config{BaseURL: server.URL, ProjectID: "3", HTTPClient: server.Client(), Clock: fixedClock})
}

func (s *mrGitLabScenario) projectBody() string {
	if s.rawProject != "" {
		return s.rawProject
	}
	if problem := publishedShapeViolation(publishedProjectObject(s.t), s.project, nil, nil); problem != "" {
		s.t.Errorf("project fixture departs from GitLab's published example: %s", problem)
	}
	return encodeGeneric(s.t, s.project)
}

func (s *mrGitLabScenario) mrsBody() string {
	if s.rawMRs != "" {
		return s.rawMRs
	}
	published := publishedCommitMergeRequestObject(s.t)
	page := make([]any, 0, len(s.mrs))
	for _, mr := range s.mrs {
		if problem := publishedShapeViolation(published, mr, []string{".merged_at"}, nil); problem != "" {
			s.t.Errorf("merge request fixture departs from GitLab's published example: %s", problem)
		}
		page = append(page, mr)
	}
	return encodeGeneric(s.t, page)
}

func TestMergeRecordsPublishedGitLab(t *testing.T) {
	t.Run("the published examples decode verbatim", func(t *testing.T) {
		s := &mrGitLabScenario{
			t: t, commit: mrPublishedMRHead,
			rawProject: string(publishedFixture(t, "gitlab/project.json")),
			rawMRs:     string(publishedFixture(t, "gitlab/commit-merge-requests.json")),
		}
		facts, err := s.adapter().MergeRecords(context.Background(), mrPublishedMRHead)
		if err != nil {
			t.Fatalf("MergeRecords over the published examples: %v", err)
		}
		if got, want := s.requestedPaths, []string{"/projects/3", "/projects/3/repository/commits/" + mrPublishedMRHead + "/merge_requests"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("requests = %v, want %v", got, want)
		}
		if facts.DefaultBranch != "main" || facts.Repository != "3" || facts.Commit != mrPublishedMRHead {
			t.Fatalf("facts = %+v, want the published default branch main for project 3", facts)
		}
		want := []forge.ChangeRequestMerge{{ChangeID: "1", State: forge.ChangeRequestOpen, TargetBranch: "main"}}
		if !reflect.DeepEqual(facts.Changes, want) {
			t.Fatalf("changes = %+v, want the opened published merge request %+v", facts.Changes, want)
		}
		proof, err := forge.ProveMergedIntoDefault(facts)
		if err != nil {
			t.Fatalf("ProveMergedIntoDefault: %v", err)
		}
		if proof.State != forge.MergeProofUnproven || proof.Reason != forge.MergeProofReasonNoMergeIntoDefault {
			t.Fatalf("proof = %+v, want unproven for an opened merge request", proof)
		}
	})

	tests := []struct {
		name       string
		mr         func(*testing.T) map[string]any
		want       forge.ChangeRequestMerge
		wantProven bool
		wantDetail string
	}{
		{
			name:       "merged with a merge commit is merged, and its merge commit is honored",
			mr:         func(t *testing.T) map[string]any { return mrGitLabMR(t, "merged", mrCommit, nil, "main") },
			want:       forge.ChangeRequestMerge{ChangeID: "1", State: forge.ChangeRequestMerged, TargetBranch: "main", MergeCommitSHA: mrCommit, MergedAt: "2018-03-26T17:40:00Z"},
			wantProven: true,
		},
		{
			name:       "a fast-forward merge reports no merge commit, and its squash commit is never one",
			mr:         func(t *testing.T) map[string]any { return mrGitLabMR(t, "merged", nil, mrCommit, "main") },
			want:       forge.ChangeRequestMerge{ChangeID: "1", State: forge.ChangeRequestMerged, TargetBranch: "main", MergedAt: "2018-03-26T17:40:00Z"},
			wantDetail: "fast-forward",
		},
		{
			name:       "locked is a merge in progress, which is not a merge",
			mr:         func(t *testing.T) map[string]any { return mrGitLabMR(t, "locked", mrCommit, nil, "main") },
			want:       forge.ChangeRequestMerge{ChangeID: "1", State: forge.ChangeRequestOpen, TargetBranch: "main"},
			wantDetail: "state=open",
		},
		{
			name:       "opened is open, and a merge_commit_sha on it is dropped",
			mr:         func(t *testing.T) map[string]any { return mrGitLabMR(t, "opened", mrCommit, nil, "main") },
			want:       forge.ChangeRequestMerge{ChangeID: "1", State: forge.ChangeRequestOpen, TargetBranch: "main"},
			wantDetail: "state=open",
		},
		{
			name:       "closed is closed without merging",
			mr:         func(t *testing.T) map[string]any { return mrGitLabMR(t, "closed", mrCommit, nil, "main") },
			want:       forge.ChangeRequestMerge{ChangeID: "1", State: forge.ChangeRequestClosed, TargetBranch: "main"},
			wantDetail: "closed without merging",
		},
		{
			name:       "merged into another target branch",
			mr:         func(t *testing.T) map[string]any { return mrGitLabMR(t, "merged", mrCommit, nil, "release") },
			want:       forge.ChangeRequestMerge{ChangeID: "1", State: forge.ChangeRequestMerged, TargetBranch: "release", MergeCommitSHA: mrCommit, MergedAt: "2018-03-26T17:40:00Z"},
			wantDetail: "merged into release, not the default branch main",
		},
		{
			name:       "merged into the default branch with a different merge commit",
			mr:         func(t *testing.T) map[string]any { return mrGitLabMR(t, "merged", mrOtherCommit, nil, "main") },
			want:       forge.ChangeRequestMerge{ChangeID: "1", State: forge.ChangeRequestMerged, TargetBranch: "main", MergeCommitSHA: mrOtherCommit, MergedAt: "2018-03-26T17:40:00Z"},
			wantDetail: "merge commit " + mrOtherCommit,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &mrGitLabScenario{t: t, commit: mrCommit, project: publishedProjectObject(t), mrs: []map[string]any{tt.mr(t)}}
			facts, err := s.adapter().MergeRecords(context.Background(), mrCommit)
			if err != nil {
				t.Fatalf("MergeRecords: %v", err)
			}
			if !reflect.DeepEqual(facts.Changes, []forge.ChangeRequestMerge{tt.want}) {
				t.Fatalf("changes = %+v, want %+v", facts.Changes, tt.want)
			}
			proof, err := forge.ProveMergedIntoDefault(facts)
			if err != nil {
				t.Fatalf("ProveMergedIntoDefault: %v", err)
			}
			if got := proof.State == forge.MergeProofProven; got != tt.wantProven {
				t.Fatalf("proof = %+v, want proven=%v", proof, tt.wantProven)
			}
			if !strings.Contains(proof.Detail, tt.wantDetail) {
				t.Fatalf("proof detail = %q, want it to name %q", proof.Detail, tt.wantDetail)
			}
		})
	}

	for _, tt := range []struct {
		name string
		mr   func(*testing.T) map[string]any
	}{
		{"a published merge request in an unknown state fails closed", func(t *testing.T) map[string]any {
			return mrGitLabMR(t, "reopened", nil, nil, "main")
		}},
		{"a merged merge request without merged_at fails closed", func(t *testing.T) map[string]any {
			mr := mrGitLabMR(t, "merged", mrCommit, nil, "main")
			delete(mr, "merged_at")
			return mr
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := &mrGitLabScenario{t: t, commit: mrCommit, project: publishedProjectObject(t), mrs: []map[string]any{tt.mr(t)}}
			if facts, err := s.adapter().MergeRecords(context.Background(), mrCommit); err == nil || errors.Is(err, forge.ErrUnavailable) {
				t.Fatalf("MergeRecords = %+v, %v; want an operational error that is not unavailability", facts, err)
			}
		})
	}
}
