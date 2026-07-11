package workbench

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatrixHandler_Happy(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)
	h := NewHandler(repo.Dir)

	req := httptest.NewRequest(http.MethodGet, "/matrix/jira:LOAN-1482", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "PREVIEW") || !strings.Contains(body, "ADVISORY") {
		t.Fatalf("missing the clearly-labeled PREVIEW/ADVISORY banner (03 §Evidence records), got: %s", body)
	}
	if !strings.Contains(body, "matrix-table") {
		t.Fatalf("missing matrix table, got: %s", body)
	}
	if !strings.Contains(body, "ac-1") {
		t.Fatalf("missing AC rows, got: %s", body)
	}
	if !strings.Contains(body, "story.violated") || !strings.Contains(body, "story.eligible") {
		t.Fatalf("missing story eligibility summary, got: %s", body)
	}
}

// TestMatrixHandler_StaleDerivedRendersLegibleError is DEFECT B's witness:
// a store whose derived/ records pin a commit unknown to this repo's git
// history. The fold's (correct, deliberate) loud ancestry failure must
// surface as a legible error page — the shared shell, the error text, and
// the stale-derived hint — while still carrying a non-2xx status (loud
// stays loud), not a bare 500 with the gitx error only in the server log.
func TestMatrixHandler_StaleDerivedRendersLegibleError(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)

	// Plant a derived snapshot directory whose commit SHA is a real hex
	// shape (so it is scanned) but is NOT in this repo's git history — the
	// exact "foreign derived record" LoadRecords raises its ancestry error
	// on.
	foreign := filepath.Join(repo.Dir, ".verdi", "data", "derived", "spec--stale-decline",
		"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatalf("planting foreign derived dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "verdicts.json"), []byte("[]\n"), 0o644); err != nil {
		t.Fatalf("writing foreign verdicts.json: %v", err)
	}

	h := NewHandler(repo.Dir)
	req := httptest.NewRequest(http.MethodGet, "/matrix/spec/stale-decline", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	// Loud stays loud: still a non-2xx (5xx operational) status.
	if rec.Code == http.StatusOK {
		t.Fatalf("expected a non-200 status (loud stays loud), got 200: %s", rec.Body.String())
	}
	if rec.Code < 500 {
		t.Fatalf("expected a 5xx status for a stale-derived operational failure, got %d", rec.Code)
	}

	body := rec.Body.String()
	// The stale-derived hint (the actionable half of DEFECT B).
	if !strings.Contains(body, "verdi sync --or-regen") {
		t.Fatalf("error page missing the stale-derived hint (`verdi sync --or-regen`); got: %s", body)
	}
	// Rendered on the shared shell, not a bare text/plain 500.
	if !strings.Contains(body, `<link rel="stylesheet" href="/assets/style.css">`) {
		t.Fatalf("error page not rendered on the shared workbench stylesheet; got: %s", body)
	}
	// The underlying gitx/ancestry error text is surfaced, not swallowed.
	if !strings.Contains(body, "ancestry") {
		t.Fatalf("error page does not surface the underlying ancestry error; got: %s", body)
	}
}

func TestMatrixHandler_Negative(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)
	h := NewHandler(repo.Dir)

	t.Run("unknown story", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/matrix/jira:NOPE-1", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/matrix/jira:LOAN-1482", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want 405", rec.Code)
		}
	})
}
