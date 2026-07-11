package github

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
	byCommit map[string][]byte
	runIDs   map[string]int64
}

type harness struct {
	srv     *fakeGitHubServer
	adapter *Adapter
}

func newHarnessForTest(t *testing.T) *harness {
	t.Helper()
	srv := &fakeGitHubServer{byCommit: map[string][]byte{}, runIDs: map[string]int64{}}
	nextRunID := int64(100)

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/svcfix/actions/runs", func(w http.ResponseWriter, r *http.Request) {
		sha := r.URL.Query().Get("head_sha")
		if _, ok := srv.byCommit[sha]; !ok {
			json.NewEncoder(w).Encode(runsResponse{})
			return
		}
		id, ok := srv.runIDs[sha]
		if !ok {
			id = nextRunID
			nextRunID++
			srv.runIDs[sha] = id
		}
		json.NewEncoder(w).Encode(runsResponse{WorkflowRuns: []run{{ID: id, Status: "completed", Conclusion: "success"}}})
	})
	mux.HandleFunc("/repos/acme/svcfix/actions/runs/", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(artifactsResponse{Artifacts: []artifact{{ID: 7, Name: defaultArtifactName}}})
	})
	mux.HandleFunc("/repos/acme/svcfix/actions/artifacts/7/zip", func(w http.ResponseWriter, r *http.Request) {
		for _, zipData := range srv.byCommit {
			w.Write(zipData)
			return
		}
		w.WriteHeader(http.StatusNotFound)
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
