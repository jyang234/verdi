package workbench

import (
	"context"
	"errors"
	"fmt"
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
frozen: { at: 2026-03-02, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
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
	// The kernel's mutable-branch rule (AC-2: identical semantics through
	// the shared core) permits typed mutations only on design/<spec-name>,
	// so the WRITE tests ride a branch named for its spec.
	commitSpecOnBranch(t, root, "design/draft-a", map[string]string{
		".verdi/specs/active/draft-a/spec.md": draftASpec,
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

	// The drafts' PROPOSED state (and so each branch board's authoring
	// mode) is git-derived now: the default branch must be provable.
	setDefaultBranchSymref(t, root)

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
	h := newBoardTestHandler(root)

	// Cut the worktree (lazy, on first request), then build the typed
	// mutation against the WORKTREE's own state — the branch board's
	// checkout identity is the worktree, not the serving root.
	if rec := bGet(t, h, "/b/design%2Fdraft-a/board/spec/draft-a"); rec.Code != http.StatusOK {
		t.Fatalf("prefixed board = %d\n%s", rec.Code, rec.Body.String())
	}
	wtRoot := filepath.Join(root, ".verdi", "data", "worktrees", "draft-a")
	envelope := mutateEnvelope(t, wtRoot, "draft-a", []map[string]any{
		{"op": "edit-ac", "id": "ac-1", "text": "criterion edited beneath the prefix", "evidence": []string{"attestation"}, "anchor": "#ac-1"},
	}, nil, nil)
	if rec := bPost(t, h, "/b/design%2Fdraft-a/board/spec/draft-a/api/mutate_draft", envelope); rec.Code != http.StatusOK {
		t.Fatalf("prefixed api mutate_draft = %d, want 200\n%s", rec.Code, rec.Body.String())
	}

	rec := bGet(t, h, "/b/design%2Fdraft-a/board/spec/draft-a/fragment")
	if rec.Code != http.StatusOK {
		t.Fatalf("prefixed fragment = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "criterion edited beneath the prefix") {
		t.Error("prefixed fragment does not reflect the prefixed api mutation")
	}

	// The edit landed in the managed worktree's spec.md...
	wtSpec := filepath.Join(wtRoot, ".verdi", "specs", "active", "draft-a", "spec.md")
	got, err := os.ReadFile(wtSpec)
	if err != nil {
		t.Fatalf("reading worktree spec: %v", err)
	}
	if !strings.Contains(string(got), "criterion edited beneath the prefix") {
		t.Error("edit did not land in the managed worktree's spec.md")
	}

	if rec := bGet(t, h, "/b/design%2Fdraft-a/board/spec/draft-a/peek?ref=spec/landed-spec"); rec.Code != http.StatusOK {
		t.Errorf("prefixed peek = %d, want 200", rec.Code)
	}
	if rec := bGet(t, h, "/b/design%2Fdraft-a/board/spec/draft-a/pinsearch?q=landed"); rec.Code != http.StatusOK {
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
	h := newBoardTestHandler(root)

	// Open both boards ("two tabs").
	if rec := bGet(t, h, "/b/design%2Fdraft-a/board/spec/draft-a"); rec.Code != http.StatusOK {
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

	// Edit through A (the typed transaction, against A's own worktree).
	wtA := filepath.Join(root, ".verdi", "data", "worktrees", "draft-a")
	envelope := mutateEnvelope(t, wtA, "draft-a", []map[string]any{
		{"op": "edit-ac", "id": "ac-1", "text": "edited only in branch A", "evidence": []string{"attestation"}, "anchor": "#ac-1"},
	}, nil, nil)
	if rec := bPost(t, h, "/b/design%2Fdraft-a/board/spec/draft-a/api/mutate_draft", envelope); rec.Code != http.StatusOK {
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
	gotA, err := os.ReadFile(filepath.Join(root, ".verdi", "data", "worktrees", "draft-a", ".verdi", "specs", "active", "draft-a", "spec.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gotA), "edited only in branch A") {
		t.Error("edit did not land in branch A's worktree")
	}

	assertServingCheckoutClean(t, root)
}

// TestBranchBoard_ModeLawPerInstance is ac-3's Go-level witness, as
// amended by final fix wave I6: modes are decided PER INSTANCE in the one
// handler, with no new mode value anywhere. The landed (frozen, accepted)
// spec renders sealed read-only at its unprefixed address; its DRAFT
// EDITION at the /b/ address — diverged bytes over the accepted revision,
// the exact diff VL-010 refuses at merge — now ALSO renders read-only,
// with a notice naming the divergence (I6: authoring requires
// Proposed/RelationNew; a modified accepted revision is never an
// authoring wall). The authoring half of the mode law is carried by
// draft-a, a genuinely NEW proposal on the same branch.
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

	// The draft edition over the accepted revision: read-only, rendered
	// from the design branch's own tree, with the divergence disclosed (I6).
	diverged := bGet(t, h, "/b/design%2Ftwo-a/board/spec/landed-spec")
	if diverged.Code != http.StatusOK {
		t.Fatalf("prefixed landed-spec = %d\n%s", diverged.Code, diverged.Body.String())
	}
	if !strings.Contains(diverged.Body.String(), `data-board-mode="readonly"`) {
		t.Error("draft edition over the accepted revision did not render read-only (I6: a modified accepted revision is never authorable)")
	}
	if !strings.Contains(diverged.Body.String(), "draft edition outcome text") {
		t.Error("prefixed render did not come from the design branch's tree")
	}
	if !strings.Contains(diverged.Body.String(), "diverge") {
		t.Error("diverged draft edition's read-only render carries no divergence notice")
	}

	// The authoring half of the mode law: a NEW proposal on the same
	// branch is the live authoring wall, in the same handler.
	authoring := bGet(t, h, "/b/design%2Ftwo-a/board/spec/draft-a")
	if authoring.Code != http.StatusOK {
		t.Fatalf("prefixed draft-a = %d\n%s", authoring.Code, authoring.Body.String())
	}
	if !strings.Contains(authoring.Body.String(), `data-board-mode="authoring"`) {
		t.Error("the new proposal at its /b/ address did not render authoring")
	}

	// Both remain reachable in the same session — neither changed the other.
	sealedAgain := bGet(t, h, "/board/spec/landed-spec")
	if sealedAgain.Code != http.StatusOK || !strings.Contains(sealedAgain.Body.String(), `data-board-mode="readonly"`) {
		t.Error("unprefixed sealed render changed after opening the /b/ boards")
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
	apiRec := bPost(t, h, "/b/design%2Fremote-only/board/spec/remote-spec/api/mutate_draft", `{}`)
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
	rec := bPost(t, h, "/b/design%2Fnever-existed/board/spec/whatever/api/mutate_draft", `{}`)
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
// git-switch action refuses beneath a per-branch board's /b/ address — the
// branch is the address (dc-1), and re-pointing the managed worktree would
// break the seam's branch<->path mapping. Here the managed worktree
// already exists (an earlier GET cut it), and the refusal leaves it on its
// branch. design/two-a still has its local ref and is not the serving
// checkout's branch, so the dispatch's refusedSwitch answers this request
// before the cached instance sees it. The instance's own guard
// (actionGitSwitch's fixedBranch refusal) is reached only once
// refusedSwitch lets a request through, which
// TestBranchBoard_GitSwitch_InstanceGuardRefuses pins.
func TestBranchBoard_GitSwitch_RefusesOnFixedBranch(t *testing.T) {
	root := newBranchBoardFixture(t)
	h := NewHandler(root)
	if rec := bGet(t, h, "/b/design%2Ftwo-a/board/spec/draft-a"); rec.Code != http.StatusOK {
		t.Fatalf("GET /b/ board = %d, want 200\n%s", rec.Code, rec.Body.String())
	}

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

// TestBranchBoard_GitSwitch_InstanceGuardRefuses pins the per-branch
// instance's own refusal (actionGitSwitch on an instance with a
// fixedBranch), which refusedSwitch otherwise answers first (R3ab review
// R3-A1). The managed worktree for design/two-a exists and its instance is
// cached. The branch is then renamed, so design/two-a has no local ref and
// refusedSwitch lets the switch through to that cached instance. Its guard
// alone refuses, with 403, and leaves the managed worktree on the renamed
// branch; without it, the switch would re-point the worktree.
func TestBranchBoard_GitSwitch_InstanceGuardRefuses(t *testing.T) {
	ctx := context.Background()
	root := newBranchBoardFixture(t)
	h := NewHandler(root)
	if rec := bGet(t, h, "/b/design%2Ftwo-a/board/spec/draft-a"); rec.Code != http.StatusOK {
		t.Fatalf("GET /b/ board = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	wt := filepath.Join(root, ".verdi", "data", "worktrees", "two-a")
	runGitBB(t, root, "branch", "-m", "design/two-a", "design/renamed")
	if branch, err := gitx.CurrentBranch(ctx, wt); err != nil || branch != "design/renamed" {
		t.Fatalf("after the rename the managed worktree is on %q (%v), want design/renamed", branch, err)
	}
	if local, err := gitx.HasLocalBranch(ctx, root, "design/two-a"); err != nil || local {
		t.Fatalf("design/two-a still resolves locally (%v, %v), so refusedSwitch would answer first", local, err)
	}

	rec := bPost(t, h, "/b/design%2Ftwo-a/board/spec/draft-a/api/git-switch", `{"branch":"design/two-b"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("git-switch on the cached per-branch instance = %d, want 403\n%s", rec.Code, rec.Body.String())
	}
	if want := fmt.Sprintf(fixedBranchSwitchRefusal, "design/two-a"); !strings.Contains(rec.Body.String(), want) {
		t.Errorf("git-switch refusal = %s, want %q", rec.Body.String(), want)
	}
	if branch, err := gitx.CurrentBranch(ctx, wt); err != nil || branch != "design/renamed" {
		t.Errorf("the managed worktree is on %q (%v) after the refused switch, want design/renamed", branch, err)
	}
	assertServingCheckoutClean(t, root)
}

// TestBranchBoard_GitSwitch_RefusesABranchCheckedOutElsewhere pins
// git-switch for a branch checked out in another, unmanaged linked
// worktree (R3ab review R3-A3). refusedSwitch refuses it with 403, so the
// serving checkout, to which the dispatch would otherwise hand the
// request, keeps its branch, and nothing is cut. Every other /b/ route in
// that state is open (backlog BL-150) and is not pinned here.
func TestBranchBoard_GitSwitch_RefusesABranchCheckedOutElsewhere(t *testing.T) {
	ctx := context.Background()
	root := newBranchBoardFixture(t)
	elsewhere := filepath.Join(t.TempDir(), "elsewhere")
	runGitBB(t, root, "worktree", "add", "--quiet", elsewhere, "design/two-b")
	// The serving checkout carries draft-a and is not on main, so a switch
	// it served would show.
	runGitBB(t, root, "checkout", "--quiet", "design/draft-a")
	before := repoGitState(t, root)

	rec := bPost(t, NewHandler(root), "/b/design%2Ftwo-b/board/spec/draft-a/api/git-switch", `{"branch":"main"}`)
	if want := fmt.Sprintf(fixedBranchSwitchRefusal, "design/two-b"); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), want) {
		t.Fatalf("git-switch for a branch checked out elsewhere = %d %s, want 403 %q", rec.Code, rec.Body.String(), want)
	}
	if branch, err := gitx.CurrentBranch(ctx, root); err != nil || branch != "design/draft-a" {
		t.Errorf("the serving checkout is on %q (%v) after the refused switch, want design/draft-a", branch, err)
	}
	if branch, err := gitx.CurrentBranch(ctx, elsewhere); err != nil || branch != "design/two-b" {
		t.Errorf("the other worktree is on %q (%v) after the refused switch, want design/two-b", branch, err)
	}
	if after := repoGitState(t, root); after != before {
		t.Errorf("the refused switch changed git state:\nbefore\n%s\nafter\n%s", before, after)
	}
}

// repoGitState is everything a /b/ request could mutate in git: every ref
// with its object, the linked-worktree list, and the worktree admin
// entries under the git directory.
func repoGitState(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	for _, args := range [][]string{
		{"for-each-ref", "--format=%(refname) %(objectname)"},
		{"worktree", "list", "--porcelain"},
	} {
		out, err := exec.CommandContext(context.Background(), "git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		b.Write(out)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".git", "worktrees"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, e := range entries {
		b.WriteString("admin " + e.Name() + "\n")
	}
	return b.String()
}

// TestBranchBoard_GitSwitch_FirstUseRefusesBeforeTheCut is ledger SI-341
// (3): a branch switch beneath /b/ for a local branch that is not the
// serving checkout's own can only be refused (the branch is the address),
// so the dispatch refuses it before ensuring the branch's managed worktree.
// The refused first use leaves no worktree, no ref, and no worktree admin
// entry: a mutation that a refusal leaves behind is outside every
// declaration (SI-325 (3)). The same request on a remote-only or absent
// branch keeps its own answer, and on the branch checked out at the serving
// root it still reaches the serving instance and switches.
func TestBranchBoard_GitSwitch_FirstUseRefusesBeforeTheCut(t *testing.T) {
	root := newBranchBoardFixture(t)
	h := NewHandler(root)
	before := repoGitState(t, root)

	rec := bPost(t, h, "/b/design%2Ftwo-a/board/spec/draft-a/api/git-switch", `{"branch":"design/two-b"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("first-use git-switch on a /b/ board = %d, want 403\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "the branch is the address") {
		t.Errorf("git-switch refusal is not disclosed: %s", rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".verdi", "data", "worktrees", "two-a")); !os.IsNotExist(err) {
		t.Errorf("the refused switch cut the managed worktree (stat err %v)", err)
	}
	if after := repoGitState(t, root); after != before {
		t.Errorf("the refused switch changed git state:\nbefore\n%s\nafter\n%s", before, after)
	}
	assertServingCheckoutClean(t, root)

	for _, tc := range []struct {
		name, target string
		want         int
		words        string
	}{
		{"a remote-only branch keeps its sealed refusal", "/b/design%2Fremote-only/board/spec/remote-spec/api/git-switch", http.StatusForbidden, "remote-tracking"},
		{"an absent branch keeps its disclosed 404", "/b/design%2Fnever-existed/board/spec/whatever/api/git-switch", http.StatusNotFound, "no longer resolves"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := bPost(t, h, tc.target, `{"branch":"main"}`)
			if rec.Code != tc.want || !strings.Contains(rec.Body.String(), tc.words) {
				t.Fatalf("POST %s = %d %s, want %d naming %q", tc.target, rec.Code, rec.Body.String(), tc.want, tc.words)
			}
		})
	}

	t.Run("the branch checked out at the serving root still switches", func(t *testing.T) {
		if err := gitx.Checkout(context.Background(), root, "design/two-b"); err != nil {
			t.Fatalf("checkout design/two-b: %v", err)
		}
		rec := bPost(t, NewHandler(root), "/b/design%2Ftwo-b/board/spec/draft-b/api/git-switch", `{"branch":"main"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("git-switch beneath the serving checkout's own /b/ address = %d, want 200\n%s", rec.Code, rec.Body.String())
		}
		assertServingCheckoutClean(t, root)
	})
}

// TestBranchBoards_RefusedSwitch is refusedSwitch's table: it answers only
// a POST api/git-switch, refuses a local branch other than the serving
// checkout's own with the per-branch instance's 403, lets the serving
// checkout's branch and a branch with no local ref through, and refuses
// with a 500 naming the failure when the branch cannot be read. Nothing it
// answers cuts a worktree.
func TestBranchBoards_RefusedSwitch(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"README.md": "x\n"}, Message: "seed"}})
	runGitBB(t, repo.Dir, "branch", "design/other")
	routes := map[string]boardSpecRoute{}
	for _, rt := range boardSpecRoutes() {
		routes[rt.suffix] = rt
	}
	tests := []struct {
		name     string
		root     string
		suffix   string
		method   string
		action   string
		branch   string
		answered bool
		code     int
		words    string
	}{
		{"a local branch other than the serving one is refused", repo.Dir, routeBoardAPI, http.MethodPost, "git-switch", "design/other", true, http.StatusForbidden, "the branch is the address"},
		{"the serving checkout's own branch falls through", repo.Dir, routeBoardAPI, http.MethodPost, "git-switch", "main", false, 0, ""},
		{"a branch with no local ref falls through", repo.Dir, routeBoardAPI, http.MethodPost, "git-switch", "design/absent", false, 0, ""},
		{"another api action falls through", repo.Dir, routeBoardAPI, http.MethodPost, "git-commit", "design/other", false, 0, ""},
		{"a GET falls through", repo.Dir, routeBoardAPI, http.MethodGet, "git-switch", "design/other", false, 0, ""},
		{"another route falls through", repo.Dir, routeBoardPage, http.MethodPost, "git-switch", "design/other", false, 0, ""},
		{"an unreadable branch refuses naming the failure", t.TempDir(), routeBoardAPI, http.MethodPost, "git-switch", "design/other", true, http.StatusInternalServerError, "could not resolve branch design/other"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := newBranchBoards(tt.root, Deps{}, nil)
			req := httptest.NewRequest(tt.method, "/b/x/board/spec/s/api/"+tt.action, strings.NewReader(`{"branch":"main"}`))
			req.SetPathValue("branch", tt.branch)
			req.SetPathValue("action", tt.action)
			rec := httptest.NewRecorder()
			got := b.refusedSwitch(rec, req, tt.branch, routes[tt.suffix])
			if got != tt.answered {
				t.Fatalf("refusedSwitch = %v, want %v (answer %d %s)", got, tt.answered, rec.Code, rec.Body.String())
			}
			if tt.answered && (rec.Code != tt.code || !strings.Contains(rec.Body.String(), tt.words)) {
				t.Fatalf("answer = %d %s, want %d naming %q", rec.Code, rec.Body.String(), tt.code, tt.words)
			}
			if !tt.answered && rec.Body.Len() != 0 {
				t.Fatalf("refusedSwitch wrote %q while letting the request through", rec.Body.String())
			}
			if _, err := os.Stat(filepath.Join(tt.root, ".verdi", "data", "worktrees")); !os.IsNotExist(err) {
				t.Fatalf("refusedSwitch touched the worktrees zone (stat err %v)", err)
			}
		})
	}
}

// TestBranchBoard_FirstUseCutsTheWorktreeOnEveryOtherRoute: every other
// /b/ route, and every other api action, still ensures the branch's
// managed worktree on first use, whatever it then answers — only the
// branch switch refuses before the cut (SI-341 (3)).
func TestBranchBoard_FirstUseCutsTheWorktreeOnEveryOtherRoute(t *testing.T) {
	root := newBranchBoardFixture(t)
	h := NewHandler(root)
	path := strings.NewReplacer("{name}", "landed-spec", "{action}", "sticky")
	for i, rt := range boardSpecRoutes() {
		t.Run(rt.suffix, func(t *testing.T) {
			name := fmt.Sprintf("first-use-%d", i)
			runGitBB(t, root, "branch", "design/"+name, "main")
			target := "/b/design%2F" + name + path.Replace(rt.suffix)
			rec := httptest.NewRecorder()
			if rt.suffix == routeBoardAPI {
				h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, target, strings.NewReader(`{}`)))
			} else {
				h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
			}
			if _, err := os.Stat(filepath.Join(root, ".verdi", "data", "worktrees", name)); err != nil {
				t.Errorf("%s (answered %d) did not cut the managed worktree on first use: %v", target, rec.Code, err)
			}
		})
	}
	assertServingCheckoutClean(t, root)
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
