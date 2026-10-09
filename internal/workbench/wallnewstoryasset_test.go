package workbench

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The wall's opener for the index's call to action (spec/index-v2 ac-4;
// SI-366 (11); lane F7c).

// wallNewStoryCeiling is the opener's own byte ceiling: a small asset
// that reads two hooks and does one thing, so 2 KiB, well inside the
// 64 KiB every asset is held to (assetBudgetError).
const wallNewStoryCeiling = 2 * 1024

// TestWallNewStoryAsset_ServedWithinBudget: the opener ships as its own
// JavaScript file (co-1: new behaviour in a new asset; boardspec.js does
// not grow), embedded, non-empty, within its 2 KiB ceiling and the shared
// one, and served at its one address, GET only.
func TestWallNewStoryAsset_ServedWithinBudget(t *testing.T) {
	const name = "wallnewstory.js"
	data, err := embeddedAssets.ReadFile("assets/" + name)
	if err != nil {
		t.Fatal(err)
	}
	if err := assetBudgetError(name, len(data)); err != nil {
		t.Fatal(err)
	}
	if len(data) > wallNewStoryCeiling {
		t.Fatalf("assets/%s is %d bytes, over its %d-byte ceiling", name, len(data), wallNewStoryCeiling)
	}
	for _, hook := range []string{`"create-spec-btn"`, `[data-create-ac]`, `"new-story"`} {
		if !strings.Contains(string(data), hook) {
			t.Errorf("assets/%s does not read the contract's hook %s", name, hook)
		}
	}
	h := NewHandler(t.TempDir())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/assets/"+name, nil))
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/javascript") {
		t.Fatalf("GET /assets/%s = %d (%s), want 200 JavaScript", name, rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec.Body.String() != string(data) {
		t.Fatalf("/assets/%s does not serve the embedded assets/%s bytes", name, name)
	}
	post := httptest.NewRecorder()
	h.ServeHTTP(post, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/assets/"+name, nil))
	if post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /assets/%s = %d, want 405", name, post.Code)
	}
}

// TestBoardPage_LoadsTheNewStoryOpener: the wall page loads the opener
// once, after the wall's own scripts — the dialog and its button are
// boardspec.js's, so the opener runs only once their click handling is
// wired — and boardspec.js itself is unchanged in size (co-1).
func TestBoardPage_LoadsTheNewStoryOpener(t *testing.T) {
	root := newBoardFixture(t)
	body := getBoard(t, NewHandler(root), boardFixtureName).Body.String()
	const opener = `<script src="/assets/wallnewstory.js"></script>`
	if n := strings.Count(body, opener); n != 1 {
		t.Fatalf("the wall loads the opener %d times, want exactly 1", n)
	}
	for _, before := range []string{`<script src="/assets/boardspec.js"></script>`, `<script src="/assets/wallminimap.js"></script>`} {
		if strings.Index(body, before) > strings.Index(body, opener) {
			t.Errorf("the opener loads before %s", before)
		}
	}
	if data, err := embeddedAssets.ReadFile("assets/boardspec.js"); err != nil || len(data) > boardSpecJSCeiling {
		t.Fatalf("assets/boardspec.js is %d bytes (%v); it must not grow past %d", len(data), err, boardSpecJSCeiling)
	}
}
