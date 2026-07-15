package workbench

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// The per-branch fixture specs (spec/draft-boards): two draft feature
// specs, each committed on its own design branch only, plus one spec that
// exists BOTH landed on main and as a draft edition on a design branch
// (ac-3's same-spec-two-modes shape).

const draftASpec = `---
id: spec/draft-a
kind: spec
class: feature
title: "Draft A"
status: draft
owners: [platform-team]
problem: { text: "tab A problem text", anchor: "#problem" }
outcome: { text: "tab A outcome text", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "tab A original criterion", evidence: [attestation], anchor: "#ac-1" }
---
# Draft A

## Problem

## Outcome

## ac-1

Prose.
`

const draftBSpec = `---
id: spec/draft-b
kind: spec
class: feature
title: "Draft B"
status: draft
owners: [platform-team]
problem: { text: "tab B problem text", anchor: "#problem" }
outcome: { text: "tab B outcome text", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "tab B original criterion", evidence: [attestation], anchor: "#ac-1" }
---
# Draft B

## Problem

## Outcome

## ac-1

Prose.
`

const landedSpec = `---
id: spec/landed-spec
kind: spec
class: feature
title: "Landed spec"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "landed problem text", anchor: "#problem" }
outcome: { text: "landed outcome text", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "landed criterion", evidence: [attestation], anchor: "#ac-1" }
frozen: { at: 2026-03-02, commit: 66588948af8b36c02c8fb8f423645afa0a58dbe4 }
---
# Landed spec

## Problem

## Outcome

## ac-1

Prose.
`

const landedSpecDraftEdition = `---
id: spec/landed-spec
kind: spec
class: feature
title: "Landed spec"
status: draft
owners: [platform-team]
problem: { text: "landed problem text", anchor: "#problem" }
outcome: { text: "draft edition outcome text", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "landed criterion", evidence: [attestation], anchor: "#ac-1" }
---
# Landed spec

## Problem

## Outcome

## ac-1

Prose.
`

const remoteOnlySpec = `---
id: spec/remote-spec
kind: spec
class: feature
title: "Remote spec"
status: draft
owners: [platform-team]
problem: { text: "remote-only problem text", anchor: "#problem" }
outcome: { text: "remote-only outcome text", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "remote-only criterion", evidence: [attestation], anchor: "#ac-1" }
---
# Remote spec

## Problem

## Outcome

## ac-1

Prose.
`

// runGitBB runs git in dir with a fixed identity, failing the test on a
// non-zero exit (the same shape wtmanager's own test helper uses).
func runGitBB(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Verdi Fixture", "GIT_AUTHOR_EMAIL=fixture@verdi.invalid",
		"GIT_COMMITTER_NAME=Verdi Fixture", "GIT_COMMITTER_EMAIL=fixture@verdi.invalid",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// commitSpecOnBranch cuts branch from the current checkout, commits the
// given spec files on it, and returns to main — the serving checkout never
// serves while this provisioning runs.
func commitSpecOnBranch(t *testing.T, root, branch string, files map[string]string) {
	t.Helper()
	ctx := context.Background()
	if err := gitx.CheckoutNewBranch(ctx, root, branch); err != nil {
		t.Fatalf("CheckoutNewBranch(%s): %v", branch, err)
	}
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGitBB(t, root, "add", "-A")
	runGitBB(t, root, "commit", "--quiet", "--no-verify", "-m", "fixture: "+branch)
	if err := gitx.Checkout(ctx, root, "main"); err != nil {
		t.Fatalf("Checkout(main): %v", err)
	}
}

// newBranchBoardFixture builds the draft-boards fixture store: main
// carries landed-spec; design/two-a carries draft-a plus landed-spec's
// draft edition; design/two-b carries draft-b; design/remote-only exists
// ONLY as a remote-tracking ref (pushed to a local bare origin, local
// branch deleted). The serving checkout ends on main, clean.
func newBranchBoardFixture(t *testing.T) string {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/landed-spec/spec.md": landedSpec,
			".verdi/.gitignore":                       "data/\n",
		},
		Message: "seed main",
	}})
	root := repo.Dir

	commitSpecOnBranch(t, root, "design/two-a", map[string]string{
		".verdi/specs/active/draft-a/spec.md":     draftASpec,
		".verdi/specs/active/landed-spec/spec.md": landedSpecDraftEdition,
	})
	commitSpecOnBranch(t, root, "design/two-b", map[string]string{
		".verdi/specs/active/draft-b/spec.md": draftBSpec,
	})

	// The remote-only branch: committed, pushed to a local bare origin
	// (no network), then the local branch deleted — only
	// refs/remotes/origin/design/remote-only survives.
	origin := filepath.Join(t.TempDir(), "origin.git")
	runGitBB(t, "", "init", "--bare", "--quiet", "--initial-branch=main", origin)
	runGitBB(t, root, "remote", "add", "origin", origin)
	commitSpecOnBranch(t, root, "design/remote-only", map[string]string{
		".verdi/specs/active/remote-spec/spec.md": remoteOnlySpec,
	})
	runGitBB(t, root, "push", "--quiet", "origin", "design/remote-only")
	runGitBB(t, root, "branch", "-D", "design/remote-only")

	return root
}

// bGet issues a GET against the handler for a raw (possibly %2F-escaped)
// target and returns the recorder.
func bGet(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func bPost(t *testing.T, h http.Handler, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, target, strings.NewReader(body)))
	return rec
}

// assertServingCheckoutClean asserts the serving checkout sits on main
// with a clean working tree — feature dc-1's no-surprise-mutation law,
// checked after every exchange.
func assertServingCheckoutClean(t *testing.T, root string) {
	t.Helper()
	ctx := context.Background()
	branch, err := gitx.CurrentBranch(ctx, root)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch != "main" {
		t.Fatalf("serving checkout switched to %q — the /b/ routes must never switch any checkout", branch)
	}
	dirty, err := gitx.StatusDirty(ctx, root)
	if err != nil {
		t.Fatalf("StatusDirty: %v", err)
	}
	if dirty {
		out, _ := exec.Command("git", "-C", root, "status", "--porcelain").CombinedOutput()
		t.Fatalf("serving checkout's working tree is dirty after serving /b/ boards:\n%s", out)
	}
}

// TestBranchBoard_AuthoringFromManagedWorktree is ac-1's Go-level witness:
// GET /b/<branch-escaped>/board/spec/<name> serves the draft's board in
// authoring mode from the seam-obtained managed worktree, content from the
// design branch's tree, while the unprefixed address does not know the
// spec at all (it is not on the serving checkout's tree) and the serving
// checkout stays untouched.
func TestBranchBoard_AuthoringFromManagedWorktree(t *testing.T) {
	root := newBranchBoardFixture(t)
	h := NewHandler(root)

	rec := bGet(t, h, "/b/design%2Ftwo-a/board/spec/draft-a")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /b/ board = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `data-board-mode="authoring"`) {
		t.Error("per-branch draft board did not render in authoring mode (the unchanged mode law, applied to the worktree's own branch state)")
	}
	if !strings.Contains(body, "tab A problem text") {
		t.Error("board content did not come from the design branch's tree")
	}

	// The worktree was cut lazily by the seam, under the data zone (co-1).
	wt := filepath.Join(root, ".verdi", "data", "worktrees", "two-a")
	if _, err := os.Stat(wt); err != nil {
		t.Errorf("managed worktree not at the seam's deterministic path: %v", err)
	}

	// Unprefixed: draft-a is not on the serving checkout's tree (dc-3 — the
	// unprefixed address keeps serving the serving checkout, unchanged).
	if rec := bGet(t, h, "/board/spec/draft-a"); rec.Code != http.StatusNotFound {
		t.Errorf("unprefixed GET for a branch-only draft = %d, want 404", rec.Code)
	}

	assertServingCheckoutClean(t, root)
}

// TestBranchBoard_SubroutesBeneathPrefix is ac-1's sub-route half: a board
// mutation through the prefixed api route succeeds, the prefixed fragment
// reflects it, and peek/pinsearch answer beneath the prefix — the existing
// handlers, mounted per branch.
func TestBranchBoard_SubroutesBeneathPrefix(t *testing.T) {
	root := newBranchBoardFixture(t)
	h := NewHandler(root)

	if rec := bPost(t, h, "/b/design%2Ftwo-a/board/spec/draft-a/api/edit-text",
		`{"id":"ac-1","text":"criterion edited beneath the prefix"}`); rec.Code != http.StatusOK {
		t.Fatalf("prefixed api edit-text = %d, want 200\n%s", rec.Code, rec.Body.String())
	}

	rec := bGet(t, h, "/b/design%2Ftwo-a/board/spec/draft-a/fragment")
	if rec.Code != http.StatusOK {
		t.Fatalf("prefixed fragment = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "criterion edited beneath the prefix") {
		t.Error("prefixed fragment does not reflect the prefixed api mutation")
	}

	// The edit landed in the managed worktree's spec.md...
	wtSpec := filepath.Join(root, ".verdi", "data", "worktrees", "two-a", ".verdi", "specs", "active", "draft-a", "spec.md")
	got, err := os.ReadFile(wtSpec)
	if err != nil {
		t.Fatalf("reading worktree spec: %v", err)
	}
	if !strings.Contains(string(got), "criterion edited beneath the prefix") {
		t.Error("edit did not land in the managed worktree's spec.md")
	}

	if rec := bGet(t, h, "/b/design%2Ftwo-a/board/spec/draft-a/peek?ref=spec/landed-spec"); rec.Code != http.StatusOK {
		t.Errorf("prefixed peek = %d, want 200", rec.Code)
	}
	if rec := bGet(t, h, "/b/design%2Ftwo-a/board/spec/draft-a/pinsearch?q=landed"); rec.Code != http.StatusOK {
		t.Errorf("prefixed pinsearch = %d, want 200", rec.Code)
	}

	assertServingCheckoutClean(t, root)
}

// TestBranchBoard_TwoBranches_EditIsolation is ac-2's Go-level witness:
// boards from two design branches serve concurrently, an edit through one
// lands only in its own branch's managed worktree, and the serving
// checkout's working tree stays clean throughout.
func TestBranchBoard_TwoBranches_EditIsolation(t *testing.T) {
	root := newBranchBoardFixture(t)
	h := NewHandler(root)

	// Open both boards ("two tabs").
	if rec := bGet(t, h, "/b/design%2Ftwo-a/board/spec/draft-a"); rec.Code != http.StatusOK {
		t.Fatalf("board A = %d\n%s", rec.Code, rec.Body.String())
	}
	if rec := bGet(t, h, "/b/design%2Ftwo-b/board/spec/draft-b"); rec.Code != http.StatusOK {
		t.Fatalf("board B = %d\n%s", rec.Code, rec.Body.String())
	}

	// Board B's rendered region, before A's edit.
	before := bGet(t, h, "/b/design%2Ftwo-b/board/spec/draft-b/fragment")
	if before.Code != http.StatusOK {
		t.Fatalf("fragment B before = %d", before.Code)
	}

	// Edit through A.
	if rec := bPost(t, h, "/b/design%2Ftwo-a/board/spec/draft-a/api/edit-text",
		`{"id":"ac-1","text":"edited only in branch A"}`); rec.Code != http.StatusOK {
		t.Fatalf("edit via A = %d\n%s", rec.Code, rec.Body.String())
	}

	// B re-fetched: byte-for-byte unaffected.
	after := bGet(t, h, "/b/design%2Ftwo-b/board/spec/draft-b/fragment")
	if after.Code != http.StatusOK {
		t.Fatalf("fragment B after = %d", after.Code)
	}
	if before.Body.String() != after.Body.String() {
		t.Error("board B's rendered region changed after an edit through board A")
	}

	// Branch B's tree carries no trace of A's edit.
	wtB := filepath.Join(root, ".verdi", "data", "worktrees", "two-b")
	if err := filepath.WalkDir(wtB, walkContains(t, "edited only in branch A")); err != nil {
		t.Fatalf("walking worktree B: %v", err)
	}

	// A's tree carries it.
	gotA, err := os.ReadFile(filepath.Join(root, ".verdi", "data", "worktrees", "two-a", ".verdi", "specs", "active", "draft-a", "spec.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gotA), "edited only in branch A") {
		t.Error("edit did not land in branch A's worktree")
	}

	assertServingCheckoutClean(t, root)
}

// TestBranchBoard_ModeLawPerInstance is ac-3's Go-level witness: the SAME
// spec renders sealed read-only at its unprefixed (serving checkout)
// address and as an authoring wall at its design-branch /b/ address, in
// the same handler, with no new mode value anywhere.
func TestBranchBoard_ModeLawPerInstance(t *testing.T) {
	root := newBranchBoardFixture(t)
	h := NewHandler(root)

	sealed := bGet(t, h, "/board/spec/landed-spec")
	if sealed.Code != http.StatusOK {
		t.Fatalf("unprefixed landed-spec = %d\n%s", sealed.Code, sealed.Body.String())
	}
	if !strings.Contains(sealed.Body.String(), `data-board-mode="readonly"`) {
		t.Error("landed spec at its unprefixed address did not render read-only")
	}
	if !strings.Contains(sealed.Body.String(), "landed outcome text") {
		t.Error("unprefixed render did not come from the serving checkout's tree")
	}

	authoring := bGet(t, h, "/b/design%2Ftwo-a/board/spec/landed-spec")
	if authoring.Code != http.StatusOK {
		t.Fatalf("prefixed landed-spec = %d\n%s", authoring.Code, authoring.Body.String())
	}
	if !strings.Contains(authoring.Body.String(), `data-board-mode="authoring"`) {
		t.Error("draft edition at its /b/ address did not render authoring")
	}
	if !strings.Contains(authoring.Body.String(), "draft edition outcome text") {
		t.Error("prefixed render did not come from the design branch's tree")
	}

	// Both remain reachable in the same session — neither changed the other.
	sealedAgain := bGet(t, h, "/board/spec/landed-spec")
	if sealedAgain.Code != http.StatusOK || !strings.Contains(sealedAgain.Body.String(), `data-board-mode="readonly"`) {
		t.Error("unprefixed sealed render changed after opening the /b/ authoring wall")
	}

	assertServingCheckoutClean(t, root)
}

// TestBranchBoard_RemoteOnly_RendersSealed is dc-4's remote-tracking
// shape: read-only render from the ref's content, remoteness disclosed,
// no worktree cut, no local branch minted; a write beneath that address
// refuses.
func TestBranchBoard_RemoteOnly_RendersSealed(t *testing.T) {
	root := newBranchBoardFixture(t)
	h := NewHandler(root)

	rec := bGet(t, h, "/b/design%2Fremote-only/board/spec/remote-spec")
	if rec.Code != http.StatusOK {
		t.Fatalf("remote-only board = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `data-board-mode="readonly"`) {
		t.Error("remote-only branch did not render sealed (read-only)")
	}
	if !strings.Contains(body, "remote-tracking ref origin/design/remote-only") {
		t.Error("remoteness not disclosed in the board chrome")
	}
	if !strings.Contains(body, "remote-only problem text") {
		t.Error("sealed render did not come from the remote-tracking ref's content")
	}

	// No worktree cut, no local branch minted.
	if _, err := os.Stat(filepath.Join(root, ".verdi", "data", "worktrees", "remote-only")); !os.IsNotExist(err) {
		t.Error("a worktree was cut for a remote-only branch")
	}
	local, err := gitx.HasLocalBranch(context.Background(), root, "design/remote-only")
	if err != nil {
		t.Fatal(err)
	}
	if local {
		t.Error("a local branch was minted for a remote-only branch")
	}

	// The fragment renders sealed too; writes refuse with a disclosure.
	if rec := bGet(t, h, "/b/design%2Fremote-only/board/spec/remote-spec/fragment"); rec.Code != http.StatusOK {
		t.Errorf("remote-only fragment = %d, want 200", rec.Code)
	}
	apiRec := bPost(t, h, "/b/design%2Fremote-only/board/spec/remote-spec/api/edit-text",
		`{"id":"ac-1","text":"never lands"}`)
	if apiRec.Code != http.StatusForbidden {
		t.Errorf("write beneath a remote-only address = %d, want 403", apiRec.Code)
	}
	if !strings.Contains(apiRec.Body.String(), "remote-tracking") {
		t.Errorf("write refusal does not disclose remoteness: %s", apiRec.Body.String())
	}

	// A spec name the ref does not carry: a disclosed 404, never a bare one.
	gone := bGet(t, h, "/b/design%2Fremote-only/board/spec/no-such-spec")
	if gone.Code != http.StatusNotFound {
		t.Errorf("missing spec on remote ref = %d, want 404", gone.Code)
	}
	if !strings.Contains(gone.Body.String(), "origin/design/remote-only") {
		t.Error("missing-spec 404 does not name the ref")
	}

	assertServingCheckoutClean(t, root)
}

// TestBranchBoard_NoRef_DisclosedNotice is dc-4's no-ref shape: HTTP 404,
// a body naming the vanished branch, and a working link back to the
// directory — never a bare NotFound. Table-driven over the hostile and
// merely-absent segment shapes.
func TestBranchBoard_NoRef_DisclosedNotice(t *testing.T) {
	root := newBranchBoardFixture(t)
	h := NewHandler(root)

	cases := []struct {
		name   string
		target string
	}{
		{"absent branch", "/b/design%2Fnever-existed/board/spec/whatever"},
		{"traversal segments fail closed", "/b/design%2F..%2F..%2Fetc/board/spec/whatever"},
		{"non-design absent branch", "/b/no-such-branch/board/spec/whatever"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := bGet(t, h, tc.target)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("GET %s = %d, want 404\n%s", tc.target, rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			if !strings.Contains(body, `data-testid="stale-entry-notice"`) {
				t.Error("no-ref 404 is not the disclosed notice page")
			}
			if !strings.Contains(body, `data-testid="back-to-directory"`) {
				t.Error("no-ref 404 has no way back to the directory")
			}
		})
	}

	// The api edition of the same absence answers JSON, still 404.
	rec := bPost(t, h, "/b/design%2Fnever-existed/board/spec/whatever/api/edit-text", `{"id":"x","text":"y"}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("api under a no-ref branch = %d, want 404", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "no longer resolves") {
		t.Errorf("api 404 does not disclose the absence: %s", rec.Body.String())
	}

	assertServingCheckoutClean(t, root)
}

// TestBranchBoard_CheckedOutHere_ServesTheServingInstance: a /b/ branch
// already checked out at the serving root dispatches into the serving
// checkout's OWN board instance — that checkout IS the branch's working
// tree — with no worktree cut (the flagged resolution of wtmanager's
// ErrCheckedOutHere refusal; see branchboard.go).
func TestBranchBoard_CheckedOutHere_ServesTheServingInstance(t *testing.T) {
	root := newBranchBoardFixture(t)
	if err := gitx.Checkout(context.Background(), root, "design/two-a"); err != nil {
		t.Fatalf("checkout design/two-a: %v", err)
	}
	h := NewHandler(root)

	rec := bGet(t, h, "/b/design%2Ftwo-a/board/spec/draft-a")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /b/ for the checked-out branch = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `data-board-mode="authoring"`) {
		t.Error("checked-out-here branch did not render its authoring wall")
	}
	if _, err := os.Stat(filepath.Join(root, ".verdi", "data", "worktrees", "two-a")); !os.IsNotExist(err) {
		t.Error("a worktree was cut for the branch already checked out at the serving root")
	}
}

// TestBranchBoard_FailedCut_DisclosedErrorPage is dc-2's failure shape: a
// cut that fails renders a disclosed error page naming the failure —
// never a dead link, never a bare 500. The failing cut is injected
// through the seam variable; no real seam failure is hermetically
// reachable for a valid local branch.
func TestBranchBoard_FailedCut_DisclosedErrorPage(t *testing.T) {
	root := newBranchBoardFixture(t)
	h := NewHandler(root)

	orig := ensureWorktree
	ensureWorktree = func(ctx context.Context, root, branch string) (string, error) {
		return "", errors.New("disk full while cutting the worktree (injected)")
	}
	defer func() { ensureWorktree = orig }()

	rec := bGet(t, h, "/b/design%2Ftwo-a/board/spec/draft-a")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("failed cut = %d, want 500", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "disk full while cutting the worktree (injected)") {
		t.Error("the failure is not named on the disclosed error page")
	}
	if !strings.Contains(body, "design/two-a") {
		t.Error("the disclosed error page does not name the branch")
	}
}

// TestBranchBoard_GitSwitch_RefusesOnFixedBranch: the branch switcher's
// git-switch action refuses on a per-branch instance — the branch is the
// address under /b/ (dc-1), and re-pointing the managed worktree would
// break the seam's branch<->path mapping.
func TestBranchBoard_GitSwitch_RefusesOnFixedBranch(t *testing.T) {
	root := newBranchBoardFixture(t)
	h := NewHandler(root)

	rec := bPost(t, h, "/b/design%2Ftwo-a/board/spec/draft-a/api/git-switch", `{"branch":"design/two-b"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("git-switch on a /b/ board = %d, want 403\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "the branch is the address") {
		t.Errorf("git-switch refusal is not disclosed: %s", rec.Body.String())
	}

	// The worktree stayed on its branch.
	wt := filepath.Join(root, ".verdi", "data", "worktrees", "two-a")
	branch, err := gitx.CurrentBranch(context.Background(), wt)
	if err != nil {
		t.Fatal(err)
	}
	if branch != "design/two-a" {
		t.Errorf("managed worktree switched to %q", branch)
	}
}

// TestBranchBoard_ReuseAcrossRequests: the second open reuses the cut
// worktree and the SAME instance (dc-2's "subsequent opens reuse") — two
// GETs succeed and exactly one worktree directory exists.
func TestBranchBoard_ReuseAcrossRequests(t *testing.T) {
	root := newBranchBoardFixture(t)
	h := NewHandler(root)

	for i := 0; i < 2; i++ {
		if rec := bGet(t, h, "/b/design%2Ftwo-a/board/spec/draft-a"); rec.Code != http.StatusOK {
			t.Fatalf("open %d = %d", i+1, rec.Code)
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, ".verdi", "data", "worktrees"))
	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) != 1 || dirs[0] != "two-a" {
		t.Errorf("worktree dirs = %v, want exactly [two-a]", dirs)
	}
}

// walkContains returns a WalkDirFunc failing the test if any regular file
// beneath the walk root contains needle.
func walkContains(t *testing.T, needle string) func(path string, d os.DirEntry, err error) error {
	return func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(string(data), needle) {
			t.Errorf("%s contains %q — the edit leaked across branches", path, needle)
		}
		return nil
	}
}
