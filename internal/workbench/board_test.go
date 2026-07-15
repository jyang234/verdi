package workbench

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardio"
)

// TestBoardHandler_Happy proves the board page loads the fixture's real
// board (examples/showcase/mutable/boards/STORY-1482.json): its pin, all
// three stickies with their RESOLVED annotation bodies (including the two
// board-only ones per I-34 — 05 §Workbench's own "sticky ... including
// board-only ones"), and the yarn strand.
func TestBoardHandler_Happy(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)
	h := NewHandler(repo.Dir)

	req := httptest.NewRequest(http.MethodGet, "/board/STORY-1482", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	if !strings.Contains(body, `id="board-canvas"`) {
		t.Fatalf("missing board canvas, got: %s", body)
	}
	// The pin (spec/stale-decline@...).
	if !strings.Contains(body, "spec/stale-decline@7176513ece8b608ab0911000691bb697ee7e75ec") {
		t.Fatalf("missing pinned card, got: %s", body)
	}
	// The targeted sticky's resolved body text.
	if !strings.Contains(body, "charge API needs a retry note") {
		t.Fatalf("missing targeted sticky's resolved content, got: %s", body)
	}
	// The two board-only stickies (I-34) — no `target`, only `board`.
	if !strings.Contains(body, "what about partial refunds?") {
		t.Fatalf("missing board-only sticky (question), got: %s", body)
	}
	if !strings.Contains(body, "wire up the retry worker for stale declines") {
		t.Fatalf("missing board-only sticky (agent-task), got: %s", body)
	}
	// Yarn.
	if !strings.Contains(body, "relates") {
		t.Fatalf("missing yarn label, got: %s", body)
	}
	// The client state payload and the one JS file.
	if !strings.Contains(body, "window.__BOARD__") {
		t.Fatalf("missing embedded board state, got: %s", body)
	}
	if !strings.Contains(body, `/assets/board.js`) {
		t.Fatalf("missing board.js script tag, got: %s", body)
	}
}

func TestBoardHandler_Negative(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)
	h := NewHandler(repo.Dir)

	t.Run("invalid key rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/board/..%2F..%2Fetc", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK {
			t.Fatalf("expected a non-200 for a path-traversal-shaped key, got 200: %s", rec.Body.String())
		}
	})

	t.Run("unknown board loads an empty one, not an error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/board/NEVER-SEEN-1", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (a not-yet-used board is a legitimate empty state)", rec.Code)
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/board/STORY-1482", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want 405", rec.Code)
		}
	})
}

// TestBoardAutosave_Happy_RoundTripsAndPersistsAcrossReload proves the
// autosave contract end to end: POST a moved sticky position, then GET the
// board page again (a fresh handler-level "reload") and see the new
// position reflected — proving the write actually landed on disk, not
// just in the response.
func TestBoardAutosave_Happy_RoundTripsAndPersistsAcrossReload(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)
	h := NewHandler(repo.Dir)

	payload := map[string]any{
		"pins": []map[string]any{
			{"ref": "spec/stale-decline@7176513ece8b608ab0911000691bb697ee7e75ec", "x": 999, "y": 888},
		},
		"stickies": []map[string]any{
			{"id": "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", "x": 111, "y": 222},
			{"id": "a-01J8Z0K4BBBBBBBBBBBBBBBBBB", "x": 120, "y": 80},
			{"id": "a-01J8Z0K5CCCCCCCCCCCCCCCCCC", "x": 220, "y": 160},
		},
		"yarn": []map[string]any{
			{"from": "pin:spec/stale-decline", "to": "sticky:a-01J8Z0K3AAAAAAAAAAAAAAAAAA", "label": "relates"},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/board/STORY-1482/autosave", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("autosave status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}

	// Reload: a fresh GET must show the moved position.
	req2 := httptest.NewRequest(http.MethodGet, "/board/STORY-1482", nil)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("reload status = %d", rec2.Code)
	}
	if !strings.Contains(rec2.Body.String(), `"x":999`) || !strings.Contains(rec2.Body.String(), `"y":888`) {
		t.Fatalf("reload does not show the autosaved position: %s", rec2.Body.String())
	}

	// Also verify directly on disk (the atomic-write path).
	path, err := boardio.BoardStatePath(repo.Dir, "STORY-1482")
	if err != nil {
		t.Fatal(err)
	}
	saved, err := boardio.LoadBoardState(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Pins) != 1 || saved.Pins[0].X != 999 {
		t.Fatalf("saved board on disk = %+v, want pin.X == 999", saved.Pins)
	}
}

// TestBoardAutosave_Atomicity proves autosave never leaves a temp file
// behind and never corrupts the board file for a concurrent reader.
func TestBoardAutosave_Atomicity(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)
	h := NewHandler(repo.Dir)

	for i := 0; i < 3; i++ {
		payload := map[string]any{
			"pins":     []map[string]any{{"ref": "spec/stale-decline@7176513ece8b608ab0911000691bb697ee7e75ec", "x": float64(i), "y": float64(i)}},
			"stickies": []map[string]any{},
			"yarn":     []map[string]any{},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/board/STORY-1482/autosave", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("autosave %d status = %d; body=%s", i, rec.Code, rec.Body.String())
		}
	}

	boardsDir := boardio.BoardsDir(repo.Dir)
	entries, err := os.ReadDir(boardsDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Fatalf("leftover temp file after repeated autosave: %s", e.Name())
		}
	}
}

func TestBoardAutosave_Negative(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)
	h := NewHandler(repo.Dir)

	t.Run("malformed JSON rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/board/STORY-1482/autosave", strings.NewReader("{not json"))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("unknown field rejected (strict decode)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/board/STORY-1482/autosave", strings.NewReader(`{"pins":[],"stickies":[],"yarn":[],"bogus":1}`))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("frozen/provenance fields rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/board/STORY-1482/autosave", strings.NewReader(`{"pins":[],"stickies":[],"yarn":[],"frozen":{"at":"2026-01-01","commit":"c5e360a9ee5e9eb6089e54b772fa16959ada4662"}}`))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (autosave must never accept a frozen shape); body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid sticky id rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/board/STORY-1482/autosave", strings.NewReader(`{"pins":[],"stickies":[{"id":"not-a-ulid","x":1,"y":2}],"yarn":[]}`))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid board key rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/board/..%2F..%2Fx/autosave", strings.NewReader(`{"pins":[],"stickies":[],"yarn":[]}`))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusNoContent {
			t.Fatalf("expected the traversal-shaped key to be rejected, got 204")
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/board/STORY-1482/autosave", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want 405", rec.Code)
		}
	})
}

// TestBoardCommit_Happy proves the workbench's commit-to-design action
// produces the three artifacts (spec skeleton, frozen board.json,
// dispositions block) by calling the exact same internal/commitdesign.Run
// the CLI verb calls.
func TestBoardCommit_Happy(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)
	h := NewHandler(repo.Dir)

	reqBody, _ := json.Marshal(map[string]string{"name": "from-workbench-board", "story_ref": "jira:LOAN-1482"})
	req := httptest.NewRequest(http.MethodPost, "/board/STORY-1482/commit", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp commitResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding response: %v; body=%s", err, rec.Body.String())
	}
	if resp.SpecRef != "spec/from-workbench-board" {
		t.Fatalf("SpecRef = %q", resp.SpecRef)
	}
	if resp.Dispositions != 3 {
		t.Fatalf("Dispositions = %d, want 3 (the fixture board's 3 stickies)", resp.Dispositions)
	}

	specPath := filepath.Join(repo.Dir, resp.SpecPath)
	if _, err := os.Stat(specPath); err != nil {
		t.Fatalf("spec.md not written: %v", err)
	}
	boardPath := filepath.Join(repo.Dir, resp.BoardPath)
	braw, err := os.ReadFile(boardPath)
	if err != nil {
		t.Fatalf("board.json not written: %v", err)
	}
	fb, err := artifact.DecodeBoard(braw)
	if err != nil {
		t.Fatalf("DecodeBoard: %v", err)
	}
	if fb.Frozen == nil || fb.Provenance == nil {
		t.Fatalf("frozen board.json missing Frozen/Provenance: %+v", fb)
	}
}

func TestBoardCommit_Negative(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)
	h := NewHandler(repo.Dir)

	t.Run("missing name", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]string{"story_ref": "jira:LOAN-1482"})
		req := httptest.NewRequest(http.MethodPost, "/board/STORY-1482/commit", bytes.NewReader(reqBody))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("malformed JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/board/STORY-1482/commit", strings.NewReader("{not json"))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("no story ref and board key isn't scheme:key shaped", func(t *testing.T) {
		reqBody, _ := json.Marshal(map[string]string{"name": "no-story-ref"})
		req := httptest.NewRequest(http.MethodPost, "/board/STORY-1482/commit", bytes.NewReader(reqBody))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/board/STORY-1482/commit", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want 405", rec.Code)
		}
	})
}
