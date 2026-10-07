package workbench

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The wall's keyboard and minimap assets and the minimap's host
// (spec/wall-canvas-v2 ac-6, co-1; lane F2c; ledger SI-358 (4)).

// TestWallKeysAssets_ServedWithinBudget: the keyboard and the minimap ship
// as their own JavaScript files (co-1: new behaviour in new assets of at
// most 64 KiB; boardspec.js does not grow), each embedded, non-empty,
// within its ceiling, and served at its one address, GET only.
func TestWallKeysAssets_ServedWithinBudget(t *testing.T) {
	for _, name := range []string{"wallkeys.js", "wallminimap.js"} {
		t.Run(name, func(t *testing.T) {
			data, err := embeddedAssets.ReadFile("assets/" + name)
			if err != nil {
				t.Fatal(err)
			}
			if err := assetBudgetError(name, len(data)); err != nil {
				t.Fatal(err)
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

// TestWallMinimap_HostSitsInTheRowHiddenFromAT: every mode's region renders
// the minimap's host once, at the status row's end after the toolbar's
// host (in the row it covers no paper, label or control, SI-358 (4)),
// empty until the asset fills it, hidden from assistive technology and
// never a tab stop — a pointer aid whose keyboard path is the arrows
// (Wave 6 §5.2) — and the page loads the keyboard and minimap assets after
// the selection and toolbar assets they build on.
func TestWallMinimap_HostSitsInTheRowHiddenFromAT(t *testing.T) {
	const host = `<div class="wall-minimap" data-testid="wall-minimap" aria-hidden="true"></div>`
	for _, mode := range []boardModeKind{modeAuthoring, modeReview, modeReadOnly} {
		region := renderBoardRegion(toolbarProjection(mode, ""), &boardGitState{}, testASDView())
		if n := strings.Count(region, host); n != 1 {
			t.Errorf("%s: the region renders the minimap host %d times, want exactly 1", mode, n)
		}
		row := sliceBetween(region, `<div class="wall-status-row"`, `</div></div></div><aside`)
		if row == "" {
			t.Fatalf("%s: no status row in the region", mode)
		}
		toolbar := strings.Index(row, `<div class="wall-toolbar"`)
		minimap := strings.Index(row, host)
		if toolbar < 0 || minimap < 0 || minimap < toolbar {
			t.Errorf("%s: the minimap host does not follow the toolbar's inside the row:\n%s", mode, row)
		}
		if strings.Contains(host, "tabindex") {
			t.Error("the minimap host is a tab stop")
		}
	}
	root := newBoardFixture(t)
	h := NewHandler(root)
	body := getBoard(t, h, boardFixtureName).Body.String()
	scripts := []string{
		`<script src="/assets/wallselect.js"></script>`,
		`<script src="/assets/walltoolbar.js"></script>`,
		`<script src="/assets/wallkeys.js"></script>`,
		`<script src="/assets/wallminimap.js"></script>`,
	}
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
}
