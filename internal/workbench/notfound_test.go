package workbench

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func getPath(t *testing.T, home HomeDeps, path string) (int, string) {
	t.Helper()
	h := NewHandlerWithHome(t.TempDir(), Deps{}, home)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

// TestCatchAll_StaleBranch_DisclosedNotice is ac-3's deleted-mid-session
// shape: a /b/ board address whose branch no longer resolves renders a
// disclosed notice page — HTTP 404, a body naming the vanished branch, and
// a working link back to the directory. Never a bare NotFound.
//
// AMENDED by spec/draft-boards: the /b/ board grammar is now a REGISTERED
// route (handler.go), so this address resolves through branchboard.go's
// dispatch — which routes the no-ref case to the SAME renderStaleEntryNotice
// this file's catch-all uses — rather than falling through to "/". The
// surface and its assertions are unchanged; the fixture is now a real
// store (the branchboard fixture) because the resolution runs through the
// worktree-manager seam's real git probes, not HomeDeps' injected fake.
func TestCatchAll_StaleBranch_DisclosedNotice(t *testing.T) {
	root := newBranchBoardFixture(t)
	h := NewHandler(root)
	req := httptest.NewRequest(http.MethodGet, "/b/design%2Fgone/board/spec/gone", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	code, body := rec.Code, rec.Body.String()

	if code != http.StatusNotFound {
		t.Fatalf("status = %d, want the honest 404", code)
	}
	if !strings.Contains(body, "design/gone") {
		t.Fatalf("stale-entry page does not name what vanished; got: %s", body)
	}
	if !strings.Contains(body, `data-testid="stale-entry-notice"`) {
		t.Fatalf("stale-entry page missing its notice block; got: %s", body)
	}
	if !strings.Contains(body, `href="/" data-testid="back-to-directory"`) {
		t.Fatalf("stale-entry page missing the way back; got: %s", body)
	}
	if !strings.Contains(body, "<html") {
		t.Fatalf("stale-entry page is not a rendered page; got: %s", body)
	}
}

// TestCatchAll_LiveBranchAddress_LegibleNotFound: a /b/ address whose
// branch still resolves must NOT claim the branch vanished.
//
// AMENDED by spec/draft-boards: with the /b/ routes registered, a live
// local branch's address serves the branch's own board — so the
// missing-SPEC case is what remains 404 here, and it must answer exactly
// like the unprefixed board's missing-spec 404 (sub-routes "work
// identically beneath the prefix", draft-boards ac-1), never as a
// vanished-branch claim.
func TestCatchAll_LiveBranchAddress_LegibleNotFound(t *testing.T) {
	root := newBranchBoardFixture(t)
	h := NewHandler(root)
	req := httptest.NewRequest(http.MethodGet, "/b/design%2Ftwo-a/board/spec/no-such-spec", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	code, body := rec.Code, rec.Body.String()

	if code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", code)
	}
	if strings.Contains(body, `data-testid="stale-entry-notice"`) {
		t.Fatalf("a still-resolving branch must not be reported as vanished; got: %s", body)
	}
}

// TestCatchAll_GenericPath_LegibleNotFound upgrades the old bare-NotFound
// catch-all: any unserved path renders a legible 404 naming the path with
// a way back (dc-5's never-a-bare-NotFound posture).
func TestCatchAll_GenericPath_LegibleNotFound(t *testing.T) {
	home := HomeDeps{Git: fakeHomeGit{}}
	code, body := getPath(t, home, "/nonexistent-page")

	if code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", code)
	}
	if !strings.Contains(body, "/nonexistent-page") || !strings.Contains(body, `data-testid="back-to-directory"`) {
		t.Fatalf("generic 404 not legible; got: %s", body)
	}
}

// TestCatchAll_BranchProbeError_NeverClaimsVanished: if branch resolution
// itself fails, the page must not assert the branch is gone.
//
// AMENDED by spec/draft-boards: with the /b/ routes registered, the
// resolution runs through the worktree-manager seam, and an operational
// failure there renders draft-boards dc-2's disclosed error page NAMING
// the failure (a 500, not a fabricated 404) — still never an unproven
// vanish claim.
func TestCatchAll_BranchProbeError_NeverClaimsVanished(t *testing.T) {
	root := newBranchBoardFixture(t)
	h := NewHandler(root)

	orig := ensureWorktree
	ensureWorktree = func(ctx context.Context, root, branch string) (string, error) {
		return "", errAny
	}
	defer func() { ensureWorktree = orig }()

	req := httptest.NewRequest(http.MethodGet, "/b/design%2Fgone/board/spec/gone", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	code, body := rec.Code, rec.Body.String()

	if code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want the disclosed 500", code)
	}
	if strings.Contains(body, `data-testid="stale-entry-notice"`) {
		t.Fatalf("an unproven vanish claim was asserted; got: %s", body)
	}
	if !strings.Contains(body, "probe failed") {
		t.Fatalf("the failure is not named; got: %s", body)
	}
}

var errAny = &probeErr{}

type probeErr struct{}

func (*probeErr) Error() string { return "probe failed" }

// TestCatchAll_NamelessBranchAddress_GenericNotFound pins the retirement of
// the vestigial draftBoardBranch parser (ADJ-71 gq-2-n1). A degenerate
// per-branch board address with an EMPTY spec name
// (/b/<branch>/board/spec/, trailing slash) matches no board route, so it
// falls to this catch-all — and it is a plain unserved path: the generic
// disclosed 404, exactly like the unprefixed /board/spec/ nameless address,
// never a fabricated "design branch is gone" claim (which the retired parser
// used to emit here whenever the branch happened not to resolve). The REAL
// vanished-branch surface — a WELL-FORMED address carrying a name — is owned
// by branchboard.go's dispatch and proven by
// TestCatchAll_StaleBranch_DisclosedNotice above.
func TestCatchAll_NamelessBranchAddress_GenericNotFound(t *testing.T) {
	home := HomeDeps{Git: fakeHomeGit{}}
	code, body := getPath(t, home, "/b/design%2Fgone/board/spec/")

	if code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", code)
	}
	if strings.Contains(body, `data-testid="stale-entry-notice"`) {
		t.Fatalf("a nameless address must not fabricate a vanished-branch claim; got: %s", body)
	}
	if !strings.Contains(body, `data-testid="path-not-found"`) {
		t.Fatalf("nameless address is not the generic disclosed 404; got: %s", body)
	}
}
