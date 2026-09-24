package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/forge"
)

// Transport-level tests for the GitHub merge-record read (SI-249; plan
// R-PB-2). The mapping from GitHub's published pull-request-simple example is
// covered in internal/forge/mergerecord_published_test.go; these drive the
// adapter's routes, pagination, and error classification over minimal
// bodies.

const (
	mrtCommit = "6dcb09b5b57875f334f61aebed695e2e4193db5e"
	mrtOther  = "e5bd3914e2e596debea16f433f57875b5b90bcd6"
	// mrtRepoID is the queried repository's id in "Get a repository"; every
	// pull request's base repository carries it unless a row says otherwise.
	mrtRepoID = 1296269
)

func mrtClock() time.Time { return time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC) }

// mrtPull is a minimal pull-request-simple body carrying the members the
// adapter reads plus some it does not (title, base.sha, base.repo.full_name),
// which must be ignored. Its base repository is the queried repository.
func mrtPull(number int64, state string, mergedAt, mergeSHA any, baseRef string) map[string]any {
	return map[string]any{
		"number": number, "state": state, "merged_at": mergedAt, "merge_commit_sha": mergeSHA,
		"title": "unmodeled", "base": map[string]any{
			"ref": baseRef, "sha": mrtOther,
			"repo": map[string]any{"id": mrtRepoID, "full_name": "acme/svcfix"},
		},
	}
}

func mrtEncode(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return string(data)
}

// mrtServer answers the two merge-record routes and records every request.
type mrtServer struct {
	t          *testing.T
	repoStatus int
	repoBody   string
	pullStatus int
	pullPages  []string // raw bodies; page n+1 is linked from page n
	// links overrides the Link header of the 1-based page it names; "{self}"
	// is replaced by the list route's absolute URL without a query.
	links map[int]string

	mu       sync.Mutex
	requests []string
	listHits int
}

// mrtMaxListHits bounds the list walk: a regression that loses the cycle
// rule then fails fast with a named error instead of hanging the suite.
const mrtMaxListHits = 20

func newMRTServer(t *testing.T, pulls ...map[string]any) *mrtServer {
	t.Helper()
	page := make([]any, 0, len(pulls))
	for _, pull := range pulls {
		page = append(page, pull)
	}
	return &mrtServer{
		t: t, repoStatus: http.StatusOK, repoBody: `{"id":1296269,"default_branch":"main","full_name":"acme/svcfix"}`,
		pullStatus: http.StatusOK, pullPages: []string{mrtEncode(t, page)},
	}
}

func (s *mrtServer) log() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.requests...)
}

func (s *mrtServer) adapter() *Adapter {
	t := s.t
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requests = append(s.requests, r.URL.Path+"?"+r.URL.RawQuery)
		s.mu.Unlock()
		switch r.URL.Path {
		case "/repos/acme/svcfix":
			w.WriteHeader(s.repoStatus)
			_, _ = w.Write([]byte(s.repoBody))
		case "/repos/acme/svcfix/commits/" + mrtCommit + "/pulls":
			if s.pullStatus != http.StatusOK {
				w.WriteHeader(s.pullStatus)
				return
			}
			s.mu.Lock()
			s.listHits++
			hits := s.listHits
			s.mu.Unlock()
			if hits > mrtMaxListHits {
				t.Errorf("the pull request list was walked more than %d times", mrtMaxListHits)
				_, _ = w.Write([]byte(`[]`))
				return
			}
			index := 0
			if page := r.URL.Query().Get("page"); page != "" {
				if _, err := fmt.Sscanf(page, "%d", &index); err != nil || index < 2 || index > len(s.pullPages) {
					t.Errorf("unexpected page %q", page)
					http.NotFound(w, r)
					return
				}
				index--
			}
			if link, ok := s.links[index+1]; ok {
				w.Header().Set("Link", strings.ReplaceAll(link, "{self}", server.URL+r.URL.Path))
			} else if index+1 < len(s.pullPages) {
				w.Header().Set("Link", fmt.Sprintf(`<%s%s?per_page=100&page=%d>; rel="next"`, server.URL, r.URL.Path, index+2))
			}
			_, _ = w.Write([]byte(s.pullPages[index]))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return New(Config{BaseURL: server.URL, Owner: "acme", Repo: "svcfix", HTTPClient: server.Client(), Clock: mrtClock})
}

func TestGitHubMergeRecords_ReadsRepositoryThenPulls(t *testing.T) {
	s := newMRTServer(t, mrtPull(12, "closed", "2026-09-20T10:15:30Z", mrtCommit, "main"))
	facts, err := s.adapter().MergeRecords(context.Background(), mrtCommit)
	if err != nil {
		t.Fatalf("MergeRecords: %v", err)
	}
	want := []string{
		"/repos/acme/svcfix?",
		"/repos/acme/svcfix/commits/" + mrtCommit + "/pulls?per_page=100",
	}
	if got := s.log(); !reflect.DeepEqual(got, want) {
		t.Fatalf("requests = %v, want %v", got, want)
	}
	if !facts.Supported || facts.Repository != "acme/svcfix" || facts.Commit != mrtCommit || facts.DefaultBranch != "main" {
		t.Fatalf("facts = %+v", facts)
	}
	wantChanges := []forge.ChangeRequestMerge{{
		ChangeID: "12", State: forge.ChangeRequestMerged, TargetBranch: "main",
		MergeCommitSHA: mrtCommit, MergedAt: "2026-09-20T10:15:30Z",
	}}
	if !reflect.DeepEqual(facts.Changes, wantChanges) {
		t.Fatalf("changes = %+v, want %+v", facts.Changes, wantChanges)
	}
	if facts.ObservedAt != "2026-09-23T12:00:00Z" {
		t.Fatalf("ObservedAt = %q, want the adapter clock", facts.ObservedAt)
	}
	if err := facts.Validate(); err != nil {
		t.Fatalf("facts do not validate: %v", err)
	}
}

func TestGitHubMergeRecords_DrainsEveryPage(t *testing.T) {
	s := newMRTServer(t)
	s.pullPages = []string{
		mrtEncode(t, []any{mrtPull(30, "open", nil, mrtCommit, "main")}),
		mrtEncode(t, []any{mrtPull(4, "closed", nil, nil, "main")}),
		mrtEncode(t, []any{mrtPull(200, "closed", "2026-09-20T10:15:30Z", mrtCommit, "main")}),
	}
	facts, err := s.adapter().MergeRecords(context.Background(), mrtCommit)
	if err != nil {
		t.Fatalf("MergeRecords: %v", err)
	}
	var ids []string
	for _, change := range facts.Changes {
		ids = append(ids, change.ChangeID)
	}
	if !reflect.DeepEqual(ids, []string{"4", "30", "200"}) {
		t.Fatalf("change ids = %v, want every page's change in numeric order", ids)
	}
	if got := len(s.log()); got != 4 {
		t.Fatalf("requests = %v, want the repository plus three pages", s.log())
	}
}

func TestGitHubMergeRecords_RefusesNonSHAInputBeforeAnyRequest(t *testing.T) {
	for _, commit := range []string{"", "main", "HEAD~1", mrtCommit[:12], strings.ToUpper(mrtCommit), mrtCommit + "a", "../../pulls"} {
		t.Run(commit, func(t *testing.T) {
			s := newMRTServer(t)
			if facts, err := s.adapter().MergeRecords(context.Background(), commit); err == nil {
				t.Fatalf("MergeRecords(%q) = %+v, want error", commit, facts)
			} else if errors.Is(err, forge.ErrUnavailable) {
				t.Fatalf("MergeRecords(%q) error %v wraps ErrUnavailable; a bad input is operational", commit, err)
			}
			if got := s.log(); len(got) != 0 {
				t.Fatalf("MergeRecords(%q) made requests %v, want none", commit, got)
			}
		})
	}
}

func TestGitHubMergeRecords_FailsClosedOnProviderShape(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*mrtServer)
	}{
		{"unknown pull request state", func(s *mrtServer) {
			s.pullPages = []string{mrtEncode(t, []any{mrtPull(1, "merged", "2026-09-20T10:15:30Z", mrtCommit, "main")})}
		}},
		{"empty pull request state", func(s *mrtServer) {
			s.pullPages = []string{mrtEncode(t, []any{mrtPull(1, "", nil, nil, "main")})}
		}},
		{"missing pull request number", func(s *mrtServer) {
			pull := mrtPull(1, "open", nil, nil, "main")
			delete(pull, "number")
			s.pullPages = []string{mrtEncode(t, []any{pull})}
		}},
		{"zero pull request number", func(s *mrtServer) {
			s.pullPages = []string{mrtEncode(t, []any{mrtPull(0, "open", nil, nil, "main")})}
		}},
		{"negative pull request number", func(s *mrtServer) {
			s.pullPages = []string{mrtEncode(t, []any{mrtPull(-3, "open", nil, nil, "main")})}
		}},
		{"pull request number of the wrong type", func(s *mrtServer) {
			pull := mrtPull(1, "open", nil, nil, "main")
			pull["number"] = "1"
			s.pullPages = []string{mrtEncode(t, []any{pull})}
		}},
		{"missing base", func(s *mrtServer) {
			pull := mrtPull(1, "open", nil, nil, "main")
			delete(pull, "base")
			s.pullPages = []string{mrtEncode(t, []any{pull})}
		}},
		{"null base", func(s *mrtServer) {
			pull := mrtPull(1, "open", nil, nil, "main")
			pull["base"] = nil
			s.pullPages = []string{mrtEncode(t, []any{pull})}
		}},
		{"duplicate pull request across pages", func(s *mrtServer) {
			s.pullPages = []string{
				mrtEncode(t, []any{mrtPull(1, "open", nil, nil, "main")}),
				mrtEncode(t, []any{mrtPull(1, "open", nil, nil, "main")}),
			}
		}},
		{"merged pull request with an unparseable merge time", func(s *mrtServer) {
			s.pullPages = []string{mrtEncode(t, []any{mrtPull(1, "closed", "last tuesday", mrtCommit, "main")})}
		}},
		{"merged pull request with an abbreviated merge commit", func(s *mrtServer) {
			s.pullPages = []string{mrtEncode(t, []any{mrtPull(1, "closed", "2026-09-20T10:15:30Z", mrtCommit[:7], "main")})}
		}},
		{"a null page", func(s *mrtServer) { s.pullPages = []string{"null"} }},
		{"an object page", func(s *mrtServer) { s.pullPages = []string{`{"pulls":[]}`} }},
		{"trailing data after a page", func(s *mrtServer) { s.pullPages = []string{`[]` + "\n" + `[]`} }},
		{"trailing data after the repository", func(s *mrtServer) {
			s.repoBody = `{"id":1296269,"default_branch":"main"} {"default_branch":"evil"}`
		}},
		{"repository without a default branch", func(s *mrtServer) { s.repoBody = `{"id":1296269,"full_name":"acme/svcfix"}` }},
		{"repository with a null default branch", func(s *mrtServer) { s.repoBody = `{"id":1296269,"default_branch":null}` }},
		{"repository default branch of the wrong type", func(s *mrtServer) { s.repoBody = `{"id":1296269,"default_branch":7}` }},
		{"a 404 on the repository read", func(s *mrtServer) {
			s.repoStatus = http.StatusNotFound
			s.repoBody = `{"message":"Not Found"}`
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newMRTServer(t)
			tt.setup(s)
			facts, err := s.adapter().MergeRecords(context.Background(), mrtCommit)
			if err == nil {
				t.Fatalf("MergeRecords = %+v, want an operational error", facts)
			}
			if errors.Is(err, forge.ErrUnavailable) {
				t.Fatalf("MergeRecords error %v wraps ErrUnavailable; a provider shape or configuration fault is operational, never unavailability", err)
			}
		})
	}
}

// TestGitHubMergeRecords_FailsClosedOnLinkPagination pins the strict Link
// walk at this call site (lane EF review F5): a cycle, a malformed or
// ambiguous rel="next", and a later page that is null or carries trailing
// data are each an operational error, never a quietly truncated list. Each
// row names the error it must fail with.
func TestGitHubMergeRecords_FailsClosedOnLinkPagination(t *testing.T) {
	merged := mrtEncode(t, []any{mrtPull(12, "closed", "2026-09-20T10:15:30Z", mrtCommit, "main")})
	tests := []struct {
		name    string
		setup   func(*mrtServer)
		wantErr string
	}{
		{"a Link cycle back to the first page", func(s *mrtServer) {
			s.pullPages = []string{merged, `[]`}
			s.links = map[int]string{2: `<{self}?per_page=100>; rel="next"`}
		}, "pagination cycle detected"},
		{"a Link naming the page it came from", func(s *mrtServer) {
			s.pullPages = []string{merged}
			s.links = map[int]string{1: `<{self}?per_page=100>; rel="next"`}
		}, "pagination cycle detected"},
		{"a rel=next Link without an angle-bracketed target", func(s *mrtServer) {
			s.pullPages = []string{merged, `[]`}
			s.links = map[int]string{1: `{self}?per_page=100&page=2; rel="next"`}
		}, `malformed approval pagination Link claims rel="next"`},
		{"a rel=next Link whose target is not a request URI", func(s *mrtServer) {
			s.pullPages = []string{merged, `[]`}
			s.links = map[int]string{1: `<page two>; rel="next"`}
		}, "malformed approval pagination next URL"},
		{"two distinct rel=next Links", func(s *mrtServer) {
			s.pullPages = []string{merged, `[]`, `[]`}
			s.links = map[int]string{1: `<{self}?per_page=100&page=2>; rel="next", <{self}?per_page=100&page=3>; rel="next"`}
		}, "multiple distinct next continuations"},
		{"a next page that is null", func(s *mrtServer) {
			s.pullPages = []string{merged, `null`}
		}, "page must be a non-null array"},
		{"a next page with trailing data", func(s *mrtServer) {
			s.pullPages = []string{merged, `[]` + "\n" + `[]`}
		}, "trailing data after approval response"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newMRTServer(t)
			tt.setup(s)
			facts, err := s.adapter().MergeRecords(context.Background(), mrtCommit)
			if err == nil {
				t.Fatalf("MergeRecords = %+v, want an operational error naming %q", facts, tt.wantErr)
			}
			if errors.Is(err, forge.ErrUnavailable) {
				t.Fatalf("MergeRecords error %v wraps ErrUnavailable; ambiguous pagination is operational, never unavailability", err)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("MergeRecords error %q, want it to name %q", err, tt.wantErr)
			}
		})
	}
}

// TestGitHubMergeRecords_RefusesAnotherRepositorysPullRequest pins lane EF
// review F2: the endpoint's published description does not bind a listed
// pull request's base repository to the queried one, so the adapter compares
// base.repo.id (required in pull-request-simple) with the id of "Get a
// repository" (required in full-repository). A mismatch is a provider
// contract violation, reported as an operational error — never unavailability,
// and never a pull request silently dropped — and a missing id on either side
// is refused. Each row names the error it must fail with, so no row can pass
// through another rule.
func TestGitHubMergeRecords_RefusesAnotherRepositorysPullRequest(t *testing.T) {
	foreign := func(number int64) map[string]any {
		pull := mrtPull(number, "closed", "2026-09-20T10:15:30Z", mrtCommit, "main")
		pull["base"] = map[string]any{"ref": "main", "sha": mrtOther, "repo": map[string]any{"id": 999, "full_name": "someone-else/fork"}}
		return pull
	}
	baseRepo := func(pull map[string]any) map[string]any {
		return pull["base"].(map[string]any)["repo"].(map[string]any)
	}
	tests := []struct {
		name    string
		setup   func(*mrtServer)
		wantErr string
	}{
		{"review probe P1: a merged pull request based in another repository", func(s *mrtServer) {
			s.pullPages = []string{mrtEncode(t, []any{foreign(99)})}
		}, "pull request 99 targets base repository id 999 (someone-else/fork), not the queried repository id 1296269"},
		{"a pull request of another repository beside one of this repository is not dropped", func(s *mrtServer) {
			s.pullPages = []string{mrtEncode(t, []any{mrtPull(12, "closed", "2026-09-20T10:15:30Z", mrtCommit, "main"), foreign(99)})}
		}, "not the queried repository id 1296269"},
		{"a pull request of another repository on a later page", func(s *mrtServer) {
			s.pullPages = []string{
				mrtEncode(t, []any{mrtPull(12, "open", nil, nil, "main")}),
				mrtEncode(t, []any{foreign(99)}),
			}
		}, "not the queried repository id 1296269"},
		{"a pull request whose base has no repository", func(s *mrtServer) {
			pull := mrtPull(5, "open", nil, nil, "main")
			delete(pull["base"].(map[string]any), "repo")
			s.pullPages = []string{mrtEncode(t, []any{pull})}
		}, "pull request 5 carries no base repository"},
		{"a pull request whose base repository is null", func(s *mrtServer) {
			pull := mrtPull(5, "open", nil, nil, "main")
			pull["base"].(map[string]any)["repo"] = nil
			s.pullPages = []string{mrtEncode(t, []any{pull})}
		}, "pull request 5 carries no base repository"},
		{"a pull request whose base repository has no id", func(s *mrtServer) {
			pull := mrtPull(5, "open", nil, nil, "main")
			delete(baseRepo(pull), "id")
			s.pullPages = []string{mrtEncode(t, []any{pull})}
		}, "pull request 5 base repository carries no positive id"},
		{"a pull request whose base repository id is of the wrong type", func(s *mrtServer) {
			pull := mrtPull(5, "open", nil, nil, "main")
			baseRepo(pull)["id"] = "1296269"
			s.pullPages = []string{mrtEncode(t, []any{pull})}
		}, "id of type int64"},
		{"a repository without an id", func(s *mrtServer) {
			s.repoBody = `{"default_branch":"main","full_name":"acme/svcfix"}`
		}, "repository acme/svcfix reports no positive id"},
		{"a repository with a null id", func(s *mrtServer) {
			s.repoBody = `{"id":null,"default_branch":"main"}`
		}, "repository acme/svcfix reports no positive id"},
		{"a repository id of the wrong type", func(s *mrtServer) {
			s.repoBody = `{"id":"1296269","default_branch":"main"}`
		}, "id of type int64"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newMRTServer(t)
			tt.setup(s)
			facts, err := s.adapter().MergeRecords(context.Background(), mrtCommit)
			if err == nil {
				t.Fatalf("MergeRecords = %+v, want an operational error naming %q", facts, tt.wantErr)
			}
			if errors.Is(err, forge.ErrUnavailable) {
				t.Fatalf("MergeRecords error %v wraps ErrUnavailable; a provider contract violation is operational, never unavailability", err)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("MergeRecords error %q, want it to name %q", err, tt.wantErr)
			}
			if !reflect.DeepEqual(facts, forge.MergeRecordFacts{}) {
				t.Fatalf("MergeRecords returned facts %+v beside its error, want the zero value", facts)
			}
		})
	}
}

func TestGitHubMergeRecords_UnavailabilityWrapsErrUnavailable(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*mrtServer)
	}{
		{"a 5xx on the repository read", func(s *mrtServer) { s.repoStatus = http.StatusBadGateway }},
		{"a 5xx on the pull request list", func(s *mrtServer) { s.pullStatus = http.StatusServiceUnavailable }},
		{"a 429 on the pull request list", func(s *mrtServer) { s.pullStatus = http.StatusTooManyRequests }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newMRTServer(t)
			tt.setup(s)
			if facts, err := s.adapter().MergeRecords(context.Background(), mrtCommit); !errors.Is(err, forge.ErrUnavailable) {
				t.Fatalf("MergeRecords = %+v, %v; want an error wrapping ErrUnavailable", facts, err)
			}
		})
	}
	t.Run("a transport failure", func(t *testing.T) {
		server := httptest.NewServer(http.NotFoundHandler())
		base := server.URL
		server.Close()
		a := New(Config{BaseURL: base, Owner: "acme", Repo: "svcfix", HTTPClient: &http.Client{Timeout: 2 * time.Second}, Clock: mrtClock})
		if facts, err := a.MergeRecords(context.Background(), mrtCommit); !errors.Is(err, forge.ErrUnavailable) {
			t.Fatalf("MergeRecords against a closed server = %+v, %v; want an error wrapping ErrUnavailable", facts, err)
		}
	})
}
