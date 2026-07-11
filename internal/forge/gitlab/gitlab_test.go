package gitlab

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/forge"
	"github.com/OWNER/verdi/internal/forge/forgetest"
)

// buildBundleZip zips the four bundle files under derived/<slug>/<commit>/,
// mirroring what verdi's own CI template would produce.
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

// fakeGitLabServer serves the three GitLab endpoints Adapter calls, keyed
// by (ref, commit) -> a seeded bundle. Missing seeds produce empty
// pipeline lists (404-equivalent: no successful pipeline).
type fakeGitLabServer struct {
	mu      map[string][]byte // commit -> zip bytes
	pipeIDs map[string]int64
	// mrsByTarget holds open MRs keyed by target branch.
	mrsByTarget map[string][]mergeRequestJSON
	// filesByRef holds seeded file content keyed by ref then path.
	filesByRef map[string]map[string][]byte
	nextMRIID  int64

	// discussions is keyed by mrID (IID, string-rendered) — the V1-P7
	// comment-round-trip extension.
	discussions map[string][]discussionJSON
	nextNoteID  int64
}

func newHarnessForTest(t *testing.T) *harness {
	t.Helper()
	srv := &fakeGitLabServer{
		mu:          map[string][]byte{},
		pipeIDs:     map[string]int64{},
		mrsByTarget: map[string][]mergeRequestJSON{},
		filesByRef:  map[string]map[string][]byte{},
		nextMRIID:   1,
		discussions: map[string][]discussionJSON{},
		nextNoteID:  1000,
	}
	nextPipeID := int64(1)

	mux := http.NewServeMux()
	mux.HandleFunc("/projects/42/merge_requests", func(w http.ResponseWriter, r *http.Request) {
		target := r.URL.Query().Get("target_branch")
		_ = json.NewEncoder(w).Encode(srv.mrsByTarget[target])
	})
	mux.HandleFunc("/projects/42/repository/files/", func(w http.ResponseWriter, r *http.Request) {
		// Path form: /projects/42/repository/files/<url-escaped-path>
		const prefix = "/projects/42/repository/files/"
		escapedPath := r.URL.Path[len(prefix):]
		path, err := url.PathUnescape(escapedPath)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		ref := r.URL.Query().Get("ref")
		content, ok := srv.filesByRef[ref][path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(repositoryFileJSON{
			Content:  base64.StdEncoding.EncodeToString(content),
			Encoding: "base64",
		})
	})
	mux.HandleFunc("/projects/42/pipelines", func(w http.ResponseWriter, r *http.Request) {
		sha := r.URL.Query().Get("sha")
		zipData, ok := srv.mu[sha]
		if !ok || zipData == nil {
			_ = json.NewEncoder(w).Encode([]pipeline{})
			return
		}
		id, ok := srv.pipeIDs[sha]
		if !ok {
			id = nextPipeID
			nextPipeID++
			srv.pipeIDs[sha] = id
		}
		_ = json.NewEncoder(w).Encode([]pipeline{{ID: id, Status: "success"}})
	})
	mux.HandleFunc("/projects/42/pipelines/", func(w http.ResponseWriter, r *http.Request) {
		// /projects/42/pipelines/{id}/jobs
		_ = json.NewEncoder(w).Encode([]job{{ID: 999, Name: defaultJobName}})
	})
	mux.HandleFunc("/projects/42/jobs/999/artifacts", func(w http.ResponseWriter, r *http.Request) {
		for _, zipData := range srv.mu {
			_, _ = w.Write(zipData)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	// /projects/42/merge_requests/{iid} (bare -> diff_refs) and
	// /projects/42/merge_requests/{iid}/discussions (list/post) — V1-P7's
	// comment-round-trip extension (DOC-DERIVED shapes — see gitlab.go's
	// package doc note).
	mux.HandleFunc("/projects/42/merge_requests/", func(w http.ResponseWriter, r *http.Request) {
		const prefix = "/projects/42/merge_requests/"
		rest := r.URL.Path[len(prefix):]
		parts := strings.SplitN(rest, "/", 2)
		mrID := parts[0]
		if len(parts) == 2 && parts[1] == "discussions" {
			switch r.Method {
			case http.MethodGet:
				_ = json.NewEncoder(w).Encode(srv.discussions[mrID])
			case http.MethodPost:
				var req createDiscussionRequest
				_ = json.NewDecoder(r.Body).Decode(&req)
				srv.nextNoteID++
				id := srv.nextNoteID
				n := noteJSON{ID: id, Body: req.Body, CreatedAt: "2026-07-11T18:00:00.000Z"}
				n.Author.Username = "fake-poster"
				d := discussionJSON{ID: fmt.Sprintf("disc-fake-%d", id)}
				if req.Position != nil {
					n.Resolvable = true
					line := req.Position.NewLine
					n.Position = &notePositionJSON{
						NewPath: req.Position.NewPath, NewLine: &line,
						BaseSHA: req.Position.BaseSHA, StartSHA: req.Position.StartSHA, HeadSHA: req.Position.HeadSHA,
						PositionType: req.Position.PositionType,
					}
				} else {
					d.IndividualNote = true
				}
				d.Notes = []noteJSON{n}
				srv.discussions[mrID] = append(srv.discussions[mrID], d)
				_ = json.NewEncoder(w).Encode(d)
			}
			return
		}
		// Bare "{iid}": PostComment's diff_refs pre-fetch.
		_ = json.NewEncoder(w).Encode(mrDiffRefsJSON{DiffRefs: struct {
			BaseSHA  string `json:"base_sha"`
			StartSHA string `json:"start_sha"`
			HeadSHA  string `json:"head_sha"`
		}{BaseSHA: "fake-base-sha", StartSHA: "fake-start-sha", HeadSHA: "fake-head-sha"}})
	})

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	adapter := New(Config{
		BaseURL:    ts.URL,
		ProjectID:  "42",
		Token:      "test-token",
		HTTPClient: ts.Client(),
	})
	return &harness{srv: srv, adapter: adapter}
}

type harness struct {
	srv     *fakeGitLabServer
	adapter *Adapter
}

func (h *harness) Forge() forge.Forge { return h.adapter }

func (h *harness) SeedBundle(t *testing.T, ref, commit string, bundle forge.EvidenceBundle) {
	t.Helper()
	h.srv.mu[commit] = buildBundleZip(t, bundle)
}

func (h *harness) WantGeneratedAttribute() string { return "gitlab-generated" }

func (h *harness) SeedOpenMR(t *testing.T, targetBranch, sourceBranch, title string) {
	t.Helper()
	h.srv.mrsByTarget[targetBranch] = append(h.srv.mrsByTarget[targetBranch], mergeRequestJSON{
		IID:          h.srv.nextMRIID,
		SourceBranch: sourceBranch,
		Title:        title,
	})
	h.srv.nextMRIID++
}

func (h *harness) SeedFile(t *testing.T, ref, path string, content []byte) {
	t.Helper()
	if h.srv.filesByRef[ref] == nil {
		h.srv.filesByRef[ref] = map[string][]byte{}
	}
	h.srv.filesByRef[ref][path] = content
}

// SeedComment arranges for ListComments(mrID) to already include c: a
// ThreadID-carrying comment becomes a resolvable DiffNote inside its own
// (or an existing) discussion; a ThreadID-less comment becomes its own
// individual_note:true discussion — mirroring how
// ListComments/GetThreadResolution themselves classify GitLab's two note
// kinds (gitlab.go's ListComments doc comment; DOC-DERIVED shapes).
func (h *harness) SeedComment(t *testing.T, mrID string, c forge.Comment) {
	t.Helper()
	h.srv.nextNoteID++
	id := h.srv.nextNoteID
	n := noteJSON{ID: id, Body: c.Body, CreatedAt: c.CreatedAt, Resolvable: c.ThreadID != ""}
	n.Author.Username = c.Author
	if c.Path != "" {
		line := c.Line
		n.Position = &notePositionJSON{NewPath: c.Path, NewLine: &line}
	}

	if c.ThreadID == "" {
		h.srv.discussions[mrID] = append(h.srv.discussions[mrID], discussionJSON{
			ID: fmt.Sprintf("disc-individual-%d", id), IndividualNote: true, Notes: []noteJSON{n},
		})
		return
	}
	for i := range h.srv.discussions[mrID] {
		if h.srv.discussions[mrID][i].ID == c.ThreadID {
			h.srv.discussions[mrID][i].Notes = append(h.srv.discussions[mrID][i].Notes, n)
			return
		}
	}
	h.srv.discussions[mrID] = append(h.srv.discussions[mrID], discussionJSON{ID: c.ThreadID, Notes: []noteJSON{n}})
}

// SeedThreadResolution sets threadID's resolution state on every note in
// its discussion (GitLab mirrors resolution across every note in a
// discussion per docs), creating the discussion if SeedComment has not
// already.
func (h *harness) SeedThreadResolution(t *testing.T, mrID string, tr forge.ThreadResolution) {
	t.Helper()
	var resolvedBy *struct {
		Username string `json:"username"`
	}
	if tr.ResolvedBy != "" {
		resolvedBy = &struct {
			Username string `json:"username"`
		}{Username: tr.ResolvedBy}
	}
	for i := range h.srv.discussions[mrID] {
		if h.srv.discussions[mrID][i].ID != tr.ThreadID {
			continue
		}
		for j := range h.srv.discussions[mrID][i].Notes {
			h.srv.discussions[mrID][i].Notes[j].Resolvable = true
			h.srv.discussions[mrID][i].Notes[j].Resolved = tr.Resolved
			h.srv.discussions[mrID][i].Notes[j].ResolvedBy = resolvedBy
		}
		return
	}
	h.srv.nextNoteID++
	n := noteJSON{ID: h.srv.nextNoteID, Resolvable: true, Resolved: tr.Resolved, ResolvedBy: resolvedBy}
	h.srv.discussions[mrID] = append(h.srv.discussions[mrID], discussionJSON{ID: tr.ThreadID, Notes: []noteJSON{n}})
}

// TestGitLab_ContractSuite proves the GitLab adapter satisfies the forge
// contract suite against an httptest double of GitLab's own API — no
// network (CLAUDE.md).
func TestGitLab_ContractSuite(t *testing.T) {
	forgetest.Run(t, func(t *testing.T) forgetest.Harness {
		return newHarnessForTest(t)
	})
}

func TestGitLab_FetchEvidenceBundle_NoBundleWrapsErrNoBundle(t *testing.T) {
	h := newHarnessForTest(t)
	_, err := h.adapter.FetchEvidenceBundle(context.Background(), "spec/x", "0000000000000000000000000000000000000000")
	if !errors.Is(err, forge.ErrNoBundle) {
		t.Fatalf("error = %v, want errors.Is(err, forge.ErrNoBundle)", err)
	}
}

func TestGitLab_GeneratedAttribute(t *testing.T) {
	a := New(Config{ProjectID: "1"})
	if got := a.GeneratedAttribute(); got != "gitlab-generated" {
		t.Errorf("GeneratedAttribute() = %q, want gitlab-generated", got)
	}
}

func TestGitLab_CIContext(t *testing.T) {
	env := map[string]string{
		"CI_DEFAULT_BRANCH":                   "main",
		"CI_MERGE_REQUEST_IID":                "42",
		"CI_MERGE_REQUEST_TARGET_BRANCH_NAME": "main",
	}
	a := New(Config{ProjectID: "1", Getenv: func(k string) string { return env[k] }})

	info, err := a.CIContext(context.Background())
	if err != nil {
		t.Fatalf("CIContext: %v", err)
	}
	if info.DefaultBranch != "main" || !info.IsMergeRequest || info.TargetBranch != "main" {
		t.Errorf("CIContext = %+v", info)
	}
}

func TestGitLab_CIContext_NotAnMR(t *testing.T) {
	env := map[string]string{"CI_DEFAULT_BRANCH": "main"}
	a := New(Config{ProjectID: "1", Getenv: func(k string) string { return env[k] }})

	info, err := a.CIContext(context.Background())
	if err != nil {
		t.Fatalf("CIContext: %v", err)
	}
	if info.IsMergeRequest {
		t.Error("IsMergeRequest = true, want false when CI_MERGE_REQUEST_IID is unset")
	}
}

func TestGitLab_CIContext_Negative_CancelledContext(t *testing.T) {
	a := New(Config{ProjectID: "1"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := a.CIContext(ctx); err == nil {
		t.Fatal("CIContext with cancelled context: want error, got nil")
	}
}

func TestGitLab_FetchEvidenceBundle_Negative_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	a := New(Config{BaseURL: ts.URL, ProjectID: "1", HTTPClient: ts.Client()})
	if _, err := a.FetchEvidenceBundle(context.Background(), "ref", "commit"); err == nil {
		t.Fatal("FetchEvidenceBundle against a 500 server: want error, got nil")
	}
}
