package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProvisionSpecImportStore_CleanMainWithProvableDefaultBranch proves
// the fixture store is exactly the importer's precondition: a clean
// checkout on main, a bare origin whose HEAD names main (synthetic
// default-branch proof), the data zone ignored, and NO policy, model
// override or forge/tracker configuration of any kind.
func TestProvisionSpecImportStore_CleanMainWithProvableDefaultBranch(t *testing.T) {
	ctx := context.Background()
	root, err := provisionSpecImportStore(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(root)) })

	if branch, _ := gitOutput(ctx, root, "rev-parse", "--abbrev-ref", "HEAD"); branch != "main" {
		t.Fatalf("branch = %q, want main", branch)
	}
	if porcelain, _ := gitOutput(ctx, root, "status", "--porcelain"); porcelain != "" {
		t.Fatalf("checkout dirty after provisioning: %q", porcelain)
	}
	if head, _ := gitOutput(ctx, root, "symbolic-ref", "refs/remotes/origin/HEAD"); head != "refs/remotes/origin/main" {
		t.Fatalf("origin/HEAD = %q, want refs/remotes/origin/main", head)
	}
	manifest, err := os.ReadFile(filepath.Join(root, ".verdi", "verdi.yaml"))
	if err != nil || string(manifest) != emptyStoreManifest {
		t.Fatalf("manifest = %q, %v", manifest, err)
	}
	for _, absent := range []string{"policy", "model.yaml", "specs"} {
		if _, err := os.Stat(filepath.Join(root, ".verdi", absent)); err == nil {
			t.Fatalf(".verdi/%s must not exist in the hermetic import store", absent)
		}
	}
	if strings.Contains(string(manifest), "forge") || strings.Contains(string(manifest), "tracker") {
		t.Fatalf("manifest configures a forge or tracker: %q", manifest)
	}
}

// TestSpecImportFixture_TamperTruncatesRecordOnBranchOnly proves the
// tamper endpoint rewrites ONLY the named branch's record.json as a
// descendant commit, leaving the serving checkout on a clean main.
func TestSpecImportFixture_TamperTruncatesRecordOnBranchOnly(t *testing.T) {
	ctx := context.Background()
	root, err := provisionSpecImportStore(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(root)) })
	f := newSpecImportFixture(testModuleRoot)
	f.root = root

	// A synthetic import layout on its own design branch, committed out of
	// band: the endpoint only needs SOME record.json under the imports dir.
	const slug = "tamper-probe"
	digest := strings.Repeat("ab", 32)
	if err := runGit(ctx, root, nil, "checkout", "--quiet", "-b", "design/"+slug, "main"); err != nil {
		t.Fatal(err)
	}
	recordDir := filepath.Join(root, ".verdi", "imports", slug, digest)
	if err := os.MkdirAll(recordDir, 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte(`{"schema":"verdi.spec-import-record/v1","preview_digest":"` + digest + `"}` + "\n")
	if err := os.WriteFile(filepath.Join(recordDir, "record.json"), original, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runGit(ctx, root, nil, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(ctx, root, nil, "commit", "--quiet", "--no-verify", "-m", "synthetic import record"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(ctx, root, nil, "checkout", "--quiet", "main"); err != nil {
		t.Fatal(err)
	}
	before, _ := gitOutput(ctx, root, "rev-parse", "refs/heads/design/"+slug)

	for _, tc := range []struct {
		name  string
		query string
		want  int
	}{
		{"wrong namespace", "?branch=main&spec=" + slug, http.StatusBadRequest},
		{"bad slug", "?branch=design/" + slug + "&spec=Bad%20Slug", http.StatusBadRequest},
		{"happy", "?branch=design/" + slug + "&spec=" + slug, http.StatusNoContent},
	} {
		req := httptest.NewRequest(http.MethodPost, "/spec-import-fixture/tamper"+tc.query, nil)
		rec := httptest.NewRecorder()
		f.tamperHandler(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("%s: status = %d, want %d; body=%s", tc.name, rec.Code, tc.want, rec.Body.String())
		}
	}
	getReq := httptest.NewRequest(http.MethodGet, "/spec-import-fixture/tamper?branch=design/"+slug+"&spec="+slug, nil)
	getRec := httptest.NewRecorder()
	f.tamperHandler(getRec, getReq)
	if getRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET tamper = %d, want 405", getRec.Code)
	}

	after, _ := gitOutput(ctx, root, "rev-parse", "refs/heads/design/"+slug)
	if after == before {
		t.Fatal("tamper did not advance the branch")
	}
	if parent, _ := gitOutput(ctx, root, "rev-parse", after+"^"); parent != before {
		t.Fatalf("tamper commit parent = %s, want the previous tip %s", parent, before)
	}
	tampered, err := gitOutput(ctx, root, "show", after+":.verdi/imports/"+slug+"/"+digest+"/record.json")
	if err != nil {
		t.Fatal(err)
	}
	if tampered != strings.TrimSpace(string(original[:10])) {
		t.Fatalf("tampered record = %q, want the first ten bytes of the original", tampered)
	}
	if branch, _ := gitOutput(ctx, root, "rev-parse", "--abbrev-ref", "HEAD"); branch != "main" {
		t.Fatalf("serving checkout moved to %q", branch)
	}
	if porcelain, _ := gitOutput(ctx, root, "status", "--porcelain"); porcelain != "" {
		t.Fatalf("serving checkout dirty after tamper: %q", porcelain)
	}
	if worktrees, _ := gitOutput(ctx, root, "worktree", "list", "--porcelain"); strings.Count(worktrees, "worktree ") != 1 {
		t.Fatalf("temporary worktree leaked: %s", worktrees)
	}
}

// TestSpecImportFixture_Handler_Negative_WrongMethod pins the endpoints'
// method guards without starting any subprocess.
func TestSpecImportFixture_Handler_Negative_WrongMethod(t *testing.T) {
	f := newSpecImportFixture(testModuleRoot)
	for path, h := range map[string]http.HandlerFunc{"/spec-import-fixture": f.handler, "/spec-import-fixture/info": f.infoHandler} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		rec := httptest.NewRecorder()
		h(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("POST %s = %d, want 405", path, rec.Code)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/spec-import-fixture/tamper?branch=design/x&spec=x", nil)
	rec := httptest.NewRecorder()
	f.tamperHandler(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("tamper before start = %d, want 409", rec.Code)
	}
}

// TestControlServer_WiresSpecImportFixture proves the three endpoints are
// mounted on the control server's own mux (no subprocess: the method
// guards answer first).
func TestControlServer_WiresSpecImportFixture(t *testing.T) {
	c := newControlServer(t.TempDir(), testModuleRoot)
	t.Cleanup(c.specImport.stop)
	for _, path := range []string{"/spec-import-fixture", "/spec-import-fixture/info"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		rec := httptest.NewRecorder()
		c.handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("POST %s = %d, want 405 (route mounted)", path, rec.Code)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/spec-import-fixture/tamper", nil)
	rec := httptest.NewRecorder()
	c.handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET tamper = %d, want 405 (route mounted)", rec.Code)
	}
}
