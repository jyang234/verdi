package github

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/forge"
	"github.com/OWNER/verdi/internal/forge/forgetest"
)

func buildBundleZip(t *testing.T, b forge.EvidenceBundle) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	files := map[string][]byte{
		"derived/spec--x/deadbeef/verdicts.json":      b.Verdicts,
		"derived/spec--x/deadbeef/tests.json":         b.Tests,
		"derived/spec--x/deadbeef/review.json":        b.Review,
		"derived/spec--x/deadbeef/boundary-diff.json": b.BoundaryDiff,
	}
	for name, content := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("zip.Create(%s): %v", name, err)
		}
		if _, err := fw.Write(content); err != nil {
			t.Fatalf("writing %s into zip: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("closing zip writer: %v", err)
	}
	return buf.Bytes()
}

type fakeGitHubServer struct {
	byCommit     map[string][]byte
	runIDs       map[string]int64
	prsByBase    map[string][]pullRequestJSON
	filesByRef   map[string]map[string][]byte
	nextPRNumber int64

	// diffComments/generalComments/threads are keyed by mrID (PR number,
	// string-rendered) — the V1-P7 comment-round-trip extension.
	diffComments    map[string][]reviewCommentJSON
	generalComments map[string][]issueCommentJSON
	threads         map[string][]reviewThreadNode // mrID -> threads
	nextCommentID   int64
}

type harness struct {
	srv     *fakeGitHubServer
	adapter *Adapter
}

func newHarnessForTest(t *testing.T) *harness {
	t.Helper()
	srv := &fakeGitHubServer{
		byCommit:        map[string][]byte{},
		runIDs:          map[string]int64{},
		prsByBase:       map[string][]pullRequestJSON{},
		filesByRef:      map[string]map[string][]byte{},
		nextPRNumber:    1,
		diffComments:    map[string][]reviewCommentJSON{},
		generalComments: map[string][]issueCommentJSON{},
		threads:         map[string][]reviewThreadNode{},
		nextCommentID:   1000,
	}
	nextRunID := int64(100)

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/svcfix/pulls", func(w http.ResponseWriter, r *http.Request) {
		base := r.URL.Query().Get("base")
		_ = json.NewEncoder(w).Encode(srv.prsByBase[base])
	})
	mux.HandleFunc("/repos/acme/svcfix/contents/", func(w http.ResponseWriter, r *http.Request) {
		const prefix = "/repos/acme/svcfix/contents/"
		path := r.URL.Path[len(prefix):]
		ref := r.URL.Query().Get("ref")
		content, ok := srv.filesByRef[ref][path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(repoContentJSON{
			Content:  base64.StdEncoding.EncodeToString(content),
			Encoding: "base64",
		})
	})
	mux.HandleFunc("/repos/acme/svcfix/actions/runs", func(w http.ResponseWriter, r *http.Request) {
		sha := r.URL.Query().Get("head_sha")
		if _, ok := srv.byCommit[sha]; !ok {
			_ = json.NewEncoder(w).Encode(runsResponse{})
			return
		}
		id, ok := srv.runIDs[sha]
		if !ok {
			id = nextRunID
			nextRunID++
			srv.runIDs[sha] = id
		}
		_ = json.NewEncoder(w).Encode(runsResponse{WorkflowRuns: []run{{ID: id, Status: "completed", Conclusion: "success"}}})
	})
	mux.HandleFunc("/repos/acme/svcfix/actions/runs/", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(artifactsResponse{Artifacts: []artifact{{ID: 7, Name: defaultArtifactName}}})
	})
	mux.HandleFunc("/repos/acme/svcfix/actions/artifacts/7/zip", func(w http.ResponseWriter, r *http.Request) {
		for _, zipData := range srv.byCommit {
			_, _ = w.Write(zipData)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	// /repos/acme/svcfix/pulls/{id} (bare -> head sha) and
	// /repos/acme/svcfix/pulls/{id}/comments (list/post diff comments) —
	// V1-P7's comment-round-trip extension.
	mux.HandleFunc("/repos/acme/svcfix/pulls/", func(w http.ResponseWriter, r *http.Request) {
		const prefix = "/repos/acme/svcfix/pulls/"
		rest := r.URL.Path[len(prefix):]
		parts := strings.SplitN(rest, "/", 2)
		mrID := parts[0]
		if len(parts) == 2 && parts[1] == "comments" {
			switch r.Method {
			case http.MethodGet:
				_ = json.NewEncoder(w).Encode(srv.diffComments[mrID])
			case http.MethodPost:
				var req createReviewCommentRequest
				_ = json.NewDecoder(r.Body).Decode(&req)
				srv.nextCommentID++
				id := srv.nextCommentID
				line := req.Line
				rc := reviewCommentJSON{ID: id, Body: req.Body, Path: req.Path, Line: &line, CreatedAt: "2026-07-11T18:00:00Z"}
				rc.User.Login = "fake-poster"
				srv.diffComments[mrID] = append(srv.diffComments[mrID], rc)
				node := reviewThreadNode{ID: "PRRT_fake_" + strconv.FormatInt(id, 10)}
				node.Comments.Nodes = append(node.Comments.Nodes, struct {
					DatabaseID int64 `json:"databaseId"`
				}{DatabaseID: id})
				srv.threads[mrID] = append(srv.threads[mrID], node)
				_ = json.NewEncoder(w).Encode(rc)
			}
			return
		}
		// Bare "{id}": PostComment's pre-fetch for the PR head sha.
		_ = json.NewEncoder(w).Encode(pullHeadJSON{Head: struct {
			SHA string `json:"sha"`
		}{SHA: "fake-head-sha"}})
	})
	// /repos/acme/svcfix/issues/{id}/comments (list/post general comments).
	mux.HandleFunc("/repos/acme/svcfix/issues/", func(w http.ResponseWriter, r *http.Request) {
		const prefix = "/repos/acme/svcfix/issues/"
		rest := r.URL.Path[len(prefix):]
		parts := strings.SplitN(rest, "/", 2)
		mrID := parts[0]
		if len(parts) != 2 || parts[1] != "comments" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(srv.generalComments[mrID])
		case http.MethodPost:
			var req createIssueCommentRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			srv.nextCommentID++
			id := srv.nextCommentID
			ic := issueCommentJSON{ID: id, Body: req.Body, CreatedAt: "2026-07-11T18:00:00Z"}
			ic.User.Login = "fake-poster"
			srv.generalComments[mrID] = append(srv.generalComments[mrID], ic)
			_ = json.NewEncoder(w).Encode(ic)
		}
	})
	// /graphql: reviewThreads query only (this adapter never issues any
	// other GraphQL query).
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		var req graphQLRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		numberF, _ := req.Variables["number"].(float64)
		mrID := strconv.FormatInt(int64(numberF), 10)
		var resp reviewThreadsResponse
		resp.Data.Repository.PullRequest.ReviewThreads.Nodes = srv.threads[mrID]
		_ = json.NewEncoder(w).Encode(resp)
	})

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	adapter := New(Config{
		BaseURL:    ts.URL,
		Owner:      "acme",
		Repo:       "svcfix",
		Token:      "test-token",
		HTTPClient: ts.Client(),
	})
	return &harness{srv: srv, adapter: adapter}
}

func (h *harness) Forge() forge.Forge { return h.adapter }

func (h *harness) SeedBundle(t *testing.T, ref, commit string, bundle forge.EvidenceBundle) {
	t.Helper()
	h.srv.byCommit[commit] = buildBundleZip(t, bundle)
}

func (h *harness) WantGeneratedAttribute() string { return "linguist-generated" }

func (h *harness) SeedOpenMR(t *testing.T, targetBranch, sourceBranch, title string) {
	t.Helper()
	h.srv.prsByBase[targetBranch] = append(h.srv.prsByBase[targetBranch], pullRequestJSON{
		Number: h.srv.nextPRNumber,
		Title:  title,
		Head: struct {
			Ref string `json:"ref"`
		}{Ref: sourceBranch},
	})
	h.srv.nextPRNumber++
}

func (h *harness) SeedFile(t *testing.T, ref, path string, content []byte) {
	t.Helper()
	if h.srv.filesByRef[ref] == nil {
		h.srv.filesByRef[ref] = map[string][]byte{}
	}
	h.srv.filesByRef[ref][path] = content
}

// SeedComment arranges for ListComments(mrID) to already include c: a
// ThreadID-carrying comment becomes a diff comment plus its (unresolved,
// unless SeedThreadResolution overrides it) thread node; a ThreadID-less
// comment becomes a general/issue comment — mirroring how
// ListComments/GetThreadResolution themselves classify GitHub's two
// comment universes (github.go's ListComments doc comment).
func (h *harness) SeedComment(t *testing.T, mrID string, c forge.Comment) {
	t.Helper()
	if c.ThreadID == "" {
		h.srv.nextCommentID++
		ic := issueCommentJSON{ID: h.srv.nextCommentID, Body: c.Body, CreatedAt: c.CreatedAt}
		ic.User.Login = c.Author
		h.srv.generalComments[mrID] = append(h.srv.generalComments[mrID], ic)
		return
	}

	h.srv.nextCommentID++
	id := h.srv.nextCommentID
	line := c.Line
	rc := reviewCommentJSON{ID: id, Body: c.Body, Path: c.Path, Line: &line, CreatedAt: c.CreatedAt}
	rc.User.Login = c.Author
	h.srv.diffComments[mrID] = append(h.srv.diffComments[mrID], rc)

	dbRef := struct {
		DatabaseID int64 `json:"databaseId"`
	}{DatabaseID: id}
	for i := range h.srv.threads[mrID] {
		if h.srv.threads[mrID][i].ID == c.ThreadID {
			h.srv.threads[mrID][i].Comments.Nodes = append(h.srv.threads[mrID][i].Comments.Nodes, dbRef)
			return
		}
	}
	node := reviewThreadNode{ID: c.ThreadID}
	node.Comments.Nodes = append(node.Comments.Nodes, dbRef)
	h.srv.threads[mrID] = append(h.srv.threads[mrID], node)
}

// SeedThreadResolution sets threadID's resolution state directly,
// creating the thread node if SeedComment has not already.
func (h *harness) SeedThreadResolution(t *testing.T, mrID string, tr forge.ThreadResolution) {
	t.Helper()
	var resolvedBy *struct {
		Login string `json:"login"`
	}
	if tr.ResolvedBy != "" {
		resolvedBy = &struct {
			Login string `json:"login"`
		}{Login: tr.ResolvedBy}
	}
	for i := range h.srv.threads[mrID] {
		if h.srv.threads[mrID][i].ID == tr.ThreadID {
			h.srv.threads[mrID][i].IsResolved = tr.Resolved
			h.srv.threads[mrID][i].ResolvedBy = resolvedBy
			return
		}
	}
	node := reviewThreadNode{ID: tr.ThreadID, IsResolved: tr.Resolved, ResolvedBy: resolvedBy}
	h.srv.threads[mrID] = append(h.srv.threads[mrID], node)
}

// TestGitHub_ContractSuite proves the GitHub adapter satisfies the forge
// contract suite against an httptest double of GitHub's own API — no
// network (CLAUDE.md).
func TestGitHub_ContractSuite(t *testing.T) {
	forgetest.Run(t, func(t *testing.T) forgetest.Harness {
		return newHarnessForTest(t)
	})
}

func TestGitHub_FetchEvidenceBundle_NoBundleWrapsErrNoBundle(t *testing.T) {
	h := newHarnessForTest(t)
	_, err := h.adapter.FetchEvidenceBundle(context.Background(), "spec/x", "0000000000000000000000000000000000000000")
	if !errors.Is(err, forge.ErrNoBundle) {
		t.Fatalf("error = %v, want errors.Is(err, forge.ErrNoBundle)", err)
	}
}

func TestGitHub_GeneratedAttribute(t *testing.T) {
	a := New(Config{Owner: "acme", Repo: "svcfix"})
	if got := a.GeneratedAttribute(); got != "linguist-generated" {
		t.Errorf("GeneratedAttribute() = %q, want linguist-generated", got)
	}
}

func TestGitHub_CIContext(t *testing.T) {
	env := map[string]string{
		"VERDI_GITHUB_DEFAULT_BRANCH": "main",
		"GITHUB_EVENT_NAME":           "pull_request",
		"GITHUB_BASE_REF":             "main",
	}
	a := New(Config{Owner: "acme", Repo: "svcfix", Getenv: func(k string) string { return env[k] }})

	info, err := a.CIContext(context.Background())
	if err != nil {
		t.Fatalf("CIContext: %v", err)
	}
	if info.DefaultBranch != "main" || !info.IsMergeRequest || info.TargetBranch != "main" {
		t.Errorf("CIContext = %+v", info)
	}
}

func TestGitHub_CIContext_NotAPullRequest(t *testing.T) {
	env := map[string]string{"GITHUB_EVENT_NAME": "push"}
	a := New(Config{Owner: "acme", Repo: "svcfix", Getenv: func(k string) string { return env[k] }})

	info, err := a.CIContext(context.Background())
	if err != nil {
		t.Fatalf("CIContext: %v", err)
	}
	if info.IsMergeRequest {
		t.Error("IsMergeRequest = true, want false for a push event")
	}
}

func TestGitHub_CIContext_Negative_CancelledContext(t *testing.T) {
	a := New(Config{Owner: "acme", Repo: "svcfix"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := a.CIContext(ctx); err == nil {
		t.Fatal("CIContext with cancelled context: want error, got nil")
	}
}

func TestGitHub_FetchEvidenceBundle_Negative_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	a := New(Config{BaseURL: ts.URL, Owner: "acme", Repo: "svcfix", HTTPClient: ts.Client()})
	if _, err := a.FetchEvidenceBundle(context.Background(), "ref", "commit"); err == nil {
		t.Fatal("FetchEvidenceBundle against a 500 server: want error, got nil")
	}
}
