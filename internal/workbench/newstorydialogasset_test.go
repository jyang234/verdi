package workbench

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The New story dialog's own script (spec/new-story-dialog-v2; SI-369
// (9)): a new asset for the dialog's new behaviour (parent co-1:
// boardspec.js does not grow), loaded before F7's opener so the opener's
// prefilled criterion is seen.

// TestNewStoryDialogAsset_ServedWithinBudget: the dialog's script ships
// as its own JavaScript file, embedded, non-empty, within the 64 KiB
// every new asset is held to, and served at its one address, GET only.
func TestNewStoryDialogAsset_ServedWithinBudget(t *testing.T) {
	const name = "newstorydialog.js"
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
}

// TestBoardPage_LoadsTheNewStoryDialogBeforeTheOpener: the wall page loads
// the dialog's script once, after boardspec.js, which owns the dialog and
// its button, and before F7's wallnewstory.js, whose opener clicks that
// button and checks a box without an event (SI-369 (9)).
func TestBoardPage_LoadsTheNewStoryDialogBeforeTheOpener(t *testing.T) {
	root := newBoardFixture(t)
	body := getBoard(t, NewHandler(root), boardFixtureName).Body.String()
	const dialog = `<script src="/assets/newstorydialog.js"></script>`
	if n := strings.Count(body, dialog); n != 1 {
		t.Fatalf("the wall loads the dialog's script %d times, want exactly 1", n)
	}
	at := strings.Index(body, dialog)
	if owner := strings.Index(body, `<script src="/assets/boardspec.js"></script>`); owner < 0 || owner > at {
		t.Errorf("the dialog's script does not load after boardspec.js")
	}
	if opener := strings.Index(body, `<script src="/assets/wallnewstory.js"></script>`); opener < 0 || opener < at {
		t.Errorf("the dialog's script does not load before the opener wallnewstory.js")
	}
}
