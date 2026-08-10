package execworkspace

// Full-lifecycle integration test closing review finding F1: a single
// identity driven all the way through materialize -> release -> gc (rank 5)
// -> gc (rank 0) -> re-materialize, against the REAL production seam (a
// real Materializer over a real NewGitReconciler(storeRoot), a real
// Releaser, and the real GC entry point) rather than the per-rank/per-step
// unit fixtures gc_test.go and materialize_test.go already exercise in
// isolation. It proves the spec's own "legitimately fresh" clause
// end-to-end (spec/execution-workspace §Workspace naming, step 3.1: "once a
// complete reclaim has removed every trace, the same deterministic id is
// legitimately fresh for a new request") and §GC slice's rank-0/rank-5
// hand-off ("any surviving registration is resolved by a later gc").
//
// Fixtures reuse this package's own established helpers (buildTestRepo,
// lockAbsent from materialize_test.go; adminWorktreesDir, scanAdminDir,
// canonicalPath from reconcile_test.go/reconcile.go) rather than
// duplicating fixture logic.
import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
)

// commitWorktreeChanges commits every change currently in dir's working
// tree — this test's own scaffolding for reaching a clean, gc-eligible
// tree after a base-plus-patch materialization (which deliberately leaves
// the patch uncommitted), never a production code path.
func commitWorktreeChanges(t *testing.T, dir string) {
	t.Helper()
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=verdi-test", "GIT_AUTHOR_EMAIL=verdi-test@example.com",
		"GIT_COMMITTER_NAME=verdi-test", "GIT_COMMITTER_EMAIL=verdi-test@example.com",
	)
	addCmd := exec.Command("git", "-C", dir, "add", "-A")
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add -A (%s): %v\n%s", dir, err, out)
	}
	commitCmd := exec.Command("git", "-C", dir, "commit", "--quiet", "-m", "test: commit applied patch")
	commitCmd.Env = env
	if out, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit (%s): %v\n%s", dir, err, out)
	}
}

// runFullLifecycleOnce drives one identity (built by buildReq) through the
// whole cycle and returns nothing; every assertion is inline so a failure
// names the exact stage that broke.
func runFullLifecycleOnce(t *testing.T, buildReq func(repoHead string) Request) {
	t.Helper()
	ctx := context.Background()
	repo := buildTestRepo(t)
	storeRoot := t.TempDir()
	reconciler := NewGitReconciler(storeRoot)
	m, err := NewMaterializer(storeRoot, repo.Dir, reconciler)
	if err != nil {
		t.Fatalf("NewMaterializer: %v", err)
	}
	req := buildReq(repo.Head)
	workspaceID, werr := req.Identity.WorkspaceID()
	if werr != nil {
		t.Fatalf("Identity.WorkspaceID: %v", werr)
	}

	// --- materialize ---
	first, err := m.Materialize(ctx, req)
	if err != nil {
		t.Fatalf("Materialize (first): %v", err)
	}
	if first.Outcome != OutcomeMaterialized {
		t.Fatalf("first Outcome = %v, want OutcomeMaterialized", first.Outcome)
	}
	if first.WorkspaceID != workspaceID {
		t.Fatalf("first.WorkspaceID = %q, want %q", first.WorkspaceID, workspaceID)
	}
	unitPath := first.Path
	lockAbsent(t, storeRoot, workspaceID)

	// A base-plus-patch materialization leaves its worktree DIRTY (the
	// patch is applied, never committed) — gc's own rank 3 correctly keeps
	// a dirty workspace rather than reclaiming it, so this test's own
	// consumer-side lifecycle commits the applied patch, reaching the same
	// clean-and-eligible state a real consumer would produce before
	// invoking Release. This is scaffolding local to the test, not a
	// production behavior.
	if dirty, derr := gitx.StatusDirty(ctx, unitPath); derr != nil {
		t.Fatalf("StatusDirty before commit-the-patch scaffolding: %v", derr)
	} else if dirty {
		commitWorktreeChanges(t, unitPath)
	}

	// --- release ---
	releaser := NewReleaser(storeRoot)
	if err := releaser.Release(workspaceID); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if mustPathKind(t, ReleasedPath(storeRoot, workspaceID)) != PathRegular {
		t.Fatal(".released marker not a regular file after Release")
	}

	adminDir := adminWorktreesDir(t, repo.Dir)

	// --- gc run 1: rank 5, reclaim ---
	results1, _, err := GC(ctx, storeRoot, repo.Dir)
	if err != nil {
		t.Fatalf("GC run 1: %v", err)
	}
	r1 := mustFindResult(t, results1, workspaceID)
	if r1.Outcome != Reclaimed {
		t.Fatalf("gc run 1 Outcome = %v (detail=%q), want Reclaimed", r1.Outcome, r1.Detail)
	}
	wantLine := "execution: reclaimed: " + workspaceID + " (any surviving registration is resolved by a later gc)"
	if got := r1.Line(); got != wantLine {
		t.Fatalf("gc run 1 Line() = %q, want %q (the conditional wording — rank 5 verifies nothing about registrations)", got, wantLine)
	}
	for _, p := range []string{
		RequestStagingPath(storeRoot, workspaceID),
		RequestPath(storeRoot, workspaceID),
		unitPath,
		ReleasedPath(storeRoot, workspaceID),
		LockPath(storeRoot, workspaceID),
	} {
		if mustPathKind(t, p) != PathAbsent {
			t.Fatalf("gc run 1: %s still present after Reclaimed, want all five unit paths gone", p)
		}
	}
	matchesAfterRun1, err := scanAdminDir(adminDir, canonicalPath(unitPath))
	if err != nil {
		t.Fatalf("scanAdminDir after gc run 1: %v", err)
	}
	if len(matchesAfterRun1) != 1 {
		t.Fatalf("admin registrations resolving to the unit after gc run 1 = %d, want exactly 1 (rank 5 deletes no registration; the real `git worktree add` registration from materialize survives)", len(matchesAfterRun1))
	}

	// --- gc run 2: rank 0, reclaim-orphaned ---
	results2, _, err := GC(ctx, storeRoot, repo.Dir)
	if err != nil {
		t.Fatalf("GC run 2: %v", err)
	}
	r2 := mustFindResult(t, results2, workspaceID)
	if r2.Outcome != ReclaimOrphaned {
		t.Fatalf("gc run 2 Outcome = %v (detail=%q), want ReclaimOrphaned (rank 0 resolving the surviving registration)", r2.Outcome, r2.Detail)
	}
	matchesAfterRun2, err := scanAdminDir(adminDir, canonicalPath(unitPath))
	if err != nil {
		t.Fatalf("scanAdminDir after gc run 2: %v", err)
	}
	if len(matchesAfterRun2) != 0 {
		t.Fatalf("admin registrations resolving to the unit after gc run 2 = %d, want 0 (surviving registration resolved)", len(matchesAfterRun2))
	}

	// --- re-materialize the SAME identity: "legitimately fresh" ---
	second, err := m.Materialize(ctx, req)
	if err != nil {
		t.Fatalf("Materialize (re-materialize after complete reclaim): %v", err)
	}
	if second.Outcome != OutcomeMaterialized {
		t.Fatalf("re-materialize Outcome = %v, want OutcomeMaterialized (the spec's own 'legitimately fresh' clause: once a complete reclaim removes every trace, the same id is fresh)", second.Outcome)
	}
	if second.WorkspaceID != workspaceID {
		t.Fatalf("re-materialize WorkspaceID = %q, want %q (same deterministic id)", second.WorkspaceID, workspaceID)
	}
	witnessData, rerr := os.ReadFile(RequestPath(storeRoot, workspaceID))
	if rerr != nil {
		t.Fatalf("reading re-materialized witness: %v", rerr)
	}
	witnessID, derr := DecodeSidecar(witnessData)
	if derr != nil {
		t.Fatalf("decoding re-materialized witness: %v", derr)
	}
	if !witnessID.Equal(req.Identity) {
		t.Fatalf("re-materialized witness identity = %s, want %s", witnessID, req.Identity)
	}
	head, herr := gitx.RevParse(ctx, second.Path, "HEAD")
	if herr != nil {
		t.Fatalf("RevParse(HEAD) on re-materialized worktree: %v", herr)
	}
	if head != req.Identity.CommitSHA {
		t.Fatalf("re-materialized worktree HEAD = %q, want exact/base sha %q (detached, never a branch)", head, req.Identity.CommitSHA)
	}
	branch, berr := gitx.CurrentBranch(ctx, second.Path)
	if berr != nil {
		t.Fatalf("CurrentBranch on re-materialized worktree: %v", berr)
	}
	if branch != "" {
		t.Fatalf("re-materialized worktree is on branch %q, want detached", branch)
	}
	lockAbsent(t, storeRoot, workspaceID)
}

func mustFindResult(t *testing.T, results []GCResult, workspaceID string) GCResult {
	t.Helper()
	for _, r := range results {
		if r.WorkspaceID == workspaceID {
			return r
		}
	}
	t.Fatalf("no GCResult for workspace %q among %d results", workspaceID, len(results))
	return GCResult{}
}

// TestFullLifecycle_ExactSHA_MaterializeReleaseGCGCReMaterialize is the
// exact-SHA shape's full-cycle proof (contract item 1).
func TestFullLifecycle_ExactSHA_MaterializeReleaseGCGCReMaterialize(t *testing.T) {
	runFullLifecycleOnce(t, func(repoHead string) Request {
		id, err := NewExactIdentity("run-lifecycle-exact", repoHead)
		if err != nil {
			t.Fatalf("NewExactIdentity: %v", err)
		}
		return Request{Identity: id}
	})
}

// TestFullLifecycle_BasePlusPatch_MaterializeReleaseGCGCReMaterialize is the
// patch-shape variant of the same loop (contract item 1's "Also a
// patch-shape variant of the same loop").
func TestFullLifecycle_BasePlusPatch_MaterializeReleaseGCGCReMaterialize(t *testing.T) {
	repoForPatch := buildTestRepo(t)
	patch := buildPatchBytes(t, repoForPatch.Dir, "a.txt", "patched by full lifecycle\n")
	runFullLifecycleOnce(t, func(repoHead string) Request {
		// buildReq is called with the FRESH per-run repo's head (built
		// inside runFullLifecycleOnce), so the patch must be rebuilt
		// against THAT repo, not repoForPatch above — buildTestRepo
		// produces byte-identical layers/history on every call
		// (fixturegit is deterministic), so a patch built against one
		// instance applies cleanly against another with the same content.
		id, err := NewPatchIdentity("run-lifecycle-patch", repoHead, patch)
		if err != nil {
			t.Fatalf("NewPatchIdentity: %v", err)
		}
		return Request{Identity: id, PatchBytes: patch}
	})
}
