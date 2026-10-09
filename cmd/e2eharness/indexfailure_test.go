package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/refindex"
)

// startIndexFailureThroughControl starts the index-failure fixture
// through the harness's own control mux — the registered GET
// /index-failure-fixture route, exactly as a Playwright file reaches it —
// and returns the fixture and its served base URL.
func startIndexFailureThroughControl(t *testing.T) (*indexFailureFixture, string) {
	t.Helper()
	neutralizeCIEnvForTest(t)
	ctrl := newControlServer(t.TempDir(), testModuleRoot, "")
	t.Cleanup(ctrl.indexFailure.stop)
	srv := httptest.NewServer(ctrl.handler())
	t.Cleanup(srv.Close)
	status, body := httpGetBody(t, srv.URL+"/index-failure-fixture")
	if status != http.StatusOK {
		t.Fatalf("GET /index-failure-fixture = %d: %s", status, body)
	}
	url := strings.TrimSpace(body)
	if !strings.HasPrefix(url, "http://127.0.0.1:") || !strings.HasSuffix(url, "/") {
		t.Fatalf("fixture URL = %q, want a loopback base URL", url)
	}
	return ctrl.indexFailure, url
}

// TestIndexFailureFixture_ServesTheDisclosedFailure (spec/index-v2 ac-6;
// SI-366 (15)): the fixture's REAL store, computed by the real
// refindex.ComputeIndex behind the production workbench handler, fails
// its index computation on the undecodable default-branch spec — and the
// served home page discloses that failure, naming the spec, while no
// status group and no entry renders: not the intact default-branch spec
// beside it, and not the design branch's draft.
func TestIndexFailureFixture_ServesTheDisclosedFailure(t *testing.T) {
	_, url := startIndexFailureThroughControl(t)
	status, page := httpGetBody(t, url)
	if status != http.StatusOK {
		t.Fatalf("GET %s = %d, want the home page still served", url, status)
	}
	if got := strings.Count(page, `<p class="notice dir-index-failed">`); got != 1 {
		t.Fatalf("index-failure notices = %d, want exactly 1:\n%s", got, page)
	}
	if !strings.Contains(page, indexFailureUndecodablePath) {
		t.Fatalf("the failure notice does not name %s:\n%s", indexFailureUndecodablePath, page)
	}
	for _, partial := range []string{`data-testid="dir-group-`, `data-testid="dir-entry-`, `data-testid="glance-group-`, `data-testid="glance-entry-`} {
		if strings.Contains(page, partial) {
			t.Fatalf("a failed index computation rendered a partial group or entry (%s):\n%s", partial, page)
		}
	}
}

// TestProvisionIndexFailureStore: the store is what the failure needs — a
// provable default branch (the bare origin's HEAD; without it the walk
// would contribute nothing and nothing would fail), the intact and the
// undecodable spec on it, and a design branch carrying a valid draft —
// and the real index computation over it fails naming the undecodable
// spec, never a partial index.
func TestProvisionIndexFailureStore(t *testing.T) {
	ctx := context.Background()
	root, err := provisionIndexFailureStore(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("provisionIndexFailureStore: %v", err)
	}
	git := refindex.NewGitRunner()
	def, err := git.DefaultBranch(ctx, root)
	if err != nil || def != "origin/main" {
		t.Fatalf("DefaultBranch = %q, %v; want origin/main (the bare origin's HEAD)", def, err)
	}
	onMain, err := git.ListTree(ctx, root, def, ".verdi/specs/active")
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}
	for _, want := range []string{indexFailureIntactPath, indexFailureUndecodablePath} {
		if !containsString(onMain, want) {
			t.Fatalf("%s at %s = %v, want %s", ".verdi/specs/active", def, onMain, want)
		}
	}
	drafts, err := git.LocalDesignBranches(ctx, root)
	if err != nil || len(drafts) != 1 || drafts[0] != "design/"+indexFailureDraftName {
		t.Fatalf("LocalDesignBranches = %v, %v; want [design/%s]", drafts, err, indexFailureDraftName)
	}
	entries, err := refindex.ComputeIndex(ctx, root, git, refindex.NewStateResolver())
	if err == nil || !strings.Contains(err.Error(), indexFailureUndecodablePath) {
		t.Fatalf("ComputeIndex error = %v, want one naming %s", err, indexFailureUndecodablePath)
	}
	if entries != nil {
		t.Fatalf("ComputeIndex = %+v alongside its error, want no partial index", entries)
	}
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// TestProvisionIndexFailureStore_CancelledContext: provisioning honours
// ctx (main.go's interrupt path) and refuses with an error.
func TestProvisionIndexFailureStore_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := provisionIndexFailureStore(ctx, t.TempDir()); err == nil {
		t.Fatal("provisionIndexFailureStore with a cancelled context: want an error, got nil")
	}
}

// TestIndexFailureFixture_Idempotent: repeated requests return the SAME
// URL, never a second instance.
func TestIndexFailureFixture_Idempotent(t *testing.T) {
	f := newIndexFailureFixture()
	t.Cleanup(f.stop)
	get := func() string {
		rec := httptest.NewRecorder()
		f.handler(rec, httptest.NewRequest(http.MethodGet, "/index-failure-fixture", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /index-failure-fixture = %d: %s", rec.Code, rec.Body.String())
		}
		return strings.TrimSpace(rec.Body.String())
	}
	if first, second := get(), get(); first != second {
		t.Fatalf("url changed across calls: %q then %q, want the same instance reused", first, second)
	}
}

// TestIndexFailureFixture_Handler_MethodNotAllowed is the route's
// negative path: only GET starts or reports the fixture.
func TestIndexFailureFixture_Handler_MethodNotAllowed(t *testing.T) {
	f := newIndexFailureFixture()
	t.Cleanup(f.stop)
	rec := httptest.NewRecorder()
	f.handler(rec, httptest.NewRequest(http.MethodPost, "/index-failure-fixture", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /index-failure-fixture = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if f.url != "" {
		t.Fatalf("a refused request started the fixture at %s", f.url)
	}
}

// TestIndexFailureFixture_StopClosesAndRemovesStore: stop() closes the
// isolated server and removes the fixture's temporary store, and is
// idempotent — safe when never started, safe twice.
func TestIndexFailureFixture_StopClosesAndRemovesStore(t *testing.T) {
	never := newIndexFailureFixture()
	never.stop()
	never.stop()

	f, url := startIndexFailureThroughControl(t)
	tmp := f.tmp
	if tmp == "" {
		t.Fatal("a started fixture recorded no temporary directory")
	}
	f.stop()
	f.stop()
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Errorf("fixture temp dir %s still exists after stop (stat err %v)", tmp, err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 2 * time.Second}
	if resp, err := client.Do(req); err == nil {
		_ = resp.Body.Close()
		t.Errorf("the isolated server still answers at %s after stop", url)
	}
}

// TestControlServer_OutageAndReset (SI-366 (16)): POST /outage flips the
// open-MR feed to 503, and POST /outage/reset restores the canned feed,
// so a later suite reads the forge as reachable again. Each route
// answers POST only.
func TestControlServer_OutageAndReset(t *testing.T) {
	c := newControlServer(t.TempDir(), t.TempDir(), "")
	h := c.handler()
	do := func(method, path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
		return rec
	}
	feed := func(wantCode int) {
		t.Helper()
		rec := do(http.MethodGet, "/openmrs")
		if rec.Code != wantCode {
			t.Fatalf("GET /openmrs = %d, want %d", rec.Code, wantCode)
		}
		if wantCode == http.StatusOK && rec.Body.String() != openMRFeedJSON {
			t.Fatalf("GET /openmrs body = %q, want the canned feed %q", rec.Body.String(), openMRFeedJSON)
		}
	}

	feed(http.StatusOK)
	if rec := do(http.MethodPost, "/outage"); rec.Code != http.StatusNoContent {
		t.Fatalf("POST /outage = %d, want 204", rec.Code)
	}
	feed(http.StatusServiceUnavailable)
	if rec := do(http.MethodPost, "/outage/reset"); rec.Code != http.StatusNoContent {
		t.Fatalf("POST /outage/reset = %d, want 204", rec.Code)
	}
	feed(http.StatusOK)
	if rec := do(http.MethodPost, "/outage/reset"); rec.Code != http.StatusNoContent {
		t.Fatalf("POST /outage/reset with no outage = %d, want 204 (idempotent)", rec.Code)
	}
	feed(http.StatusOK)

	for _, path := range []string{"/outage", "/outage/reset"} {
		if rec := do(http.MethodGet, path); rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("GET %s = %d, want 405", path, rec.Code)
		}
	}
	if rec := do(http.MethodPost, "/outage"); rec.Code != http.StatusNoContent {
		t.Fatalf("POST /outage = %d, want 204", rec.Code)
	}
	if rec := do(http.MethodGet, "/outage/reset"); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /outage/reset = %d, want 405", rec.Code)
	}
	feed(http.StatusServiceUnavailable) // a refused reset leaves the outage in place
}
