//go:build darwin

package execworkspace

// Contract item 3: EVERY DELETION FAILURE BOUNDARY (rank 5) — one
// engineered failure per fixed-order step, proving the disclosed Partial
// NAMES that step, that the landing matches spec §Safe cleanup's own
// "Deleting the witness first means every partial failure degrades into a
// state this spec has already defined" paragraph, and that a later run
// (obstacle cleared) finishes the job.
//
// The UNIT PATH (directory-removal) step is deliberately NOT re-tested
// here: gc_test.go's TestDecideUnit_Rank5_PartialAtDirectoryStep_
// ThenReEntrantFinish already engineers that exact failure (via os.Chmod
// on the unit directory itself — a distinct, self-contained directory
// whose own children need write+exec to unlink) and already proves its
// own re-entrant finish; duplicating it here would violate this lane's
// no-duplicate-test instruction. Collectively, all five rank-5 steps are
// covered: unit path there, the other four (.request.staging, .request,
// .released, .lock) here.
//
// MECHANISM (documented per the contract's own "permission engineering on
// the parent/target" wording): the four sibling steps below are FLAT
// children of one shared parent, data/execution/ — the SAME parent
// filelock.Acquire must create a NEW file in for the .lock step. Removing
// write permission from that shared PARENT would ALSO block lock
// ACQUISITION itself (a new file, needing parent write) before rank 5's
// deletion sequence even begins, and would ALSO block the (already-tested)
// directory step's own final unlink of the unit's directory ENTRY from
// that same parent — so a parent-level permission change cannot isolate
// ONE flat sibling's removal from the others or from lock acquisition.
// Darwin's per-file `chflags UF_IMMUTABLE` (TARGET-level engineering) has
// none of that coupling: it blocks unlink of exactly the flagged file and
// leaves lock acquisition and every other step's own removal untouched.
// This file is therefore darwin-only; deletion_boundary_other_test.go
// names the same limitation and documented-skips on every other platform
// rather than faking isolation with a weaker, coupled mechanism.
import (
	"context"
	"os"
	"strings"
	"syscall"
	"testing"
)

// ufImmutable is BSD/Darwin's UF_IMMUTABLE flag (<sys/stat.h>: 0x00000002).
// Hardcoded rather than imported from golang.org/x/sys/unix (an
// indirect-only module dependency this test-only file should not newly
// promote to direct) because Go's own syscall package exposes Chflags but
// not this constant on darwin.
const ufImmutable = 0x2

// mxSetImmutable sets (or clears) darwin's user-immutable flag on path,
// which blocks unlink/rename/write of exactly that file — independent of
// its parent directory's own write permission — without requiring root.
func mxSetImmutable(t *testing.T, path string, immutable bool) {
	t.Helper()
	flag := 0
	if immutable {
		flag = ufImmutable
	}
	if err := syscall.Chflags(path, flag); err != nil {
		t.Fatalf("chflags(%s, immutable=%v): %v", path, immutable, err)
	}
}

// mxClearImmutableBestEffort is the deferred-cleanup counterpart to
// mxSetImmutable(t, path, false): a successful re-entrant finish (this
// file's own tests each end by proving one) legitimately DELETES path
// before the function's own deferred safety-net cleanup runs, so ENOENT
// there is the SUCCESS case, not a test bug — never fatal.
func mxClearImmutableBestEffort(path string) {
	_ = syscall.Chflags(path, 0)
}

// TestDecideUnit_Rank5_DeletionFailureBoundary_StagingStep forces exactly
// the FIRST fixed-order step (.request.staging) to fail: staging is the
// only present regular sibling made immutable, so decideRank5's very
// first removeRegularIfPresent call is the one that fails — no other step
// is even attempted.
func TestDecideUnit_Rank5_DeletionFailureBoundary_StagingStep(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	id := gcUnitID(0)
	unitPath := cutGCUnit(t, repo, storeRoot, id)
	markReleased(t, storeRoot, id)

	stagingPath := RequestStagingPath(storeRoot, id)
	if err := os.WriteFile(stagingPath, []byte("staging residue"), 0o644); err != nil {
		t.Fatalf("planting staging residue: %v", err)
	}
	mxSetImmutable(t, stagingPath, true)
	defer mxClearImmutableBestEffort(stagingPath) // best-effort even if the test fails early

	res := decideUnit(context.Background(), storeRoot, repo.Dir, id, NewGitReconciler(storeRoot))
	if res.Outcome != Partial {
		t.Fatalf("Outcome = %v (detail=%q), want Partial (staging step forbidden by immutability)", res.Outcome, res.Detail)
	}
	if !strings.Contains(res.Detail, "request-staging") {
		t.Fatalf("Detail = %q, want it to NAME the request-staging step specifically", res.Detail)
	}
	// Landing: staging survives (unremoved); nothing downstream was even
	// attempted — the fixed order stops at its first failure.
	if mustPathKind(t, stagingPath) != PathRegular {
		t.Fatal("staging residue removed despite immutability (test setup invalid)")
	}
	if mustPathKind(t, RequestPath(storeRoot, id)) != PathAbsent {
		t.Fatal(".request present — test fixture invalid")
	}
	if mustPathKind(t, unitPath) != PathDir {
		t.Fatal("unit directory removed even though the staging step (which precedes it in the fixed order) failed")
	}
	if mustPathKind(t, ReleasedPath(storeRoot, id)) != PathRegular {
		t.Fatal(".released removed even though the staging step (which precedes it) failed")
	}
	lockAbsent(t, storeRoot, id) // the lock's own release is unaffected by staging's immutability

	// Later run (obstacle cleared): rank 5 re-decides and re-enters from
	// scratch, this time finishing every step.
	mxSetImmutable(t, stagingPath, false)
	res2 := decideUnit(context.Background(), storeRoot, repo.Dir, id, NewGitReconciler(storeRoot))
	if res2.Outcome != Reclaimed {
		t.Fatalf("re-entrant run: Outcome = %v (detail=%q), want Reclaimed once the obstacle is cleared", res2.Outcome, res2.Detail)
	}
	for _, p := range []string{stagingPath, RequestPath(storeRoot, id), unitPath, ReleasedPath(storeRoot, id), LockPath(storeRoot, id)} {
		if mustPathKind(t, p) != PathAbsent {
			t.Fatalf("%s still present after the re-entrant finish", p)
		}
	}
}

// TestDecideUnit_Rank5_DeletionFailureBoundary_RequestStep forces the
// SECOND fixed-order step (.request) to fail: staging is absent (a
// no-op, no permission needed to lstat an absent path), so request's own
// removeRegularIfPresent is the first one actually attempted, and the one
// that fails.
func TestDecideUnit_Rank5_DeletionFailureBoundary_RequestStep(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	id := gcUnitID(0)
	unitPath := cutGCUnit(t, repo, storeRoot, id)
	markReleased(t, storeRoot, id)

	requestPath := RequestPath(storeRoot, id)
	if err := os.WriteFile(requestPath, []byte("witness"), 0o644); err != nil {
		t.Fatalf("planting .request: %v", err)
	}
	mxSetImmutable(t, requestPath, true)
	defer mxClearImmutableBestEffort(requestPath)

	res := decideUnit(context.Background(), storeRoot, repo.Dir, id, NewGitReconciler(storeRoot))
	if res.Outcome != Partial {
		t.Fatalf("Outcome = %v (detail=%q), want Partial (request step forbidden by immutability)", res.Outcome, res.Detail)
	}
	if !strings.Contains(res.Detail, "request step") {
		t.Fatalf("Detail = %q, want it to NAME the request step specifically", res.Detail)
	}
	if mustPathKind(t, RequestStagingPath(storeRoot, id)) != PathAbsent {
		t.Fatal(".request.staging present — test fixture invalid")
	}
	if mustPathKind(t, requestPath) != PathRegular {
		t.Fatal(".request removed despite immutability (test setup invalid)")
	}
	if mustPathKind(t, unitPath) != PathDir {
		t.Fatal("unit directory removed even though the request step (which precedes it) failed")
	}
	if mustPathKind(t, ReleasedPath(storeRoot, id)) != PathRegular {
		t.Fatal(".released removed even though the request step (which precedes it) failed")
	}
	lockAbsent(t, storeRoot, id)

	mxSetImmutable(t, requestPath, false)
	res2 := decideUnit(context.Background(), storeRoot, repo.Dir, id, NewGitReconciler(storeRoot))
	if res2.Outcome != Reclaimed {
		t.Fatalf("re-entrant run: Outcome = %v (detail=%q), want Reclaimed once the obstacle is cleared", res2.Outcome, res2.Detail)
	}
	for _, p := range []string{RequestStagingPath(storeRoot, id), requestPath, unitPath, ReleasedPath(storeRoot, id), LockPath(storeRoot, id)} {
		if mustPathKind(t, p) != PathAbsent {
			t.Fatalf("%s still present after the re-entrant finish", p)
		}
	}
}

// TestDecideUnit_Rank5_DeletionFailureBoundary_ReleasedStep forces the
// FOURTH fixed-order step (.released) to fail, AFTER the directory step
// has already succeeded — proving spec §Safe cleanup's own named landing:
// "a failure at the .released or .lock step leaves siblings with no
// directory, which is rank 0's orphaned metadata".
func TestDecideUnit_Rank5_DeletionFailureBoundary_ReleasedStep(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	id := gcUnitID(0)
	unitPath := cutGCUnit(t, repo, storeRoot, id)
	markReleased(t, storeRoot, id)
	releasedPath := ReleasedPath(storeRoot, id)
	mxSetImmutable(t, releasedPath, true)
	defer mxClearImmutableBestEffort(releasedPath)

	res := decideUnit(context.Background(), storeRoot, repo.Dir, id, NewGitReconciler(storeRoot))
	if res.Outcome != Partial {
		t.Fatalf("Outcome = %v (detail=%q), want Partial (released step forbidden by immutability)", res.Outcome, res.Detail)
	}
	if !strings.Contains(res.Detail, "released") {
		t.Fatalf("Detail = %q, want it to NAME the released-marker step specifically", res.Detail)
	}
	// Landing named by the spec: siblings with no directory.
	if mustPathKind(t, unitPath) != PathAbsent {
		t.Fatal("unit directory survives even though the directory step (which precedes released) should have already succeeded")
	}
	if mustPathKind(t, RequestPath(storeRoot, id)) != PathAbsent {
		t.Fatal(".request survives — test fixture invalid")
	}
	if mustPathKind(t, RequestStagingPath(storeRoot, id)) != PathAbsent {
		t.Fatal(".request.staging survives — test fixture invalid")
	}
	if mustPathKind(t, releasedPath) != PathRegular {
		t.Fatal(".released removed despite immutability (test setup invalid)")
	}
	lockAbsent(t, storeRoot, id) // the lock's own release is unaffected by released's immutability

	// Later run: with no directory left, this is rank 0's shape now
	// (orphaned sibling, no unit path) — the finishing outcome is
	// ReclaimOrphaned, not Reclaimed, exactly matching the spec's own
	// named landing.
	mxSetImmutable(t, releasedPath, false)
	res2 := decideUnit(context.Background(), storeRoot, repo.Dir, id, NewGitReconciler(storeRoot))
	if res2.Outcome != ReclaimOrphaned {
		t.Fatalf("re-entrant run: Outcome = %v (detail=%q), want ReclaimOrphaned (siblings with no directory — rank 0's own shape)", res2.Outcome, res2.Detail)
	}
	if mustPathKind(t, releasedPath) != PathAbsent {
		t.Fatal(".released still present after the re-entrant finish")
	}
	lockAbsent(t, storeRoot, id)
}

// TestDecideUnit_Rank5_DeletionFailureBoundary_LockStep forces the FIFTH
// and LAST fixed-order step — the fused lock-deletion/release — to fail,
// AFTER staging/request/directory/released have all already succeeded:
// the same "siblings with no directory" landing the spec names for
// ".released or .lock", this time with .lock as the sole survivor.
//
// The lock file does not exist until rank 5's own acquisition creates it,
// so this test uses gcHookAfterLockForTests (the same seam
// TestDecideUnit_Rank0_ReDerivation_* and TestDecideUnit_Rank5_ReDerivation_*
// already use) to flag it immutable in the one window that exists for
// it: immediately after acquisition, before the fixed deletion sequence
// runs.
func TestDecideUnit_Rank5_DeletionFailureBoundary_LockStep(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	id := gcUnitID(0)
	unitPath := cutGCUnit(t, repo, storeRoot, id)
	markReleased(t, storeRoot, id)
	lockPath := LockPath(storeRoot, id)

	fired := false
	gcHookAfterLockForTests = func(gotID string) {
		if fired || gotID != id {
			return
		}
		fired = true
		mxSetImmutable(t, lockPath, true)
	}
	defer func() { gcHookAfterLockForTests = nil }()
	defer mxClearImmutableBestEffort(lockPath)

	res := decideUnit(context.Background(), storeRoot, repo.Dir, id, NewGitReconciler(storeRoot))
	if !fired {
		t.Fatal("test hook never fired — test did not exercise the lock-deletion step")
	}
	if res.Outcome != Partial {
		t.Fatalf("Outcome = %v (detail=%q), want Partial (lock-deletion step forbidden by immutability)", res.Outcome, res.Detail)
	}
	if !strings.Contains(res.Detail, "lock") {
		t.Fatalf("Detail = %q, want it to NAME the lock step specifically", res.Detail)
	}
	if mustPathKind(t, unitPath) != PathAbsent {
		t.Fatal("unit directory survives even though it precedes the lock step")
	}
	if mustPathKind(t, ReleasedPath(storeRoot, id)) != PathAbsent {
		t.Fatal(".released survives even though it precedes the lock step")
	}
	if mustPathKind(t, lockPath) != PathRegular {
		t.Fatal("lock file removed despite immutability (test setup invalid)")
	}

	// Later run: clear the OS-level obstacle AND remove the wedged lock
	// file directly. Unlike the other three steps, this one cannot be
	// "resolved by simply re-running in this same process": the lock
	// body's recorded holder is THIS test process's own live pid
	// (filelock.Acquire always embeds os.Getpid()), so a bare re-run
	// would read it as held by a live process — itself — rather than
	// stale, and would never self-heal within one test binary. Clearing
	// the flag and removing the file directly is the honest proxy for
	// "a later process observes and clears a wedged lock" (or simply the
	// passage of enough time for the SAME process to have exited, in
	// production), exactly what "permissions restored" means for the one
	// step whose obstacle is entangled with this test's own liveness.
	mxSetImmutable(t, lockPath, false)
	if err := os.Remove(lockPath); err != nil {
		t.Fatalf("removing wedged lock file during restore: %v", err)
	}

	res2 := decideUnit(context.Background(), storeRoot, repo.Dir, id, NewGitReconciler(storeRoot))
	if res2.Outcome != ReclaimOrphaned {
		t.Fatalf("re-entrant run: Outcome = %v (detail=%q), want ReclaimOrphaned (nothing left but a surviving `git worktree add` registration — rank 0's own shape)", res2.Outcome, res2.Detail)
	}
	lockAbsent(t, storeRoot, id)
}
