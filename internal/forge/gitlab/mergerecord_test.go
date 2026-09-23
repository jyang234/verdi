package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/forge"
)

// Transport-level tests for the GitLab merge-record read (SI-249; plan
// R-PB-2). The mapping from GitLab's published examples is covered in
// internal/forge/mergerecord_published_test.go; these drive the adapter's
// routes, pagination, and error classification over minimal bodies.

const mrtCommit = "6dcb09b5b57875f334f61aebed695e2e4193db5e"

func mrtClock() time.Time { return time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC) }

// mrtMR is a minimal merge request body carrying the members the adapter
// reads plus two it never reads (title, squash_commit_sha), which must be
// ignored.
func mrtMR(iid int64, state string, mergedAt, mergeSHA any, target string) map[string]any {
	return map[string]any{
		"iid": iid, "state": state, "merged_at": mergedAt, "merge_commit_sha": mergeSHA,
		"target_branch": target, "title": "unmodeled", "squash_commit_sha": mrtCommit,
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

// mrtServer answers the two merge-record routes for project 42 and records
// every request.
type mrtServer struct {
	t             *testing.T
	projectStatus int
	projectBody   string
	mrStatus      int
	mrPages       []string // raw bodies; page n links page n+1 with X-Next-Page
	nextPage      map[int]string

	mu       sync.Mutex
	requests []string
}

func newMRTServer(t *testing.T, mrs ...map[string]any) *mrtServer {
	t.Helper()
	page := make([]any, 0, len(mrs))
	for _, mr := range mrs {
		page = append(page, mr)
	}
	return &mrtServer{
		t: t, projectStatus: http.StatusOK, projectBody: `{"id":42,"default_branch":"main","name":"svcfix"}`,
		mrStatus: http.StatusOK, mrPages: []string{mrtEncode(t, page)},
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requests = append(s.requests, r.URL.Path+"?"+r.URL.RawQuery)
		s.mu.Unlock()
		switch r.URL.Path {
		case "/projects/42":
			w.WriteHeader(s.projectStatus)
			_, _ = w.Write([]byte(s.projectBody))
		case "/projects/42/repository/commits/" + mrtCommit + "/merge_requests":
			if s.mrStatus != http.StatusOK {
				w.WriteHeader(s.mrStatus)
				return
			}
			page, err := strconv.Atoi(r.URL.Query().Get("page"))
			if err != nil || page < 1 || page > len(s.mrPages) {
				t.Errorf("unexpected page %q", r.URL.Query().Get("page"))
				http.NotFound(w, r)
				return
			}
			if next, ok := s.nextPage[page]; ok {
				w.Header().Set("X-Next-Page", next)
			} else if page < len(s.mrPages) {
				w.Header().Set("X-Next-Page", strconv.Itoa(page+1))
			}
			_, _ = w.Write([]byte(s.mrPages[page-1]))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return New(Config{BaseURL: server.URL, ProjectID: "42", HTTPClient: server.Client(), Clock: mrtClock})
}

func TestGitLabMergeRecords_ReadsProjectThenMergeRequests(t *testing.T) {
	s := newMRTServer(t, mrtMR(7, "merged", "2026-09-20T10:15:30.123Z", mrtCommit, "main"))
	facts, err := s.adapter().MergeRecords(context.Background(), mrtCommit)
	if err != nil {
		t.Fatalf("MergeRecords: %v", err)
	}
	want := []string{
		"/projects/42?",
		"/projects/42/repository/commits/" + mrtCommit + "/merge_requests?page=1&per_page=100",
	}
	if got := s.log(); !reflect.DeepEqual(got, want) {
		t.Fatalf("requests = %v, want %v", got, want)
	}
	if !facts.Supported || facts.Repository != "42" || facts.Commit != mrtCommit || facts.DefaultBranch != "main" {
		t.Fatalf("facts = %+v", facts)
	}
	wantChanges := []forge.ChangeRequestMerge{{
		ChangeID: "7", State: forge.ChangeRequestMerged, TargetBranch: "main",
		MergeCommitSHA: mrtCommit, MergedAt: "2026-09-20T10:15:30.123Z",
	}}
	if !reflect.DeepEqual(facts.Changes, wantChanges) {
		t.Fatalf("changes = %+v, want %+v", facts.Changes, wantChanges)
	}
	if err := facts.Validate(); err != nil {
		t.Fatalf("facts do not validate: %v", err)
	}
}

func TestGitLabMergeRecords_RepositoryIsTheStableProjectID(t *testing.T) {
	s := newMRTServer(t)
	s.projectBody = `{"id":4242,"default_branch":"trunk","path_with_namespace":"acme/svcfix"}`
	facts, err := s.adapter().MergeRecords(context.Background(), mrtCommit)
	if err != nil {
		t.Fatalf("MergeRecords: %v", err)
	}
	if facts.Repository != "4242" || facts.DefaultBranch != "trunk" {
		t.Fatalf("facts = %+v, want the project's own id and default branch", facts)
	}
}

func TestGitLabMergeRecords_DrainsEveryPage(t *testing.T) {
	s := newMRTServer(t)
	s.mrPages = []string{
		mrtEncode(t, []any{mrtMR(30, "opened", nil, nil, "main")}),
		mrtEncode(t, []any{mrtMR(4, "closed", nil, nil, "main")}),
		mrtEncode(t, []any{mrtMR(200, "merged", "2026-09-20T10:15:30Z", mrtCommit, "main")}),
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
		t.Fatalf("requests = %v, want the project plus three pages", s.log())
	}
}

func TestGitLabMergeRecords_RefusesNonSHAInputBeforeAnyRequest(t *testing.T) {
	for _, commit := range []string{"", "main", "HEAD~1", mrtCommit[:12], strings.ToUpper(mrtCommit), mrtCommit + "a", "../../merge_requests"} {
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

func TestGitLabMergeRecords_FailsClosedOnProviderShape(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*mrtServer)
	}{
		{"unknown merge request state", func(s *mrtServer) {
			s.mrPages = []string{mrtEncode(t, []any{mrtMR(1, "reopened", nil, nil, "main")})}
		}},
		{"a GitHub state name", func(s *mrtServer) {
			s.mrPages = []string{mrtEncode(t, []any{mrtMR(1, "open", nil, nil, "main")})}
		}},
		{"empty merge request state", func(s *mrtServer) {
			s.mrPages = []string{mrtEncode(t, []any{mrtMR(1, "", nil, nil, "main")})}
		}},
		{"missing iid", func(s *mrtServer) {
			mr := mrtMR(1, "opened", nil, nil, "main")
			delete(mr, "iid")
			s.mrPages = []string{mrtEncode(t, []any{mr})}
		}},
		{"zero iid", func(s *mrtServer) {
			s.mrPages = []string{mrtEncode(t, []any{mrtMR(0, "opened", nil, nil, "main")})}
		}},
		{"negative iid", func(s *mrtServer) {
			s.mrPages = []string{mrtEncode(t, []any{mrtMR(-2, "opened", nil, nil, "main")})}
		}},
		{"iid of the wrong type", func(s *mrtServer) {
			mr := mrtMR(1, "opened", nil, nil, "main")
			mr["iid"] = "1"
			s.mrPages = []string{mrtEncode(t, []any{mr})}
		}},
		{"missing target branch", func(s *mrtServer) {
			mr := mrtMR(1, "opened", nil, nil, "main")
			delete(mr, "target_branch")
			s.mrPages = []string{mrtEncode(t, []any{mr})}
		}},
		{"merged without merged_at", func(s *mrtServer) {
			s.mrPages = []string{mrtEncode(t, []any{mrtMR(1, "merged", nil, mrtCommit, "main")})}
		}},
		{"merged with an unparseable merged_at", func(s *mrtServer) {
			s.mrPages = []string{mrtEncode(t, []any{mrtMR(1, "merged", "last tuesday", mrtCommit, "main")})}
		}},
		{"merged with an abbreviated merge commit", func(s *mrtServer) {
			s.mrPages = []string{mrtEncode(t, []any{mrtMR(1, "merged", "2026-09-20T10:15:30Z", mrtCommit[:8], "main")})}
		}},
		{"duplicate merge request across pages", func(s *mrtServer) {
			s.mrPages = []string{
				mrtEncode(t, []any{mrtMR(1, "opened", nil, nil, "main")}),
				mrtEncode(t, []any{mrtMR(1, "opened", nil, nil, "main")}),
			}
		}},
		{"a null page", func(s *mrtServer) { s.mrPages = []string{"null"} }},
		{"an object page", func(s *mrtServer) { s.mrPages = []string{`{"merge_requests":[]}`} }},
		{"trailing data after a page", func(s *mrtServer) { s.mrPages = []string{`[]` + "\n" + `[]`} }},
		{"a malformed X-Next-Page", func(s *mrtServer) {
			s.mrPages = []string{`[]`, `[]`}
			s.nextPage = map[int]string{1: "two"}
		}},
		{"an X-Next-Page that revisits a page", func(s *mrtServer) {
			s.mrPages = []string{`[]`, `[]`}
			s.nextPage = map[int]string{1: "2", 2: "1"}
		}},
		{"trailing data after the project", func(s *mrtServer) {
			s.projectBody = `{"id":42,"default_branch":"main"} {"id":43}`
		}},
		{"project without a default branch", func(s *mrtServer) { s.projectBody = `{"id":42}` }},
		{"project with a null default branch", func(s *mrtServer) { s.projectBody = `{"id":42,"default_branch":null}` }},
		{"project without an id", func(s *mrtServer) { s.projectBody = `{"default_branch":"main"}` }},
		{"project id of the wrong type", func(s *mrtServer) { s.projectBody = `{"id":"42","default_branch":"main"}` }},
		{"a 404 on the project read", func(s *mrtServer) {
			s.projectStatus = http.StatusNotFound
			s.projectBody = `{"message":"404 Project Not Found"}`
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

func TestGitLabMergeRecords_UnavailabilityWrapsErrUnavailable(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*mrtServer)
	}{
		{"a 5xx on the project read", func(s *mrtServer) { s.projectStatus = http.StatusBadGateway }},
		{"a 5xx on the merge request list", func(s *mrtServer) { s.mrStatus = http.StatusServiceUnavailable }},
		{"a 429 on the merge request list", func(s *mrtServer) { s.mrStatus = http.StatusTooManyRequests }},
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
		a := New(Config{BaseURL: base, ProjectID: "42", HTTPClient: &http.Client{Timeout: 2 * time.Second}, Clock: mrtClock})
		if facts, err := a.MergeRecords(context.Background(), mrtCommit); !errors.Is(err, forge.ErrUnavailable) {
			t.Fatalf("MergeRecords against a closed server = %+v, %v; want an error wrapping ErrUnavailable", facts, err)
		}
	})
}
