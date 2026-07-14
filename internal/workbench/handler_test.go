package workbench

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHealthHandler_Happy(t *testing.T) {
	h := NewHandler("/store/root")
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "ok" {
		t.Fatalf("body = %q, want \"ok\"", rec.Body.String())
	}
}

func TestHealthHandler_Negative(t *testing.T) {
	h := NewHandler("/store/root")
	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 for POST /healthz", rec.Code)
	}
}

func TestIndexHandler_Happy(t *testing.T) {
	h := NewHandler("/store/root")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "/store/root") {
		t.Fatalf("index page does not mention the store root: %s", rec.Body.String())
	}
}

// gitHome execs one git command for the home-page fixture provisioning
// below — a real local git repo, no network (co-2).
func gitHome(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=workbench-test", "GIT_AUTHOR_EMAIL=t@verdi.invalid",
		"GIT_COMMITTER_NAME=workbench-test", "GIT_COMMITTER_EMAIL=t@verdi.invalid",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// wipDraftSpec is the design-branch draft provisionHomeRefs commits: the
// leanest valid feature-class draft.
const wipDraftSpec = `---
id: spec/wip-draft
kind: spec
class: feature
title: "WIP draft (fixture)"
status: draft
owners: [platform-team]
acceptance_criteria:
  - { id: ac-1, text: "holds", evidence: [static] }
---
# WIP draft
`

// provisionHomeRefs gives the fixture repo the ref state the whole-store
// directory reads: a local bare origin with origin/HEAD set (the default-
// branch resolution refindex keys off) and one draft on a local design
// branch — the entry the old single-checkout home page could not show.
func provisionHomeRefs(t *testing.T, dir string) {
	t.Helper()
	origin := filepath.Join(t.TempDir(), "origin.git")
	gitHome(t, ".", "init", "--bare", "--quiet", "--initial-branch=main", origin)
	gitHome(t, dir, "remote", "add", "origin", origin)
	gitHome(t, dir, "push", "--quiet", "origin", "main")
	gitHome(t, dir, "remote", "set-head", "origin", "main")

	gitHome(t, dir, "checkout", "--quiet", "-b", "design/wip-draft")
	specDir := filepath.Join(dir, ".verdi", "specs", "active", "wip-draft")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(wipDraftSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	gitHome(t, dir, "add", ".verdi/specs/active/wip-draft")
	gitHome(t, dir, "commit", "--quiet", "--no-verify", "-m", "design: wip-draft")
	gitHome(t, dir, "checkout", "--quiet", "main")
}

// TestIndexHandler_Home is the whole-store directory's integration witness
// (spec/directory-home ac-1 over the real seam): production HomeDeps, a
// real git fixture repo with an origin and a design branch — the home page
// renders the grouped directory (default-branch specs AND the design-branch
// draft), the surviving sections, and working links.
func TestIndexHandler_Home(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)
	provisionHomeRefs(t, repo.Dir)

	// Add a discoverable service so the Services section has real data
	// (testdata/corpus carries no .flowmap.yaml of its own).
	svcDir := filepath.Join(repo.Dir, "home-service")
	if err := os.MkdirAll(svcDir, 0o755); err != nil {
		t.Fatalf("creating service dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(svcDir, ".flowmap.yaml"), []byte("version: 1\nservice: home-service\n"), 0o644); err != nil {
		t.Fatalf("writing .flowmap.yaml: %v", err)
	}

	h := NewHandler(repo.Dir)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	// The four dc-2 groups organize the page.
	for _, g := range []string{"drafts-in-progress", "accepted-pending-build", "active-components", "terminal"} {
		if !strings.Contains(body, `data-testid="dir-group-`+g+`"`) {
			t.Fatalf("home missing status group %s; got: %s", g, body)
		}
	}
	// The design-branch draft — the entry the old home could not show —
	// linked under the /b/ per-branch grammar with its source disclosed.
	if !strings.Contains(body, `href="/b/design%2Fwip-draft/board/spec/wip-draft"`) {
		t.Fatalf("home missing the design-branch draft's /b/ board link; got: %s", body)
	}
	if !strings.Contains(body, `badge-src-local">local branch</span>`) {
		t.Fatalf("home missing the design entry's source disclosure; got: %s", body)
	}
	// A fixture spec, linked to its corpus page and its unprefixed board.
	if !strings.Contains(body, `href="/a/spec/stale-decline"`) {
		t.Fatalf("home missing the fixture spec's corpus link; got: %s", body)
	}
	if !strings.Contains(body, `href="/board/spec/stale-decline"`) {
		t.Fatalf("home missing the fixture spec's board link; got: %s", body)
	}
	// stale-decline is a feature spec (story jira:LOAN-1482): it also links
	// its matrix and verdict pages via that scalar story ref.
	if !strings.Contains(body, `href="/matrix/jira:LOAN-1482"`) {
		t.Fatalf("home missing the feature spec's matrix link; got: %s", body)
	}
	if !strings.Contains(body, `href="/verdict/jira:LOAN-1482"`) {
		t.Fatalf("home missing the feature spec's verdict link; got: %s", body)
	}
	// The archived spec appears, in the terminal group.
	if !strings.Contains(body, `href="/a/spec/loan-refi-2023"`) {
		t.Fatalf("home missing the archived spec; got: %s", body)
	}
	// Other kinds grouped and linked (the corpus's ADRs).
	if !strings.Contains(body, `href="/a/adr/0002-outbox-events"`) {
		t.Fatalf("home missing an other-kind (adr) link; got: %s", body)
	}
	// The grandfathered v0 board, linked to its board page.
	if !strings.Contains(body, `href="/board/STORY-1482"`) {
		t.Fatalf("home missing the board link; got: %s", body)
	}
	// The discovered service, named.
	if !strings.Contains(body, "home-service") {
		t.Fatalf("home missing the discovered service; got: %s", body)
	}

	// Follow one href — the spec page — and confirm it resolves 200.
	req2 := httptest.NewRequest(http.MethodGet, "/a/spec/stale-decline", nil)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("following the spec href: status = %d, want 200; body=%s", rec2.Code, rec2.Body.String())
	}
}

// TestIndexHandler_HomeNoBoards proves the honest empty state: a store with
// no boards says so rather than rendering an empty list.
func TestIndexHandler_HomeNoBoards(t *testing.T) {
	repo := buildWorkbenchFixtureRepo(t)
	// Remove the fixture's one board so the boards dir is empty.
	if err := os.RemoveAll(boardsDirForTest(repo.Dir)); err != nil {
		t.Fatalf("clearing boards: %v", err)
	}

	h := NewHandler(repo.Dir)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "No boards yet.") {
		t.Fatalf("home does not honestly report the empty boards state; got: %s", rec.Body.String())
	}
}

func boardsDirForTest(root string) string {
	return filepath.Join(root, ".verdi", "data", "mutable", "boards")
}

func TestStyleCSSHandler_Happy(t *testing.T) {
	h := NewHandler("/store/root")
	req := httptest.NewRequest(http.MethodGet, "/assets/style.css", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Fatalf("Content-Type = %q, want text/css", ct)
	}
	body := rec.Body.String()
	// The composed stylesheet must carry both chroma palettes, so the
	// workbench's shared class-based code rendering is coloured and its dark
	// palette (github-dark, #e6edf3 foreground) lives inside the
	// prefers-color-scheme:dark block.
	if !strings.Contains(body, ".chroma-chroma") {
		t.Fatalf("workbench style.css missing the chroma palette; got:\n%s", body)
	}
	darkIdx := strings.Index(body, "@media (prefers-color-scheme: dark)")
	if darkIdx < 0 || !strings.Contains(body[darkIdx:], "#e6edf3") {
		t.Fatalf("workbench style.css missing the dark chroma palette in its dark media block; got:\n%s", body)
	}
}

func TestStyleCSSHandler_Negative(t *testing.T) {
	h := NewHandler("/store/root")
	req := httptest.NewRequest(http.MethodPost, "/assets/style.css", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 for POST /assets/style.css", rec.Code)
	}
}

func TestIndexHandler_Negative(t *testing.T) {
	h := NewHandler("/store/root")

	t.Run("unknown path 404s", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/nonexistent-page", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("wrong method on / is rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want 405 for POST /", rec.Code)
		}
	})
}
