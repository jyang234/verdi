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
// pill's live region is a polite status region rendered once in the page,
// after the swapped region (co-2: a region swap must never re-create the
// live region, which would re-announce an unchanged selection; the visual
// pill itself is the asset's, drawn inside the canvas while something is
// selected, SI-358 (4)), and the page loads the selection asset after the
// board scripts it builds on.
func TestWallCanvas_PageCarriesStatusPillOutsideRegionAndAsset(t *testing.T) {
	root := newBoardFixture(t)
	h := NewHandler(root)
	body := getBoard(t, h, boardFixtureName).Body.String()
	const pill = `<div id="wall-status-live" class="wall-status-live" data-testid="wall-status-live" role="status" aria-live="polite"></div>`
	if strings.Count(body, pill) != 1 {
		t.Fatalf("page carries %d status pill hosts, want exactly 1:\n%s", strings.Count(body, pill), pill)
	}
	main := sliceBetween(body, `<main id="boardv2-region">`, `</main>`)
	if main == "" {
		t.Fatal("no board region in the page")
	}
	if strings.Contains(main, "wall-status-live") {
		t.Error("the live region sits inside the swapped region")
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
	// The region render alone never carries the live region: it belongs to
	// the page. It does carry the wall frame — the canvas, then the pill's
	// reserved row below it in normal flow (SI-358 (4)) — empty until the
	// asset names a selection in it.
	proj := &BoardProjection{Spec: "s", Mode: modeAuthoring, Cards: []cardView{{ID: "dc-1", Kind: "decision", Text: "x"}}}
	region := renderBoardRegion(proj, &boardGitState{}, testASDView())
	if strings.Contains(region, "wall-status-live") {
		t.Error("renderBoardRegion writes the live region")
	}
	if !strings.Contains(region, `<div class="board-layout"><div class="wall-frame" data-testid="wall-frame"><div id="board-canvas" `) {
		t.Error("the canvas is not the wall frame's first child")
	}
	if !strings.Contains(region, `</div><div class="wall-status-row" data-testid="wall-status-row"></div></div><aside class="board-side">`) {
		t.Error("the status row does not follow the canvas inside the frame, before the rail")
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
