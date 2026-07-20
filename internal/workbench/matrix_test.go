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
// a store whose derived/ records carry a genuine fold-level failure must
// surface as a legible error page — the shared shell, the error text, and
// the stale-derived hint — while still carrying a non-2xx status (loud
// stays loud), not a bare 500 with the underlying error only in the server
// log.
//
// Trigger: a record whose evidence_for names an AC the spec does not
// declare — the fold's OWN deliberately loud "dangling binding" guard (03
// §Declarations: "a misspelled ac-3 must never surface as a silent
// no-signal"), the second class isStaleDerivedError recognizes alongside
// LoadRecords's ancestry probe. This test used to plant a derived
// directory keyed by a commit SHA unknown to this repo's git history (the
// "foreign derived record" shape) to trigger the ancestry probe's own
// error — spec/evidence-resilience ac-2 (X-15) removed exactly that
// trigger BY DESIGN: a commit-shaped derived directory that resolves to no
// real commit at all now reads as gracefully excluded (quarantined),
// never an operational error, so a deleted branch can never again brick a
// story's closure OR this page. "dangling binding" is the still-genuine
// operational-failure trigger left to prove DEFECT B's rendering — the
// rendering mechanism itself (isStaleDerivedError, renderError) is
// entirely unchanged by that fix.
func TestMatrixHandler_StaleDerivedRendersLegibleError(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)

	dir := filepath.Join(repo.Dir, ".verdi", "data", "derived", "spec--stale-decline", repo.Head)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("planting derived dir: %v", err)
	}
	record := `[{"schema":"verdi.evidence/v1","evidence_for":["ac-99"],"kind":"static","verdict":"pass",` +
		`"witness":"w","provenance":{"source":"ci","pipeline":"1","commit":"` + repo.Head + `"},` +
		`"digest":"sha256:` + strings.Repeat("ab", 32) + `"}]`
	if err := os.WriteFile(filepath.Join(dir, "verdicts.json"), []byte(record), 0o644); err != nil {
		t.Fatalf("writing verdicts.json with an unknown AC binding: %v", err)
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
	// The stale-derived hint (the actionable half of DEFECT B) — still
	// shown for this class too (isStaleDerivedError's own, pre-existing,
	// broader classification; unchanged by this story).
	if !strings.Contains(body, "verdi sync --or-regen") {
		t.Fatalf("error page missing the stale-derived hint (`verdi sync --or-regen`); got: %s", body)
	}
	// Rendered on the shared shell, not a bare text/plain 500.
	if !strings.Contains(body, `<link rel="stylesheet" href="/assets/style.css">`) {
		t.Fatalf("error page not rendered on the shared workbench stylesheet; got: %s", body)
	}
	// The underlying fold error text is surfaced, not swallowed.
	if !strings.Contains(body, "dangling binding") {
		t.Fatalf("error page does not surface the underlying fold error; got: %s", body)
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
