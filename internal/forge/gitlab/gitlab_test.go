package gitlab

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
}

func newHarnessForTest(t *testing.T) *harness {
	t.Helper()
	srv := &fakeGitLabServer{mu: map[string][]byte{}, pipeIDs: map[string]int64{}}
	nextPipeID := int64(1)

	mux := http.NewServeMux()
	mux.HandleFunc("/projects/42/pipelines", func(w http.ResponseWriter, r *http.Request) {
		sha := r.URL.Query().Get("sha")
		zipData, ok := srv.mu[sha]
		if !ok || zipData == nil {
			json.NewEncoder(w).Encode([]pipeline{})
			return
		}
		id, ok := srv.pipeIDs[sha]
		if !ok {
			id = nextPipeID
			nextPipeID++
			srv.pipeIDs[sha] = id
		}
		json.NewEncoder(w).Encode([]pipeline{{ID: id, Status: "success"}})
	})
	mux.HandleFunc("/projects/42/pipelines/", func(w http.ResponseWriter, r *http.Request) {
		// /projects/42/pipelines/{id}/jobs
		json.NewEncoder(w).Encode([]job{{ID: 999, Name: defaultJobName}})
	})
	mux.HandleFunc("/projects/42/jobs/999/artifacts", func(w http.ResponseWriter, r *http.Request) {
		for _, zipData := range srv.mu {
			w.Write(zipData)
			return
		}
		w.WriteHeader(http.StatusNotFound)
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
