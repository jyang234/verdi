package workbench

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestIndexHandler_Home_RemoteOnlyDefaultBranch is the home page's own
// witness for the directory defect: a valid clone with origin/HEAD ->
// origin/main and refs/remotes/origin/main present, but NO local main
// (the serving checkout sits on a design branch). The whole-store
// directory (spec/directory-home ac-1) must render its default-branch
// specs from the genuine remote default revision — never disclose an
// index failure because a bare "main" fails to resolve.
func TestIndexHandler_Home_RemoteOnlyDefaultBranch(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)
	provisionHomeRefs(t, repo.Dir)
	// Leave main behind entirely: serve from the design branch with no
	// local default branch at all. origin/main (pushed by
	// provisionHomeRefs) is the only place the default tree exists.
	gitHome(t, repo.Dir, "checkout", "--quiet", "design/wip-draft")
	gitHome(t, repo.Dir, "branch", "-D", "main")

	h := NewHandler(repo.Dir)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "Could not compute the directory index") {
		t.Fatalf("home disclosed an index failure for a valid remote-only default branch; got: %s", body)
	}
	// A default-branch spec, sourced from the default branch, with its
	// corpus link — the tree was read from the remote-tracking revision.
	if !strings.Contains(body, `data-testid="dir-entry-stale-decline" data-source="default"`) {
		t.Fatalf("home missing the default-branch fixture spec sourced from the default branch; got: %s", body)
	}
	if !strings.Contains(body, `href="/a/spec/stale-decline"`) {
		t.Fatalf("home missing the fixture spec's corpus link; got: %s", body)
	}
	// The design-branch draft is still an unmerged draft entry.
	if !strings.Contains(body, `href="/b/design%2Fwip-draft/board/spec/wip-draft"`) {
		t.Fatalf("home missing the design-branch draft's /b/ board link; got: %s", body)
	}
}
