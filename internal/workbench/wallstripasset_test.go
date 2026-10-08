package workbench

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The strip's and the drawer's assets (spec/wall-strip-and-drawer-v2;
// ledger SI-368; lanes F3a and F3b fill them).

// TestWallStripDrawerAssets_ServedWithinBudget: the strip and the drawer
// ship as their own JavaScript files (parent co-1: new behaviour in new
// assets of at most 64 KiB; boardspec.js does not grow), each embedded,
// non-empty, within its ceiling, and served at its one address, GET only.
func TestWallStripDrawerAssets_ServedWithinBudget(t *testing.T) {
	for _, name := range []string{"wallstrip.js", "walldrawer.js"} {
		t.Run(name, func(t *testing.T) {
			data, err := embeddedAssets.ReadFile("assets/" + name)
			if err != nil {
				t.Fatal(err)
			}
			if err := assetBudgetError(name, len(data)); err != nil {
				t.Fatal(err)
			}
			if _, grandfathered := grandfatheredAssetCeilings()[name]; grandfathered {
				t.Fatalf("%s borrows a grandfathered ceiling; a new asset meets the 64 KiB one", name)
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
		})
	}
}

// TestWallPage_LoadsTheStripAndDrawerAssets: the wall page loads the strip's
// and the drawer's scripts once each, after the wall's existing scripts they
// build on (the selection, the toolbar, the keys and the minimap).
func TestWallPage_LoadsTheStripAndDrawerAssets(t *testing.T) {
	root := newBoardFixture(t)
	body := getBoard(t, NewHandler(root), boardFixtureName).Body.String()
	last := -1
	for _, s := range []string{
		`<script src="/assets/wallminimap.js"></script>`,
		`<script src="/assets/wallstrip.js"></script>`,
		`<script src="/assets/walldrawer.js"></script>`,
	} {
		if n := strings.Count(body, s); n != 1 {
			t.Errorf("page loads %s %d times, want once", s, n)
			continue
		}
		at := strings.Index(body, s)
		if at < last {
			t.Errorf("%s loads before the script it follows", s)
		}
		last = at
	}
}
