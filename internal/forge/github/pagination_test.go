package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/forge"
)

// TestParseLinkNext is a table-driven unit test over parseLinkNext's
// well-formed, absent, and malformed inputs (spec/forge-transport co-1's
// "Link-header parsing negatives... stop cleanly, no infinite loop" —
// exercised here at the parse-function level; the walker-level "stops
// cleanly" behavior is proven by TestListOpenMRs_MalformedLinkHeader_
// StopsCleanly below).
func TestParseLinkNext(t *testing.T) {
	tests := []struct {
		name, header, want string
	}{
		{"absent header", "", ""},
		{"header present but carries no rel=\"next\" member", `<https://api.github.com/x?page=9>; rel="last"`, ""},
		{"malformed: no angle brackets around the URL", `https://api.github.com/x?page=2; rel="next"`, ""},
		{"malformed: no semicolon/rel segment at all", `<https://api.github.com/x?page=2>`, ""},
		{"malformed: garbage string", "garbled-not-a-link-header;;;", ""},
		{
			"well-formed next among other rels",
			`<https://api.github.com/x?page=2>; rel="next", <https://api.github.com/x?page=9>; rel="last"`,
			"https://api.github.com/x?page=2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLinkNext(tt.header)
			if got != tt.want {
				t.Errorf("parseLinkNext(%q) = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}

// TestListOpenMRs_DrainsMultiplePages_LinkHeader is ac-2's REST witness for
// GitHub: page one is "full" (from this fake's perspective) and carries a
// Link rel="next"; the decisive open MR (#101) sits on page two only. A
// walker that stops at page one would never see it — pendingsupersession.go
// would silently drop the candidate (the spec's own framing).
func TestListOpenMRs_DrainsMultiplePages_LinkHeader(t *testing.T) {
	var ts *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/svcfix/pulls", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			_ = json.NewEncoder(w).Encode([]pullRequestJSON{
				{Number: 101, Title: "decisive, page two only", Head: struct {
					Ref string `json:"ref"`
				}{Ref: "design/page-two-branch"}},
			})
			return
		}
		w.Header().Set("Link", fmt.Sprintf(`<%s/repos/acme/svcfix/pulls?state=open&base=main&per_page=100&page=2>; rel="next"`, ts.URL))
		_ = json.NewEncoder(w).Encode([]pullRequestJSON{
			{Number: 1, Title: "page one", Head: struct {
				Ref string `json:"ref"`
			}{Ref: "design/page-one-branch"}},
		})
	})
	ts = httptest.NewServer(mux)
	defer ts.Close()

	a := New(Config{BaseURL: ts.URL, Owner: "acme", Repo: "svcfix", HTTPClient: ts.Client()})
	mrs, err := a.ListOpenMRs(context.Background(), "main")
	if err != nil {
		t.Fatalf("ListOpenMRs: %v", err)
	}
	if len(mrs) != 2 {
		t.Fatalf("ListOpenMRs drained %d MRs, want 2 (one per page): %+v", len(mrs), mrs)
	}
	var found bool
	for _, m := range mrs {
		if m.ID == "101" {
			found = true
		}
	}
	if !found {
		t.Fatalf("ListOpenMRs drained result missing the page-two decisive MR #101: %+v", mrs)
	}
}

// TestListOpenMRs_MalformedLinkHeader_StopsCleanly proves a Link header
// that IS present but does not parse to a rel="next" URL stops the walk
// after page one — no error, no second request, no hang (co-1's negative
// witness).
func TestListOpenMRs_MalformedLinkHeader_StopsCleanly(t *testing.T) {
	requests := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/svcfix/pulls", func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Link", "garbled-not-a-link-header;;;")
		_ = json.NewEncoder(w).Encode([]pullRequestJSON{{Number: 1, Title: "only page"}})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	a := New(Config{BaseURL: ts.URL, Owner: "acme", Repo: "svcfix", HTTPClient: ts.Client()})
	mrs, err := a.ListOpenMRs(context.Background(), "main")
	if err != nil {
		t.Fatalf("ListOpenMRs with a malformed Link header: %v", err)
	}
	if len(mrs) != 1 {
		t.Fatalf("ListOpenMRs with a malformed Link header returned %d MRs, want exactly 1 (stop cleanly on page one)", len(mrs))
	}
	if requests != 1 {
		t.Fatalf("ListOpenMRs with a malformed Link header issued %d requests, want exactly 1", requests)
	}
}

// TestGithubDrainList_RepeatedNextURL_FailsLoud proves a server whose Link
// header's rel="next" always points back at the exact URL just fetched
// fails loud (a named "pagination loop detected" error) rather than
// looping forever — bounded by the guard itself, not by test design
// racing a hang (co-1: "a fake that always returns a full page with a next
// link must be bounded by test design, not by a production cap" — this
// fake IS bounded, by the guard firing on the very first repeat).
func TestGithubDrainList_RepeatedNextURL_FailsLoud(t *testing.T) {
	var ts *httptest.Server
	requests := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/svcfix/pulls", func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Link", fmt.Sprintf(`<%s>; rel="next"`, ts.URL+r.URL.String()))
		_ = json.NewEncoder(w).Encode([]pullRequestJSON{{Number: 1}})
	})
	ts = httptest.NewServer(mux)
	defer ts.Close()

	a := New(Config{BaseURL: ts.URL, Owner: "acme", Repo: "svcfix", HTTPClient: ts.Client()})
	_, err := a.ListOpenMRs(context.Background(), "main")
	if err == nil {
		t.Fatal("ListOpenMRs against a Link header repeating the just-fetched URL: want error, got nil")
	}
	if !strings.Contains(err.Error(), "loop") {
		t.Errorf("error %q does not name the pagination loop", err.Error())
	}
	if requests != 1 {
		t.Fatalf("ListOpenMRs issued %d requests before failing loud, want exactly 1 (the guard fires on the first repeat, never spins)", requests)
	}
}

// TestFindRuns_DrainsMultiplePages_LinkHeader is ac-2's second GitHub REST
// walker witness (findRuns, distinct from ListOpenMRs): the matching
// successful run for commit sits on page two only.
func TestFindRuns_DrainsMultiplePages_LinkHeader(t *testing.T) {
	var ts *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/svcfix/actions/runs", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			_ = json.NewEncoder(w).Encode(runsResponse{WorkflowRuns: []run{
				{ID: 101, Status: "completed", Conclusion: "success"},
			}})
			return
		}
		w.Header().Set("Link", fmt.Sprintf(`<%s/repos/acme/svcfix/actions/runs?head_sha=deadbeef&status=success&per_page=100&page=2>; rel="next"`, ts.URL))
		_ = json.NewEncoder(w).Encode(runsResponse{WorkflowRuns: []run{
			{ID: 1, Status: "completed", Conclusion: "success"},
		}})
	})
	ts = httptest.NewServer(mux)
	defer ts.Close()

	a := New(Config{BaseURL: ts.URL, Owner: "acme", Repo: "svcfix", HTTPClient: ts.Client()})
	ids, err := a.findRuns(context.Background(), "deadbeef")
	if err != nil {
		t.Fatalf("findRuns: %v", err)
	}
	var found bool
	for _, id := range ids {
		if id == 101 {
			found = true
		}
	}
	if !found {
		t.Fatalf("findRuns drained result missing the page-two run id 101: %v", ids)
	}
}

// TestGetThreadResolution_DrainsOuterCursor_UnresolvedThreadPastPageOne is
// ac-2's GraphQL gate-pass witness: reviewThreads spans two cursor pages,
// and the thread on page two (id "t-101", the shape a thread past position
// 100 would take) is unresolved. GetThreadResolution — and transitively
// gate_threads.go's checkReviewThreadsCondition, which reads exactly this
// method's Resolved field — must report it unresolved; a walker that
// stopped at first:100 would never see this thread at all, and the gate
// would PASS a PR that should not (the spec's own framing: "an unresolved
// thread at position >100 fails the gate check").
func TestGetThreadResolution_DrainsOuterCursor_UnresolvedThreadPastPageOne(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		var req graphQLRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		cursor, _ := req.Variables["cursor"].(string)

		var resp reviewThreadsResponse
		switch cursor {
		case "":
			resp.Data.Repository.PullRequest.ReviewThreads.Nodes = []reviewThreadNode{
				{ID: "t-1", IsResolved: true},
			}
			resp.Data.Repository.PullRequest.ReviewThreads.PageInfo = graphQLPageInfo{HasNextPage: true, EndCursor: "cursor-2"}
		case "cursor-2":
			resp.Data.Repository.PullRequest.ReviewThreads.Nodes = []reviewThreadNode{
				{ID: "t-101", IsResolved: false},
			}
			resp.Data.Repository.PullRequest.ReviewThreads.PageInfo = graphQLPageInfo{HasNextPage: false}
		default:
			t.Errorf("unexpected GraphQL cursor variable %q", cursor)
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	a := New(Config{BaseURL: ts.URL, Owner: "acme", Repo: "svcfix", HTTPClient: ts.Client()})
	got, err := a.GetThreadResolution(context.Background(), "7")
	if err != nil {
		t.Fatalf("GetThreadResolution: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("GetThreadResolution drained %d threads, want 2 (one per cursor page): %+v", len(got), got)
	}
	var page2 *forge.ThreadResolution
	for i := range got {
		if got[i].ThreadID == "t-101" {
			page2 = &got[i]
		}
	}
	if page2 == nil {
		t.Fatalf("GetThreadResolution missing the page-two thread t-101: %+v", got)
	}
	if page2.Resolved {
		t.Fatal("GetThreadResolution reported the page-two thread t-101 as Resolved=true, want false — the gate-pass witness: this thread must fail checkReviewThreadsCondition")
	}
}

// TestGetThreadResolution_GraphQLCursorLoop_FailsLoud mirrors the REST
// same-URL guard for the GraphQL outer cursor: a server claiming
// hasNextPage:true while never advancing endCursor is a broken-server
// signature, not a legitimate multi-page shape.
func TestGetThreadResolution_GraphQLCursorLoop_FailsLoud(t *testing.T) {
	requests := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		requests++
		var resp reviewThreadsResponse
		resp.Data.Repository.PullRequest.ReviewThreads.Nodes = []reviewThreadNode{{ID: "t-1", IsResolved: true}}
		resp.Data.Repository.PullRequest.ReviewThreads.PageInfo = graphQLPageInfo{HasNextPage: true, EndCursor: ""}
		_ = json.NewEncoder(w).Encode(resp)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	a := New(Config{BaseURL: ts.URL, Owner: "acme", Repo: "svcfix", HTTPClient: ts.Client()})
	_, err := a.GetThreadResolution(context.Background(), "7")
	if err == nil {
		t.Fatal("GetThreadResolution against a hasNextPage:true/endCursor stuck response: want error, got nil")
	}
	if !strings.Contains(err.Error(), "loop") {
		t.Errorf("error %q does not name the pagination loop", err.Error())
	}
	if requests != 1 {
		t.Fatalf("GetThreadResolution issued %d GraphQL requests before failing loud, want exactly 1", requests)
	}
}

// TestListComments_DrainsInnerCommentsCursor_JoinsOverflowCommentToThread
// is the dc-3 inner-walk witness. FIELD ANALYSIS (see this file's package
// doc note in the implementer's report): GitHub's isResolved lives directly
// on the reviewThreads NODE, never derived from its comments connection —
// so an overflow comment page cannot itself flip a thread's resolution
// state, and the outer-cursor witness above already proves the gate-pass
// case standalone. What the inner comments cursor DOES feed is
// ListComments' REST-diff-comment-to-GraphQL-thread join
// (threadByDBID, github.go's ListComments): a diff comment whose databaseId
// only appears on the thread's overflow comments page would be joined to
// ThreadID "" (silently misclassified as threadless) if that page were
// never walked. This test proves the join survives the overflow.
func TestListComments_DrainsInnerCommentsCursor_JoinsOverflowCommentToThread(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/svcfix/pulls/9/comments", func(w http.ResponseWriter, r *http.Request) {
		line := 42
		_ = json.NewEncoder(w).Encode([]reviewCommentJSON{
			{ID: 101, Body: "overflow-page comment", Path: "x.go", Line: &line, CreatedAt: "2026-07-13T00:00:00Z"},
		})
	})
	mux.HandleFunc("/repos/acme/svcfix/issues/9/comments", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]issueCommentJSON{})
	})
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		var req graphQLRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if _, isInner := req.Variables["threadID"]; isInner {
			var resp threadCommentsResponse
			resp.Data.Node.Comments.Nodes = []threadCommentNode{{DatabaseID: 101}}
			resp.Data.Node.Comments.PageInfo = graphQLPageInfo{HasNextPage: false}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		var resp reviewThreadsResponse
		node := reviewThreadNode{ID: "t-overflow", IsResolved: false}
		node.Comments.Nodes = []threadCommentNode{{DatabaseID: 1}} // page-one databaseId, NOT 101
		node.Comments.PageInfo = graphQLPageInfo{HasNextPage: true, EndCursor: "inner-cursor"}
		resp.Data.Repository.PullRequest.ReviewThreads.Nodes = []reviewThreadNode{node}
		_ = json.NewEncoder(w).Encode(resp)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	a := New(Config{BaseURL: ts.URL, Owner: "acme", Repo: "svcfix", HTTPClient: ts.Client()})
	comments, err := a.ListComments(context.Background(), "9")
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	var got *forge.Comment
	for i := range comments {
		if comments[i].ID == "101" {
			got = &comments[i]
		}
	}
	if got == nil {
		t.Fatalf("ListComments dropped the diff comment entirely: %+v", comments)
	}
	if got.ThreadID != "t-overflow" {
		t.Fatalf("ListComments joined databaseId 101 to ThreadID %q, want %q — the inner comments cursor page was not drained", got.ThreadID, "t-overflow")
	}
}
