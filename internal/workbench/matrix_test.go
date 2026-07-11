package workbench

import (
	"net/http"
	"net/http/httptest"
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
