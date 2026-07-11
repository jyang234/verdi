package workbench

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthHandler_Happy(t *testing.T) {
	h := NewHandler("/store/root")
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "ok" {
		t.Fatalf("body = %q, want \"ok\"", rec.Body.String())
	}
}

func TestHealthHandler_Negative(t *testing.T) {
	h := NewHandler("/store/root")
	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 for POST /healthz", rec.Code)
	}
}

func TestIndexHandler_Happy(t *testing.T) {
	h := NewHandler("/store/root")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "/store/root") {
		t.Fatalf("index page does not mention the store root: %s", rec.Body.String())
	}
}

func TestIndexHandler_Negative(t *testing.T) {
	h := NewHandler("/store/root")

	t.Run("unknown path 404s", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/nonexistent-page", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("wrong method on / is rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want 405 for POST /", rec.Code)
		}
	})
}
