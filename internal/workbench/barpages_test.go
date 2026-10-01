package workbench

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// probedGet serves one GET through h with a bar probe on the request and
// returns the status and every bar-facts value the page put in its view
// data.
func probedGet(t *testing.T, h http.Handler, path string) (int, []barFacts) {
	t.Helper()
	var seen []barFacts
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	req = req.WithContext(withBarProbe(req.Context(), func(f barFacts) { seen = append(seen, f) }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, seen
}

// pageBarCase is one workbench page and the bar facts its view data must
// carry.
type pageBarCase struct {
	name, path string
	status     int
	title      string
	// spec is the spec the page is about ("" for a page not about one):
	// the spec builder's facts; every other page carries the branch-level
	// builder's.
	spec string
}

// assertPageBar serves tc and asserts its page carried exactly one set of
// bar facts: the page's title, the spec facts only on a page about one
// spec, and — the plumbing every handler needs — the serving checkout's
// own branch, proven.
func assertPageBar(t *testing.T, h http.Handler, root, branch string, tc pageBarCase) barFacts {
	t.Helper()
	status, seen := probedGet(t, h, tc.path)
	if status != tc.status {
		t.Fatalf("GET %s = %d, want %d", tc.path, status, tc.status)
	}
	if len(seen) != 1 {
		t.Fatalf("GET %s: the page's view data carried %d bar-facts values, want exactly 1", tc.path, len(seen))
	}
	got := seen[0]
	checkBarFacts(t, got)
	if got.Title != tc.title {
		t.Errorf("title = %q, want %q", got.Title, tc.title)
	}
	switch {
	case tc.spec == "" && got.Spec != nil:
		t.Errorf("a page not about one spec carries spec facts %+v", got.Spec)
	case tc.spec == "" && got.Posture.BaseDigest != nil:
		t.Errorf("a page not about one spec carries a base digest %v", got.Posture.BaseDigest)
	case tc.spec != "" && (got.Spec == nil || got.Spec.Name != tc.spec || got.Spec.Unproven != ""):
		t.Errorf("spec facts = %+v, want proven facts for %s", got.Spec, tc.spec)
	}
	if got.Posture.Checkout.Text != root || got.Posture.Branch != provenFact(branch) || got.Posture.AcceptedHead.Unproven != "" {
		t.Errorf("posture = %+v, want the serving checkout %s on %s, proven", got.Posture, root, branch)
	}
	return got
}

// TestEveryPageCarriesItsBarFacts (SI-323 (1), (2), (4)): every workbench
// page's view data carries the top bar's facts — from the spec builder on
// the wall and its Document page, from the branch-level builder on every
// other page, error and not-found pages included.
func TestEveryPageCarriesItsBarFacts(t *testing.T) {
	root := newBarFixture(t)
	h := NewHandlerWith(root, Deps{Design: testDesignBridge{}, ReadinessLoader: fixedSnapshotLoader{snap: readinessFixture()}})
	branch := "design/" + boardFixtureName
	for _, tc := range []pageBarCase{
		{name: "index", path: "/", status: http.StatusOK, title: "Workbench"},
		{name: "disclosures", path: "/disclosures", status: http.StatusOK, title: "Disclosures"},
		{name: "corpus /a/", path: "/a/adr/0001-outbox-events", status: http.StatusOK, title: "Outbox pattern for domain events (board fixture)"},
		{name: "readiness", path: "/readiness?spec=" + boardFixtureName, status: http.StatusOK, title: "Readiness"},
		{name: "readiness refusal (error page)", path: "/readiness?spec=Not_A_Name", status: http.StatusBadRequest, title: "Error"},
		{name: "matrix error page", path: "/matrix/no-such-story", status: http.StatusNotFound, title: "Error"},
		{name: "verdict error page", path: "/verdict/no-such-story", status: http.StatusNotFound, title: "Error"},
		{name: "spec import", path: "/design/import", status: http.StatusOK, title: "Import existing spec"},
		{name: "spec import record failure", path: "/design/import/record", status: http.StatusBadRequest, title: "Source record"},
		{name: "not found", path: "/no-such-page", status: http.StatusNotFound, title: "Not found"},
		{name: "a vanished design branch's board", path: "/b/design%2Fgone/board/spec/x", status: http.StatusNotFound, title: "Not found"},
		{name: "wall", path: "/board/spec/" + boardFixtureName, status: http.StatusOK, title: "Refi test flow", spec: boardFixtureName},
		{name: "Document page", path: "/board/spec/" + boardFixtureName + "/document", status: http.StatusOK, title: "Refi test flow", spec: boardFixtureName},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertPageBar(t, h, root, branch, tc)
		})
	}

	t.Run("the Document page states the wall's facts", func(t *testing.T) {
		_, wall := probedGet(t, h, "/board/spec/"+boardFixtureName)
		_, doc := probedGet(t, h, "/board/spec/"+boardFixtureName+"/document")
		if len(wall) != 1 || len(doc) != 1 || !reflect.DeepEqual(wall[0], doc[0]) {
			t.Fatalf("wall facts %+v\nDocument facts %+v", wall, doc)
		}
	})
}

// TestEveryPageCarriesItsBarFacts_OtherSurfaces covers the pages that need
// fixtures of their own: the diagram editor; the v0 board (SI-323 (4)),
// the matrix, and the verdict viewer over a store with derived records;
// a design branch's served wall; and a remote-only branch's sealed wall.
func TestEveryPageCarriesItsBarFacts_OtherSurfaces(t *testing.T) {
	t.Run("diagram editor", func(t *testing.T) {
		root, _ := newDiagramFixture(t)
		h := NewHandler(root)
		assertPageBar(t, h, root, "design/diagrams", pageBarCase{path: "/board/diagram/" + diagramFixtureName, status: http.StatusOK, title: "Target topology"})
	})

	t.Run("v0 board, matrix, and verdict viewer", func(t *testing.T) {
		repo := buildWorkbenchFixtureRepo(t)
		setDefaultBranchSymref(t, repo.Dir)
		h := NewHandler(repo.Dir)
		for _, tc := range []pageBarCase{
			{path: "/board/STORY-1482", status: http.StatusOK, title: "Board: STORY-1482"},
			{path: "/matrix/jira:LOAN-1482", status: http.StatusOK, title: "Advisory preview matrix: jira:LOAN-1482"},
			{path: "/verdict/spec/stale-decline", status: http.StatusOK, title: "Verdict viewer: spec/stale-decline"},
		} {
			assertPageBar(t, h, repo.Dir, "main", tc)
		}
	})

	root := newBranchBoardFixture(t)
	h := NewHandlerWith(root, Deps{Design: testDesignBridge{}})

	t.Run("a design branch's served wall", func(t *testing.T) {
		_, seen := probedGet(t, h, "/b/design%2Fdraft-a/board/spec/draft-a")
		if len(seen) != 1 || seen[0].Spec == nil || seen[0].Spec.Name != "draft-a" || seen[0].Posture.Branch != provenFact("design/draft-a") {
			t.Fatalf("facts = %+v, want the draft-a wall's own branch", seen)
		}
		checkBarFacts(t, seen[0])
	})

	t.Run("a remote-only branch's sealed wall", func(t *testing.T) {
		status, seen := probedGet(t, h, "/b/design%2Fremote-only/board/spec/remote-spec")
		if status != http.StatusOK || len(seen) != 1 || seen[0].Spec == nil || seen[0].Spec.Name != "remote-spec" {
			t.Fatalf("GET sealed wall = %d, facts %+v", status, seen)
		}
		checkBarFacts(t, seen[0])
		if seen[0].Posture.AcceptedHead.Unproven == "" || seen[0].Posture.WorktreeHead.Unproven == "" {
			t.Fatalf("sealed posture = %+v, want its heads disclosed-unproven", seen[0].Posture)
		}
	})
}

// TestErrorPage_BeforeTheStoreIsRead: an error page whose handler knows no
// store root discloses every Git fact as unproven, never omits them.
func TestErrorPage_BeforeTheStoreIsRead(t *testing.T) {
	var seen []barFacts
	ctx := withBarProbe(t.Context(), func(f barFacts) { seen = append(seen, f) })
	rec := httptest.NewRecorder()
	renderError(ctx, rec, "", http.StatusInternalServerError, errors.New("the store could not be opened"))
	if rec.Code != http.StatusInternalServerError || len(seen) != 1 {
		t.Fatalf("status %d, facts %+v", rec.Code, seen)
	}
	checkBarFacts(t, seen[0])
	if seen[0].Title != "Error" || seen[0].Posture.Checkout.Unproven == "" || seen[0].Posture.Branch.Unproven == "" {
		t.Fatalf("facts = %+v, want the title and every Git fact disclosed-unproven", seen[0])
	}
}
