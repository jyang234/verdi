package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestVocabFixture_Handler_Happy proves the vocab-rename fixture serves a
// REAL workbench over a REAL store carrying the vocab-rename model.yaml
// (spec/vocabulary-surfaces ac-2, the harness half): the home page's
// status chip for the accepted probe spec reads the renamed label with
// the bare id kept in the chip's CSS class — flowed through the true
// pipeline (workbench.NewHandler -> store.Open -> Config.Model ->
// refindex.ComputeIndex), never a canned label.
func TestVocabFixture_Handler_Happy(t *testing.T) {
	f := newVocabFixture("../..")

	req := httptest.NewRequest(http.MethodGet, "/vocab-fixture", nil)
	rec := httptest.NewRecorder()
	f.handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	url := strings.TrimSpace(rec.Body.String())
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Fatalf("url = %q, want a loopback URL", url)
	}

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	page := string(body)

	if !strings.Contains(page, `data-testid="glance-entry-vocab-probe"`) {
		t.Fatalf("home page missing the vocab-probe glance entry (the bare-origin default-branch walk must list it); got: %s", page)
	}
	if !strings.Contains(page, `<span class="badge badge-accepted-pending-build">Ready to build</span>`) {
		t.Fatalf("home page missing the renamed status chip (id-bearing class, renamed text); got: %s", page)
	}
	if strings.Contains(page, `>accepted-pending-build<`) {
		t.Fatalf("home page still renders the bare state id as visible text; got: %s", page)
	}

	// The authoring half (judged-client-js-prose-has-no-browser-proof's
	// harness obligation): the draft spec's board serves in AUTHORING mode
	// (status draft + the checkout left on its design branch), and the
	// embedded client state carries the renamed class words boardspec.js's
	// STICKY_TYPES menu and proto-yarn dialog copy resolve. The words are
	// asserted here at the payload seam; the browser-executed proof lives
	// in e2e/tests/45-vocabulary.spec.ts.
	resp2, err := http.Get(url + "board/spec/vocab-draft")
	if err != nil {
		t.Fatalf("GET %sboard/spec/vocab-draft: %v", url, err)
	}
	defer func() { _ = resp2.Body.Close() }()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("GET board/spec/vocab-draft status = %d, want 200", resp2.StatusCode)
	}
	board, err := io.ReadAll(resp2.Body)
	if err != nil {
		t.Fatalf("reading board body: %v", err)
	}
	boardPage := string(board)
	if !strings.Contains(boardPage, `data-board-mode="authoring"`) {
		t.Fatalf("vocab-draft board is not in authoring mode (the sticky menu and proto-yarn gestures need it); got: %s", boardPage)
	}
	for _, want := range []string{`"story":"Workstream"`, `"spike":"Timebox"`} {
		if !strings.Contains(boardPage, want) {
			t.Fatalf("vocab-draft board's embedded state is missing the renamed class word %s; got: %s", want, boardPage)
		}
	}
}

// TestVocabFixture_Handler_Negative_WrongMethod mirrors the endpoint
// convention's negative path.
func TestVocabFixture_Handler_Negative_WrongMethod(t *testing.T) {
	f := newVocabFixture("../..")
	req := httptest.NewRequest(http.MethodPost, "/vocab-fixture", nil)
	rec := httptest.NewRecorder()
	f.handler(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

// TestControlServer_WiresVocabFixture proves the endpoint is mounted on
// the control server's own mux.
func TestControlServer_WiresVocabFixture(t *testing.T) {
	c := newControlServer(t.TempDir(), "../..")
	req := httptest.NewRequest(http.MethodGet, "/vocab-fixture", nil)
	rec := httptest.NewRecorder()
	c.handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.HasPrefix(strings.TrimSpace(rec.Body.String()), "http://127.0.0.1:") {
		t.Fatalf("body = %q, want a loopback URL", rec.Body.String())
	}
}
