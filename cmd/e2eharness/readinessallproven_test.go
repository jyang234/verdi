package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestReadinessAllProvenSnapshot_Valid pins the fixture snapshot to the
// real readinesspilot contract: an invalid snapshot would make the
// browser's all-proven posture claims untrustworthy.
func TestReadinessAllProvenSnapshot_Valid(t *testing.T) {
	snap := readinessAllProvenSnapshot()
	if err := snap.Validate(); err != nil {
		t.Fatalf("all-proven fixture snapshot violates the readiness contract: %v", err)
	}
	if snap.CurrentFocus != "" || len(snap.Attention) != 0 {
		t.Fatalf("fixture is not all-proven: focus=%q attention=%d", snap.CurrentFocus, len(snap.Attention))
	}
	for _, concern := range snap.AllConcerns {
		if string(concern.State) != "proven" {
			t.Fatalf("fixture concern %q is %q, want proven", concern.ID, concern.State)
		}
	}
}

// TestReadinessAllProvenFixture_Handler_Happy proves the isolated server:
// GET returns a loopback URL whose /readiness page is the REAL workbench
// render of the strictly valid all-proven snapshot.
func TestReadinessAllProvenFixture_Handler_Happy(t *testing.T) {
	f := newReadinessAllProvenFixture()

	req := httptest.NewRequest(http.MethodGet, "/readiness-all-proven-fixture", nil)
	rec := httptest.NewRecorder()
	f.handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	url := strings.TrimSpace(rec.Body.String())
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Fatalf("url = %q, want a loopback URL", url)
	}

	resp, err := http.Get(url + "readiness")
	if err != nil {
		t.Fatalf("GET %sreadiness: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %sreadiness status = %d, want 200", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	page := string(body)
	for _, want := range []string{
		"All four steps are complete.",
		"Nothing needs attention: every check in this snapshot is proven.",
		"readiness-completed",
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("all-proven render is missing %q; got: %s", want, page)
		}
	}
	if strings.Contains(page, "aria-current") {
		t.Fatalf("all-proven render invents a current focus: %s", page)
	}
}

// TestReadinessAllProvenFixture_Handler_Reuse proves start-once reuse:
// a second GET returns the identical URL, not a second server.
func TestReadinessAllProvenFixture_Handler_Reuse(t *testing.T) {
	f := newReadinessAllProvenFixture()
	urls := make([]string, 2)
	for i := range urls {
		req := httptest.NewRequest(http.MethodGet, "/readiness-all-proven-fixture", nil)
		rec := httptest.NewRecorder()
		f.handler(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("call %d status = %d, want 200", i, rec.Code)
		}
		urls[i] = strings.TrimSpace(rec.Body.String())
	}
	if urls[0] != urls[1] {
		t.Fatalf("fixture URL changed across calls: %q then %q", urls[0], urls[1])
	}
}

// TestReadinessAllProvenFixture_Handler_Negative: wrong methods are 405.
func TestReadinessAllProvenFixture_Handler_Negative(t *testing.T) {
	f := newReadinessAllProvenFixture()
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/readiness-all-proven-fixture", nil)
		rec := httptest.NewRecorder()
		f.handler(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s status = %d, want 405", method, rec.Code)
		}
	}
}

// TestReadinessAllProvenFixture_ControlWiring proves the control server
// actually routes the endpoint.
func TestReadinessAllProvenFixture_ControlWiring(t *testing.T) {
	ctrl := newControlServer(t.TempDir(), t.TempDir())
	srv := httptest.NewServer(ctrl.handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/readiness-all-proven-fixture")
	if err != nil {
		t.Fatalf("GET control endpoint: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("control endpoint status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(body)), "http://127.0.0.1:") {
		t.Fatalf("control endpoint body = %q, want a loopback URL", body)
	}
}
