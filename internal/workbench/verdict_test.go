package workbench

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestVerdictHandler_Diff_Happy(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)
	h := NewHandler(repo.Dir)

	q := url.Values{"a": {"7176513ece8b608ab0911000691bb697ee7e75ec"}, "b": {"93ddc5bbbb398cf747151e1c466afb83114398df"}}
	req := httptest.NewRequest(http.MethodGet, "/verdict/jira:LOAN-1482?"+q.Encode(), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "verdict-diff") {
		t.Fatalf("missing diff table, got: %s", body)
	}
	// ac-1 and ac-2 only have evidence in snapshot A; ac-3 only in B.
	if !strings.Contains(body, "ac-1") || !strings.Contains(body, "ac-2") || !strings.Contains(body, "ac-3") {
		t.Fatalf("missing expected AC rows, got: %s", body)
	}
	if !strings.Contains(body, "retryWorker") {
		t.Fatalf("missing snapshot A's witness text, got: %s", body)
	}
	if !strings.Contains(body, "abstain") {
		t.Fatalf("missing snapshot B's ac-3 abstain verdict, got: %s", body)
	}
	if !strings.Contains(body, "removed in B") || !strings.Contains(body, "added in B") {
		t.Fatalf("missing expected diff labels, got: %s", body)
	}
}

func TestVerdictHandler_Picker_Happy(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)
	h := NewHandler(repo.Dir)

	req := httptest.NewRequest(http.MethodGet, "/verdict/spec/stale-decline", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "7176513ece8b608ab0911000691bb697ee7e75ec") {
		t.Fatalf("picker missing first snapshot commit, got: %s", body)
	}
	if !strings.Contains(body, "93ddc5bbbb398cf747151e1c466afb83114398df") {
		t.Fatalf("picker missing second snapshot commit, got: %s", body)
	}
}

func TestVerdictHandler_Negative(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)
	h := NewHandler(repo.Dir)

	t.Run("unknown story", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/verdict/jira:NOPE-1", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/verdict/jira:LOAN-1482", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want 405", rec.Code)
		}
	})

	t.Run("unknown snapshot commit", func(t *testing.T) {
		q := url.Values{"a": {"0000000"}, "b": {"1111111"}}
		req := httptest.NewRequest(http.MethodGet, "/verdict/jira:LOAN-1482?"+q.Encode(), nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
	})
}
