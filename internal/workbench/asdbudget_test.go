package workbench

// SI-168's structural asset budget: the ASD workbench's route-scoped
// JavaScript asset must not exceed 64 KiB uncompressed. A deterministic
// merge gate, never a wall-clock threshold.

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestBoardSpecASDAssetBudget(t *testing.T) {
	const ceiling = 64 * 1024
	data, err := os.ReadFile("assets/boardspecasd.js")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > ceiling {
		t.Fatalf("assets/boardspecasd.js is %d bytes, over the %d-byte SI-168 ceiling", len(data), ceiling)
	}
	if len(data) == 0 {
		t.Fatal("assets/boardspecasd.js is empty")
	}
}

// TestTopBarAsset_ServedWithinBudget: the top bar's script ships as its own
// JavaScript asset (spec/workbench-redesign co-1: new behavior ships in new
// assets of at most 64 KiB each, boardspec.js does not grow), embedded,
// non-empty, within the 64 KiB ceiling, and served at /assets/topbar.js —
// the one address every workbench page can reference.
func TestTopBarAsset_ServedWithinBudget(t *testing.T) {
	const ceiling = 64 * 1024
	data, err := os.ReadFile("assets/topbar.js")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || len(data) > ceiling {
		t.Fatalf("assets/topbar.js is %d bytes; want 1..%d", len(data), ceiling)
	}
	h := NewHandler(t.TempDir())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/assets/topbar.js", nil))
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/javascript") {
		t.Fatalf("GET /assets/topbar.js = %d (%s), want 200 JavaScript", rec.Code, rec.Header().Get("Content-Type"))
	}
	if !bytes.Equal(rec.Body.Bytes(), data) {
		t.Fatal("/assets/topbar.js does not serve the embedded assets/topbar.js bytes")
	}
	post := httptest.NewRecorder()
	h.ServeHTTP(post, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/assets/topbar.js", nil))
	if post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /assets/topbar.js = %d, want 405", post.Code)
	}
}
