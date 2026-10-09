package workbench

// SI-168's structural asset budget: the ASD workbench's route-scoped
// JavaScript asset must not exceed 64 KiB uncompressed. A deterministic
// merge gate, never a wall-clock threshold.

import (
	"bytes"
	"fmt"
	"io/fs"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
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

// TestWorkbenchAssets_EveryAssetWithinItsCeiling is spec/wall-canvas-v2
// co-1's structural gate, written generically so a new asset is covered
// the moment the server embeds it, with no edit here: it walks the
// embedded FS the server serves (embeddedAssets), not the directory on
// disk, so nothing served escapes it. Every asset is at most 64 KiB
// (SI-168; parent co-1), and boardspec.js does not grow past its size
// when the story started (122 974 B). An asset that predates the ceiling
// and exceeds it is held to its own recorded ceiling
// (grandfatheredAssetCeilings), never waved through.
func TestWorkbenchAssets_EveryAssetWithinItsCeiling(t *testing.T) {
	assets, err := workbenchAssets(embeddedAssets)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, a := range assets {
		seen[a.name] = true
		if err := assetBudgetError(a.name, len(a.data)); err != nil {
			t.Error(err)
		}
	}
	for name := range grandfatheredAssetCeilings() {
		if !seen[name] {
			t.Errorf("grandfathered asset %s is not under assets/: drop its entry rather than keep a dead exception", name)
		}
	}
	if !seen["boardspec.js"] {
		t.Fatal("assets/boardspec.js was not enumerated: the ratchet would be vacuous")
	}
}

// TestAssetBudgetError pins the ceilings' arithmetic at their edges: an
// asset exactly at its ceiling passes, one byte more is refused, and a new
// asset never borrows a grandfathered ceiling.
func TestAssetBudgetError(t *testing.T) {
	for _, tc := range []struct {
		name string
		size int
		ok   bool
	}{
		{"boardspec.js", 122974, true},
		{"boardspec.js", 122975, false}, // the ratchet allows no growth
		{"boardspec.js", 1, true},
		{"wallselect.js", 64 * 1024, true},
		{"wallselect.js", 64*1024 + 1, false},
		{"newstorydialog.js", 64 * 1024, true}, // the New story dialog's script (SI-369 (9))
		{"newstorydialog.js", 64*1024 + 1, false},
		{"specimport.js", 96 * 1024, true},
		{"specimport.js", 96*1024 + 1, false},
		{"wallselect.js", 0, false},
		{"sub/boardspec.js", 64*1024 + 1, false}, // a ceiling is keyed by path, not base name
	} {
		err := assetBudgetError(tc.name, tc.size)
		if (err == nil) != tc.ok {
			t.Errorf("assetBudgetError(%q, %d) = %v, want ok=%v", tc.name, tc.size, err, tc.ok)
		}
	}
}

// TestWorkbenchAssets_EnumeratesEveryFile: the enumeration every
// generic asset guard reads returns every file under assets/ in the FS it
// is handed, nested ones included, in path order and named by their path
// under assets/ — so no guard can skip an asset added later. A file that
// FS holds outside assets/ is refused rather than skipped: the guards
// walk the embedded FS the server serves, so an embed pattern naming a
// file elsewhere would otherwise ship it unguarded. An unreadable root
// fails loudly.
func TestWorkbenchAssets_EnumeratesEveryFile(t *testing.T) {
	fsys := fstest.MapFS{
		"assets/boardspec.js":       {Data: []byte("a")},
		"assets/wallselect.js":      {Data: []byte("bb")},
		"assets/wall/minimap.js":    {Data: []byte("ccc")},
		"assets/wall/empty-dir/.gk": {Data: []byte("d")},
	}
	assets, err := workbenchAssets(fsys)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, a := range assets {
		got = append(got, a.name+"="+string(a.data))
	}
	want := []string{"boardspec.js=a", "wall/empty-dir/.gk=d", "wall/minimap.js=ccc", "wallselect.js=bb"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("workbenchAssets = %v, want %v", got, want)
	}
	for _, stray := range []string{"stray.js", "static/stray.js", "assetsx/stray.js"} {
		withStray := maps.Clone(fsys)
		withStray[stray] = &fstest.MapFile{Data: []byte("e")}
		if _, err := workbenchAssets(withStray); err == nil || !strings.Contains(err.Error(), stray) {
			t.Errorf("workbenchAssets with %s embedded outside assets/ = %v, want an error naming it", stray, err)
		}
	}
	if _, err := workbenchAssets(os.DirFS(filepath.Join(t.TempDir(), "absent"))); err == nil {
		t.Error("workbenchAssets over a missing root succeeded, want an error")
	}
}

// TestWorkbenchAssets_WalksTheServedFS: the guards' enumeration of the
// embedded FS names every asset the server serves, the two grandfathered
// ones included, and each carries the bytes the embed holds.
func TestWorkbenchAssets_WalksTheServedFS(t *testing.T) {
	assets, err := workbenchAssets(embeddedAssets)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, a := range assets {
		names[a.name] = true
		served, err := embeddedAssets.ReadFile("assets/" + a.name)
		if err != nil || !bytes.Equal(served, a.data) {
			t.Errorf("assets/%s: enumerated bytes differ from the embedded bytes (%v)", a.name, err)
		}
	}
	for _, name := range []string{"boardspec.js", "boardspecasd.js", "specimport.js", "topbar.js"} {
		if !names[name] {
			t.Errorf("the enumeration of the served FS lacks %s", name)
		}
	}
}

// TestGrandfatheredAssetCeilings_ExactlySI353 pins the closed exception
// list to SI-353's two entries and their ceilings, byte for byte: an extra
// entry, a dropped one, or a raised ceiling fails here, so changing the
// list is a ledger change first.
func TestGrandfatheredAssetCeilings_ExactlySI353(t *testing.T) {
	want := map[string]int{
		"boardspec.js":  122974,
		"specimport.js": 98304, // 96 KiB
	}
	if got := grandfatheredAssetCeilings(); !maps.Equal(got, want) {
		t.Errorf("grandfatheredAssetCeilings = %v, want exactly SI-353's %v", got, want)
	}
}

// assetCeiling is the per-asset byte ceiling: no JavaScript asset above
// 64 KiB uncompressed (Wave 6 §5.3, SI-168; spec/workbench-redesign co-1,
// spec/wall-canvas-v2 co-1).
const assetCeiling = 64 * 1024

// boardSpecJSCeiling is boardspec.js's size when spec/wall-canvas-v2 was
// cut (393639fb): "boardspec.js does not grow" (co-1, parent co-1).
const boardSpecJSCeiling = 122974

// grandfatheredAssetCeilings holds the assets that predate the 64 KiB
// ceiling and exceed it, each to its own recorded ceiling, keyed by path
// under assets/: boardspec.js to the no-growth ratchet, and specimport.js
// to the 96 KiB structural ceiling its own story set
// (TestSpecImport_HomeAndPageDiscoverable). Closed: a new asset is never
// added here — it meets assetCeiling.
func grandfatheredAssetCeilings() map[string]int {
	return map[string]int{
		"boardspec.js":  boardSpecJSCeiling,
		"specimport.js": 96 * 1024,
	}
}

// assetBudgetError reports whether an asset of size bytes at name (its
// path under assets/) is within its ceiling: non-empty, and at most its
// grandfathered ceiling or else assetCeiling.
func assetBudgetError(name string, size int) error {
	ceiling := assetCeiling
	if c, ok := grandfatheredAssetCeilings()[name]; ok {
		ceiling = c
	}
	if size == 0 {
		return fmt.Errorf("assets/%s is empty", name)
	}
	if size > ceiling {
		return fmt.Errorf("assets/%s is %d bytes, over its %d-byte ceiling", name, size, ceiling)
	}
	return nil
}

// workbenchAsset is one file the workbench serves from assets/.
type workbenchAsset struct {
	name string // slash path under assets/
	data []byte
}

// assetsDir is the directory, relative to the package (and so to the
// embedded FS's root), every served asset lives in.
const assetsDir = "assets"

// workbenchAssets reads every regular file in fsys, nested ones included,
// in path order, naming each by its path under assets/: the one
// enumeration every generic asset guard in this package reads, so an
// asset added later is covered without editing any guard. The guards
// hand it embeddedAssets, the FS the server serves, rooted at the package
// directory; a file there outside assets/ (an embed pattern may name any
// file in the package) is refused, never skipped, so nothing the server
// embeds escapes the guards.
func workbenchAssets(fsys fs.FS) ([]workbenchAsset, error) {
	var out []workbenchAsset
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		name, ok := strings.CutPrefix(path, assetsDir+"/")
		if !ok {
			return fmt.Errorf("%s is embedded outside %s/, where no asset guard reads it", path, assetsDir)
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		out = append(out, workbenchAsset{name: name, data: data})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("enumerating workbench assets: %w", err)
	}
	return out, nil
}

// TestIndexAsset_ServedWithinBudget: the index's filter script ships as
// its own JavaScript asset (spec/workbench-redesign co-1; spec/index-v2
// ac-3), embedded, non-empty, within the 64 KiB ceiling, and served at
// /assets/index.js with the method guard every asset carries.
func TestIndexAsset_ServedWithinBudget(t *testing.T) {
	const ceiling = 64 * 1024
	data, err := os.ReadFile("assets/index.js")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || len(data) > ceiling {
		t.Fatalf("assets/index.js is %d bytes; want 1..%d", len(data), ceiling)
	}
	h := NewHandler(t.TempDir())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/assets/index.js", nil))
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/javascript") {
		t.Fatalf("GET /assets/index.js = %d (%s), want 200 JavaScript", rec.Code, rec.Header().Get("Content-Type"))
	}
	if !bytes.Equal(rec.Body.Bytes(), data) {
		t.Fatal("/assets/index.js does not serve the embedded assets/index.js bytes")
	}
	post := httptest.NewRecorder()
	h.ServeHTTP(post, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/assets/index.js", nil))
	if post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /assets/index.js = %d, want 405", post.Code)
	}
}
