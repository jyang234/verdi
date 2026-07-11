package workbench

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// TestIndexHandler_Home is DEFECT A's witness: the home page must be a real
// index, not a dead end — it lists a fixture spec, a board, and a service,
// every one a working href. It also follows one href (the spec page) to
// prove the link resolves.
func TestIndexHandler_Home(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)

	// Add a discoverable service so the Services section has real data
	// (testdata/corpus carries no .flowmap.yaml of its own).
	svcDir := filepath.Join(repo.Dir, "home-service")
	if err := os.MkdirAll(svcDir, 0o755); err != nil {
		t.Fatalf("creating service dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(svcDir, ".flowmap.yaml"), []byte("version: 1\nservice: home-service\n"), 0o644); err != nil {
		t.Fatalf("writing .flowmap.yaml: %v", err)
	}

	h := NewHandler(repo.Dir)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	// A fixture spec, linked to its corpus page.
	if !strings.Contains(body, `href="/a/spec/stale-decline"`) {
		t.Fatalf("home missing the fixture spec's corpus link; got: %s", body)
	}
	// stale-decline is a feature spec (story jira:LOAN-1482): it also links
	// its matrix and verdict pages via that scalar story ref.
	if !strings.Contains(body, `href="/matrix/jira:LOAN-1482"`) {
		t.Fatalf("home missing the feature spec's matrix link; got: %s", body)
	}
	if !strings.Contains(body, `href="/verdict/jira:LOAN-1482"`) {
		t.Fatalf("home missing the feature spec's verdict link; got: %s", body)
	}
	// The archived spec appears (separately from active).
	if !strings.Contains(body, `href="/a/spec/loan-refi-2023"`) {
		t.Fatalf("home missing the archived spec; got: %s", body)
	}
	// Other kinds grouped and linked (the corpus's ADRs).
	if !strings.Contains(body, `href="/a/adr/0002-outbox-events"`) {
		t.Fatalf("home missing an other-kind (adr) link; got: %s", body)
	}
	// The board, linked to its board page.
	if !strings.Contains(body, `href="/board/STORY-1482"`) {
		t.Fatalf("home missing the board link; got: %s", body)
	}
	// The discovered service, named.
	if !strings.Contains(body, "home-service") {
		t.Fatalf("home missing the discovered service; got: %s", body)
	}

	// Follow one href — the spec page — and confirm it resolves 200.
	req2 := httptest.NewRequest(http.MethodGet, "/a/spec/stale-decline", nil)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("following the spec href: status = %d, want 200; body=%s", rec2.Code, rec2.Body.String())
	}
}

// TestIndexHandler_HomeNoBoards proves the honest empty state: a store with
// no boards says so rather than rendering an empty list.
func TestIndexHandler_HomeNoBoards(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)
	// Remove the fixture's one board so the boards dir is empty.
	if err := os.RemoveAll(boardsDirForTest(repo.Dir)); err != nil {
		t.Fatalf("clearing boards: %v", err)
	}

	h := NewHandler(repo.Dir)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "No boards yet.") {
		t.Fatalf("home does not honestly report the empty boards state; got: %s", rec.Body.String())
	}
}

func boardsDirForTest(root string) string {
	return filepath.Join(root, ".verdi", "data", "mutable", "boards")
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
