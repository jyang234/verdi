package workbench

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The wall's selection asset and the status pill's host (spec/wall-canvas-v2
// ac-2, co-1, co-2; lane F2a; ledger SI-350 (14)).

// TestWallCanvas_PageCarriesStatusPillOutsideRegionAndAsset: the status
// pill's host is a polite status region rendered once in the page, after
// the swapped region (co-2: a region swap must never re-create the live
// region, which would re-announce an unchanged selection), and the page
// loads the selection asset after the board scripts it builds on.
func TestWallCanvas_PageCarriesStatusPillOutsideRegionAndAsset(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)
	body := getBoard(t, h, boardFixtureName).Body.String()
	const pill = `<div id="wall-status" class="wall-status" data-testid="wall-status" role="status" aria-live="polite"></div>`
	if strings.Count(body, pill) != 1 {
		t.Fatalf("page carries %d status pill hosts, want exactly 1:\n%s", strings.Count(body, pill), pill)
	}
	main := sliceBetween(body, `<main id="boardv2-region">`, `</main>`)
	if main == "" {
		t.Fatal("no board region in the page")
	}
	if strings.Contains(main, "wall-status") {
		t.Error("the status pill host sits inside the swapped region")
	}
	if strings.Index(body, pill) < strings.Index(body, `</main>`) {
		t.Error("the status pill host precedes the region's end")
	}
	scripts := []string{`<script src="/assets/boardspec.js"></script>`, `<script src="/assets/boardspecasd.js"></script>`, `<script src="/assets/wallselect.js"></script>`}
	last := -1
	for _, s := range scripts {
		at := strings.Index(body, s)
		if at < 0 {
			t.Errorf("page lacks %s", s)
			continue
		}
		if at < last {
			t.Errorf("%s loads before the script it follows", s)
		}
		last = at
	}
	// The region render alone never carries the host: it belongs to the page.
	proj := &BoardProjection{Spec: "s", Mode: modeAuthoring, Cards: []cardView{{ID: "dc-1", Kind: "decision", Text: "x"}}}
	if region := renderBoardRegion(proj, &boardGitState{}, testASDView()); strings.Contains(region, "wall-status") {
		t.Error("renderBoardRegion writes the status pill host")
	}
}

// TestWallSelectAsset_ServedWithinBudget: the selection asset ships as its
// own JavaScript file (spec/wall-canvas-v2 co-1: new behaviour in new
// assets of at most 64 KiB; boardspec.js does not grow), embedded,
// non-empty, and served at /assets/wallselect.js, GET only.
func TestWallSelectAsset_ServedWithinBudget(t *testing.T) {
	data, err := embeddedAssets.ReadFile("assets/wallselect.js")
	if err != nil {
		t.Fatal(err)
	}
	if err := assetBudgetError("wallselect.js", len(data)); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(t.TempDir())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/assets/wallselect.js", nil))
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/javascript") {
		t.Fatalf("GET /assets/wallselect.js = %d (%s), want 200 JavaScript", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec.Body.String() != string(data) {
		t.Fatal("/assets/wallselect.js does not serve the embedded assets/wallselect.js bytes")
	}
	post := httptest.NewRecorder()
	h.ServeHTTP(post, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/assets/wallselect.js", nil))
	if post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /assets/wallselect.js = %d, want 405", post.Code)
	}
}
