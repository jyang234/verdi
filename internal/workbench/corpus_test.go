package workbench

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCorpusHandler_Happy(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)
	h := NewHandler(repo.Dir)

	req := httptest.NewRequest(http.MethodGet, "/a/spec/stale-decline", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Stale decline handling (fixture)") {
		t.Errorf("missing title, got: %s", body)
	}
	if !strings.Contains(body, "platform-team") {
		t.Errorf("missing owners in frontmatter card, got: %s", body)
	}
	if !strings.Contains(body, "jira:LOAN-1482") {
		t.Errorf("missing story in frontmatter card, got: %s", body)
	}
	if !strings.Contains(body, "Charge API calls are retried") {
		t.Errorf("missing rendered body, got: %s", body)
	}
	if !strings.Contains(body, `class="dispositions-table"`) {
		t.Errorf("missing I-5 dispositions table, got: %s", body)
	}
	if !strings.Contains(body, "incorporated") || !strings.Contains(body, "contradicted") || !strings.Contains(body, "open-question") {
		t.Errorf("dispositions table missing expected values, got: %s", body)
	}
	// Links panel: this spec's own `implements: adr/0002-outbox-events` link.
	if !strings.Contains(body, "implements") || !strings.Contains(body, "adr/0002-outbox-events") {
		t.Errorf("missing outgoing link, got: %s", body)
	}
}

// TestCorpusHandler_Backlink proves the backlinks half of the
// links/backlinks panel: adr/0002-outbox-events is `implements`-linked BY
// spec/stale-decline, so its own corpus page must show the computed
// inverse (implemented-by).
func TestCorpusHandler_Backlink(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)
	h := NewHandler(repo.Dir)

	req := httptest.NewRequest(http.MethodGet, "/a/adr/0002-outbox-events", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "implemented-by") {
		t.Errorf("missing computed backlink, got: %s", body)
	}
	if !strings.Contains(body, "spec/stale-decline") {
		t.Errorf("backlink does not name the source spec, got: %s", body)
	}
}

func TestCorpusHandler_Negative(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)
	h := NewHandler(repo.Dir)

	t.Run("unknown artifact 404s", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/a/spec/does-not-exist", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("wrong method rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/a/spec/stale-decline", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want 405", rec.Code)
		}
	})

	t.Run("bad store root", func(t *testing.T) {
		h := NewHandler(t.TempDir())
		req := httptest.NewRequest(http.MethodGet, "/a/spec/anything", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code < 400 {
			t.Fatalf("status = %d, want a 4xx/5xx for a root with no .verdi/ at all", rec.Code)
		}
	})
}
