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

func TestStyleCSSHandler_Happy(t *testing.T) {
	h := NewHandler("/store/root")
	req := httptest.NewRequest(http.MethodGet, "/assets/style.css", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Fatalf("Content-Type = %q, want text/css", ct)
	}
	body := rec.Body.String()
	// The composed stylesheet must carry both chroma palettes, so the
	// workbench's shared class-based code rendering is coloured and its dark
	// palette (github-dark, #e6edf3 foreground) lives inside the
	// prefers-color-scheme:dark block.
	if !strings.Contains(body, ".chroma-chroma") {
		t.Fatalf("workbench style.css missing the chroma palette; got:\n%s", body)
	}
	darkIdx := strings.Index(body, "@media (prefers-color-scheme: dark)")
	if darkIdx < 0 || !strings.Contains(body[darkIdx:], "#e6edf3") {
		t.Fatalf("workbench style.css missing the dark chroma palette in its dark media block; got:\n%s", body)
	}
}

func TestStyleCSSHandler_Negative(t *testing.T) {
	h := NewHandler("/store/root")
	req := httptest.NewRequest(http.MethodPost, "/assets/style.css", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 for POST /assets/style.css", rec.Code)
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
