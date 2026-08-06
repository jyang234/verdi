package execworkspace

// Tests for spec/execution-workspace §GC slice's execution gc slice (gc.go).
// Fixtures reuse this package's own established helpers: newReconcileTestRepo/
// newExecutionStoreRoot/adminWorktreesDir/soleAdminEntry/hashAdminDir/
// cutWorktree (reconcile_test.go) and lockAbsent (materialize_test.go) — no
// copy-pasted fixture logic, hermetic (fixturegit, t.TempDir, no network).
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/filelock"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// --- fixture helpers local to this file ---

// gcUnitID builds a syntactically valid <workspace-id> (ValidWorkspaceID)
// for test n, distinct across a single test's own units.
func gcUnitID(n int) string {
	return "u" + string(rune('a'+n)) + "--0123456789ab"
}

// cutGCUnit cuts a REAL, registered git worktree at workspaceID's unit path
// (reconcile_test.go's cutWorktree, reused directly) — the on-disk/registry
// shape gc's decisions read, regardless of how it got there (gc never cares
// whether a unit was produced by the real Materializer or built directly
// for a test, only what is currently on disk and registered).
func cutGCUnit(t *testing.T, repo *fixturegit.Repo, storeRoot, workspaceID string) string {
	t.Helper()
	unitPath := UnitPath(storeRoot, workspaceID)
	cutWorktree(t, repo.Dir, unitPath)
	return unitPath
}

// markReleased creates workspaceID's .released marker directly (this
// file's tests only need the marker's PRESENCE, never its content — the
// same "existence is the entire record" contract release.go itself
// documents).
func markReleased(t *testing.T, storeRoot, workspaceID string) {
	t.Helper()
	f, err := os.OpenFile(ReleasedPath(storeRoot, workspaceID), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("creating .released marker: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("closing .released marker: %v", err)
	}
}

func mustPathKind(t *testing.T, path string) PathKind {
	t.Helper()
	kind, err := LstatType(path)
	if err != nil {
		t.Fatalf("LstatType(%s): %v", path, err)
	}
	return kind
}

// writeDeadPIDLock writes a lock body naming a guaranteed-dead pid directly
// at path (filelock_test.go's own TestAcquire_TakeoverAfterDeadPID
// pattern, reused here since this package cannot import that _test.go
// file): a lone stale .lock, eligible for Acquire's own stale takeover.
func writeDeadPIDLock(t *testing.T, path string) {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatalf("running short-lived child: %v", err)
	}
	info := filelock.Info{PID: cmd.Process.Pid, Start: time.Now().Add(-time.Hour).Unix()}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshaling dead-pid lock info: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing dead-pid lock: %v", err)
	}
}

// hashTreeExcluding walks root (lstat kind + regular-file content, like
// hashAdminDir) into one deterministic digest, skipping any path under
// excluded — used to prove a gc call touched NOTHING outside the specific
// unit paths it was entitled to mutate.
func hashTreeExcluding(t *testing.T, root string, excluded []string) string {
	t.Helper()
	h := sha256.New()
	var paths []string
	if err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		for _, ex := range excluded {
			if p == ex || (len(p) > len(ex) && p[:len(ex)+1] == ex+string(filepath.Separator)) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		paths = append(paths, p)
		return nil
	}); err != nil {
		t.Fatalf("WalkDir(%s): %v", root, err)
	}
	sort.Strings(paths)
	for _, p := range paths {
		rel, _ := filepath.Rel(root, p)
		h.Write([]byte(rel))
		fi, serr := os.Lstat(p)
		if serr != nil {
			t.Fatalf("lstat %s: %v", p, serr)
		}
		h.Write([]byte{byte(fi.Mode() & os.ModeType >> 24)})
		if fi.Mode().IsRegular() {
			data, rerr := os.ReadFile(p)
			if rerr != nil {
				t.Fatalf("read %s: %v", p, rerr)
			}
			h.Write(data)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// --- GCOutcome: closed vocabulary ---

func TestGCOutcome_String_ClosedSetDistinctNonEmpty(t *testing.T) {
	seen := map[string]bool{}
	for o := GCOutcomeUnknown; o < numGCOutcomes; o++ {
		s := o.String()
		if s == "" {
			t.Fatalf("GCOutcome(%d).String() is empty", o)
		}
		if seen[s] {
			t.Fatalf("GCOutcome(%d).String() = %q duplicates an earlier value", o, s)
		}
		seen[s] = true
	}
}

func TestGCOutcome_String_OutOfRange_FailsClosed(t *testing.T) {
	got := GCOutcome(999).String()
	if got == "" {
		t.Fatal("out-of-range GCOutcome.String() is empty, want a self-naming fallback")
	}
}

// --- classifyResolvedAdminPath: pure function, table test ---

func TestClassifyResolvedAdminPath(t *testing.T) {
	const root = "/store/.verdi/data/execution"
	cases := []struct {
		name          string
		resolved      string
		wantID        string
		wantIsUnit    bool
		wantUnderRoot bool
	}{
		{"direct child, valid id", root + "/run--0123456789ab", "run--0123456789ab", true, true},
		{"direct child, invalid id shape", root + "/not-a-unit-id", "", false, true},
		{"nested location", root + "/run--0123456789ab/nested", "", false, true},
		{"outside root entirely", "/store/.verdi/data/worktrees/foo", "", false, false},
		{"root itself", root, "", false, true},
		{"empty resolved", "", "", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id, isUnit, underRoot := classifyResolvedAdminPath(c.resolved, root)
			if id != c.wantID || isUnit != c.wantIsUnit || underRoot != c.wantUnderRoot {
				t.Fatalf("classifyResolvedAdminPath(%q, %q) = (%q, %v, %v), want (%q, %v, %v)",
					c.resolved, root, id, isUnit, underRoot, c.wantID, c.wantIsUnit, c.wantUnderRoot)
			}
		})
	}
}

// --- rank 0: nothing at all at the unit path ---

func TestDecideUnit_Rank0_OrphanedSiblingsWithoutDir_ReclaimOrphaned(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	id := gcUnitID(0)
	unitPath := cutGCUnit(t, repo, storeRoot, id)
	markReleased(t, storeRoot, id)
	// Simulate a partial prior reclaim: directory gone, siblings (.released,
	// the real registration) still present.
	if err := os.RemoveAll(unitPath); err != nil {
		t.Fatalf("RemoveAll(unitPath): %v", err)
	}

	reconciler := NewGitReconciler(storeRoot)
	res := decideUnit(context.Background(), storeRoot, repo.Dir, id, reconciler)
	if res.Outcome != ReclaimOrphaned {
		t.Fatalf("Outcome = %v (detail=%q), want ReclaimOrphaned", res.Outcome, res.Detail)
	}
	if mustPathKind(t, ReleasedPath(storeRoot, id)) != PathAbsent {
		t.Fatal(".released sibling still present after reclaim-orphaned")
	}
	lockAbsent(t, storeRoot, id)

	adminDir := adminWorktreesDir(t, repo.Dir)
	matches, err := scanAdminDir(adminDir, canonicalPath(unitPath))
	if err != nil {
		t.Fatalf("scanAdminDir: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("registration survives reclaim-orphaned: %v", matches)
	}
}

func TestDecideUnit_Rank0_RegistryOnlyUnit_ReconciliationIsTheWholeAction(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	id := gcUnitID(0)
	unitPath := cutGCUnit(t, repo, storeRoot, id)
	// Nothing at all on disk for this unit: no directory, no siblings —
	// the filesystem alone could never surface this state.
	if err := os.RemoveAll(unitPath); err != nil {
		t.Fatalf("RemoveAll(unitPath): %v", err)
	}

	adminDir := adminWorktreesDir(t, repo.Dir)
	before, err := scanAdminDir(adminDir, canonicalPath(unitPath))
	if err != nil || len(before) != 1 {
		t.Fatalf("setup sanity: want exactly 1 registry entry, got %v (err=%v)", before, err)
	}

	reconciler := NewGitReconciler(storeRoot)
	res := decideUnit(context.Background(), storeRoot, repo.Dir, id, reconciler)
	if res.Outcome != ReclaimOrphaned {
		t.Fatalf("Outcome = %v (detail=%q), want ReclaimOrphaned", res.Outcome, res.Detail)
	}
	after, err := scanAdminDir(adminDir, canonicalPath(unitPath))
	if err != nil {
		t.Fatalf("scanAdminDir: %v", err)
	}
	if len(after) != 0 {
		t.Fatalf("registry entry survives registry-only reclaim: %v", after)
	}
	lockAbsent(t, storeRoot, id)
}

func TestDecideUnit_Rank0_LstatFailureAtUnitPath_KeepMalformed(t *testing.T) {
	storeRoot := t.TempDir()
	// Plant a REGULAR FILE where the execution root directory must be, so
	// lstat of any path underneath it fails with ENOTDIR — never a clean
	// not-found.
	if err := os.MkdirAll(filepath.Dir(ExecutionRoot(storeRoot)), 0o755); err != nil {
		t.Fatalf("preparing parent of execution root: %v", err)
	}
	if err := os.WriteFile(ExecutionRoot(storeRoot), []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("planting non-dir execution root: %v", err)
	}
	res := decideUnit(context.Background(), storeRoot, t.TempDir(), gcUnitID(0), NewGitReconciler(storeRoot))
	if res.Outcome != KeepMalformed {
		t.Fatalf("Outcome = %v (detail=%q), want KeepMalformed (lstat failure never read as absence)", res.Outcome, res.Detail)
	}
}

func TestDecideUnit_Rank0_LoneLiveLock_KeepLocked(t *testing.T) {
	storeRoot := newExecutionStoreRoot(t)
	id := gcUnitID(0)
	lockPath := LockPath(storeRoot, id)
	f, err := filelock.Acquire(lockPath)
	if err != nil {
		t.Fatalf("test-side Acquire (simulating a live materialization-in-flight holder): %v", err)
	}
	defer func() { _ = filelock.Release(f, lockPath) }()

	res := decideUnit(context.Background(), storeRoot, t.TempDir(), id, NewGitReconciler(storeRoot))
	if res.Outcome != KeepLocked {
		t.Fatalf("Outcome = %v (detail=%q), want KeepLocked", res.Outcome, res.Detail)
	}
	if mustPathKind(t, lockPath) == PathAbsent {
		t.Fatal(".lock deleted out from under its live holder")
	}
}

func TestDecideUnit_Rank0_LoneStaleLock_TakenOverThenReclaimOrphaned(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	id := gcUnitID(0)
	writeDeadPIDLock(t, LockPath(storeRoot, id))

	res := decideUnit(context.Background(), storeRoot, repo.Dir, id, NewGitReconciler(storeRoot))
	if res.Outcome != ReclaimOrphaned {
		t.Fatalf("Outcome = %v (detail=%q), want ReclaimOrphaned (stale lock taken over, nothing else on disk)", res.Outcome, res.Detail)
	}
	lockAbsent(t, storeRoot, id)
}

func TestDecideUnit_Rank0_ReconciliationRefusal_Partial(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	id := gcUnitID(0)
	unitPath := cutGCUnit(t, repo, storeRoot, id)
	if err := os.RemoveAll(unitPath); err != nil {
		t.Fatalf("RemoveAll(unitPath): %v", err)
	}
	adminDir := adminWorktreesDir(t, repo.Dir)
	entry := soleAdminEntry(t, adminDir)
	if err := os.WriteFile(filepath.Join(adminDir, entry, "locked"), nil, 0o644); err != nil {
		t.Fatalf("planting git lock marker: %v", err)
	}

	res := decideUnit(context.Background(), storeRoot, repo.Dir, id, NewGitReconciler(storeRoot))
	if res.Outcome != Partial {
		t.Fatalf("Outcome = %v (detail=%q), want Partial (reconciliation refused on a lock marker)", res.Outcome, res.Detail)
	}
	// Own lock released even though the flow bailed before its fused
	// deletion: the acquiring holder never leaks its own hold.
	lockAbsent(t, storeRoot, id)
}

func TestGC_Rank0Partial_SweepContinuesToSecondUnit(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)

	lockedID := gcUnitID(0)
	lockedUnitPath := cutGCUnit(t, repo, storeRoot, lockedID)
	if err := os.RemoveAll(lockedUnitPath); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	adminDir := adminWorktreesDir(t, repo.Dir)
	lockedEntry := soleAdminEntry(t, adminDir)
	if err := os.WriteFile(filepath.Join(adminDir, lockedEntry, "locked"), nil, 0o644); err != nil {
		t.Fatalf("planting git lock marker: %v", err)
	}

	cleanID := gcUnitID(1)
	cutGCUnit(t, repo, storeRoot, cleanID)
	markReleased(t, storeRoot, cleanID)

	results, _, err := GC(context.Background(), storeRoot, repo.Dir)
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	byID := map[string]GCResult{}
	for _, r := range results {
		byID[r.WorkspaceID] = r
	}
	if byID[lockedID].Outcome != Partial {
		t.Fatalf("locked unit outcome = %v, want Partial", byID[lockedID].Outcome)
	}
	if byID[cleanID].Outcome != Reclaimed {
		t.Fatalf("clean unit outcome = %v, want Reclaimed (sweep must continue past the partial)", byID[cleanID].Outcome)
	}
}

// --- rank 1: malformed ---

func TestDecideUnit_Rank1_NonDirAtUnitPath_KeepMalformed(t *testing.T) {
	storeRoot := newExecutionStoreRoot(t)
	id := gcUnitID(0)
	if err := os.WriteFile(UnitPath(storeRoot, id), []byte("x"), 0o644); err != nil {
		t.Fatalf("planting regular file at unit path: %v", err)
	}
	res := decideUnit(context.Background(), storeRoot, t.TempDir(), id, NewGitReconciler(storeRoot))
	if res.Outcome != KeepMalformed {
		t.Fatalf("Outcome = %v, want KeepMalformed", res.Outcome)
	}
}

func TestDecideUnit_Rank1_NonRegularMarker_KeepMalformed(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	id := gcUnitID(0)
	cutGCUnit(t, repo, storeRoot, id)
	if err := os.Symlink(t.TempDir(), ReleasedPath(storeRoot, id)); err != nil {
		t.Fatalf("planting symlink at marker path: %v", err)
	}
	res := decideUnit(context.Background(), storeRoot, repo.Dir, id, NewGitReconciler(storeRoot))
	if res.Outcome != KeepMalformed {
		t.Fatalf("Outcome = %v, want KeepMalformed (symlink at marker path, never followed)", res.Outcome)
	}
}

// --- rank 2: not yet released ---

func TestDecideUnit_Rank2_NoMarker_KeepNotEligible(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	id := gcUnitID(0)
	cutGCUnit(t, repo, storeRoot, id)
	res := decideUnit(context.Background(), storeRoot, repo.Dir, id, NewGitReconciler(storeRoot))
	if res.Outcome != KeepNotEligible {
		t.Fatalf("Outcome = %v, want KeepNotEligible", res.Outcome)
	}
}

// --- rank 3: dirty ---

func TestDecideUnit_Rank3_ReleasedButDirty_KeepDirty_OrderProven(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	id := gcUnitID(0)
	unitPath := cutGCUnit(t, repo, storeRoot, id)
	markReleased(t, storeRoot, id)
	if err := os.WriteFile(filepath.Join(unitPath, "uncommitted.txt"), []byte("wip\n"), 0o644); err != nil {
		t.Fatalf("dirtying unit worktree: %v", err)
	}
	res := decideUnit(context.Background(), storeRoot, repo.Dir, id, NewGitReconciler(storeRoot))
	if res.Outcome != KeepDirty {
		t.Fatalf("Outcome = %v (detail=%q), want KeepDirty (released AND dirty proves dirty is checked, and wins, after eligibility)", res.Outcome, res.Detail)
	}
	if mustPathKind(t, unitPath) == PathAbsent {
		t.Fatal("dirty unit was deleted")
	}
}

func TestDecideUnit_Rank3_StatusDirtyError_KeepDirty_FailClosed(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	id := gcUnitID(0)
	unitPath := cutGCUnit(t, repo, storeRoot, id)
	markReleased(t, storeRoot, id)
	// Break the worktree's own git linkage so `git status` fails outright —
	// StatusDirty becomes unevaluable, not merely false.
	if err := os.WriteFile(filepath.Join(unitPath, ".git"), []byte("garbage, not a real gitdir record\n"), 0o644); err != nil {
		t.Fatalf("corrupting worktree .git linkage: %v", err)
	}
	res := decideUnit(context.Background(), storeRoot, repo.Dir, id, NewGitReconciler(storeRoot))
	if res.Outcome != KeepDirty {
		t.Fatalf("Outcome = %v (detail=%q), want KeepDirty (unevaluable predicate keeps at its own rank, fail-closed)", res.Outcome, res.Detail)
	}
	if res.Detail == "" {
		t.Fatal("unevaluable-predicate keep must name the failed check in Detail")
	}
}

// --- rank 4: locked ---

func TestDecideUnit_Rank4_LiveHolder_KeepLocked(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	id := gcUnitID(0)
	cutGCUnit(t, repo, storeRoot, id)
	markReleased(t, storeRoot, id)

	lockPath := LockPath(storeRoot, id)
	f, err := filelock.Acquire(lockPath)
	if err != nil {
		t.Fatalf("test-side Acquire: %v", err)
	}
	defer func() { _ = filelock.Release(f, lockPath) }()

	res := decideUnit(context.Background(), storeRoot, repo.Dir, id, NewGitReconciler(storeRoot))
	if res.Outcome != KeepLocked {
		t.Fatalf("Outcome = %v, want KeepLocked", res.Outcome)
	}
}

func TestDecideUnit_Rank4_UndecodableLockBody_KeepLocked(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	id := gcUnitID(0)
	cutGCUnit(t, repo, storeRoot, id)
	markReleased(t, storeRoot, id)
	if err := os.WriteFile(LockPath(storeRoot, id), []byte("not-json-garbage"), 0o644); err != nil {
		t.Fatalf("planting undecodable lock body: %v", err)
	}
	res := decideUnit(context.Background(), storeRoot, repo.Dir, id, NewGitReconciler(storeRoot))
	if res.Outcome != KeepLocked {
		t.Fatalf("Outcome = %v (detail=%q), want KeepLocked (undecodable body is 'any other acquisition failure')", res.Outcome, res.Detail)
	}
}

// --- rank 5: reclaim ---

func TestDecideUnit_Rank5_ReleasedCleanUnlocked_Reclaimed_AllFivePathsGone(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	id := gcUnitID(0)
	unitPath := cutGCUnit(t, repo, storeRoot, id)
	markReleased(t, storeRoot, id)
	if err := os.WriteFile(RequestPath(storeRoot, id), []byte("witness"), 0o644); err != nil {
		t.Fatalf("planting .request: %v", err)
	}

	res := decideUnit(context.Background(), storeRoot, repo.Dir, id, NewGitReconciler(storeRoot))
	if res.Outcome != Reclaimed {
		t.Fatalf("Outcome = %v (detail=%q), want Reclaimed", res.Outcome, res.Detail)
	}
	for _, p := range []string{
		RequestStagingPath(storeRoot, id),
		RequestPath(storeRoot, id),
		unitPath,
		ReleasedPath(storeRoot, id),
		LockPath(storeRoot, id),
	} {
		if mustPathKind(t, p) != PathAbsent {
			t.Fatalf("%s still present after reclaim", p)
		}
	}
	// No double-unlink: the fused .lock deletion is the ONLY release call,
	// and it produced no error (proven above by Reclaimed with no Partial).
	if want := "execution: reclaimed: " + id + " (any surviving registration is resolved by a later gc)"; res.Line() != want {
		t.Fatalf("Line() = %q, want %q", res.Line(), want)
	}
}

// A released, clean unit directory that was NEVER registered as a worktree
// of repo.Dir still reaches rank 5 and reclaims — so no surviving
// registration exists for a later gc to resolve. rank 5 performs no
// verification of that fact either way (its five fixed deletions are the
// whole action), so the disclosed line may only CONDITION the claim, never
// assert it.
//
// The unit is a REAL linked worktree of a SECOND repository, not a
// standalone `git init` at the unit path: rank 3's evaluability guard
// (whole-wave finding F1) requires a linked worktree's regular-file `.git`
// before it will consult gitx.StatusDirty at all, and a standalone repo's
// `.git` DIRECTORY leaves the predicate unevaluable-for-this-unit. Cutting
// from another repository keeps the property this test is actually about —
// clean, released, and absent from repo.Dir's registry — while the dirty
// predicate stays genuinely evaluable.
func TestDecideUnit_Rank5_UnregisteredUnit_LineMakesNoUnconditionalRegistrationClaim(t *testing.T) {
	repo := newReconcileTestRepo(t)
	otherRepo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	id := gcUnitID(0)
	unitPath := UnitPath(storeRoot, id)
	cutWorktree(t, otherRepo.Dir, unitPath)
	markReleased(t, storeRoot, id)

	res := decideUnit(context.Background(), storeRoot, repo.Dir, id, NewGitReconciler(storeRoot))
	if res.Outcome != Reclaimed {
		t.Fatalf("Outcome = %v (detail=%q), want Reclaimed", res.Outcome, res.Detail)
	}
	line := res.Line()
	if strings.Contains(line, "registration remains") {
		t.Fatalf("Line() = %q asserts a surviving registration as fact, but this unit was never registered", line)
	}
	if !strings.Contains(line, "any surviving registration is resolved by a later gc") {
		t.Fatalf("Line() = %q, want the conditional registration disclosure", line)
	}
}

func TestDecideUnit_Rank5_PartialAtDirectoryStep_ThenReEntrantFinish(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores permission bits; this test needs a non-root permission failure")
	}
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	id := gcUnitID(0)
	unitPath := cutGCUnit(t, repo, storeRoot, id)
	markReleased(t, storeRoot, id)
	// .request.staging never existed; .request is absent too, so those two
	// fixed-order steps are no-ops needing no write permission — isolating
	// the failure to the directory-removal step specifically. Only the
	// UNIT directory itself loses write permission (never the execution
	// root, which the lock file and sibling forms also live directly
	// under and must stay usable): with no write bit on unitPath, os.
	// RemoveAll can still lstat/read its way in but cannot unlink any of
	// its own children, so it fails cleanly at the directory step alone.
	if err := os.Chmod(unitPath, 0o555); err != nil {
		t.Fatalf("chmod unit directory read-only: %v", err)
	}
	restored := false
	restore := func() {
		if restored {
			return
		}
		restored = true
		if err := os.Chmod(unitPath, 0o755); err != nil {
			t.Fatalf("restoring unit directory permissions: %v", err)
		}
	}
	defer restore()

	res := decideUnit(context.Background(), storeRoot, repo.Dir, id, NewGitReconciler(storeRoot))
	if res.Outcome != Partial {
		t.Fatalf("Outcome = %v (detail=%q), want Partial (directory step forbidden by permissions)", res.Outcome, res.Detail)
	}
	if mustPathKind(t, unitPath) == PathAbsent {
		t.Fatal("unit directory removed despite permission failure (test setup invalid)")
	}
	if mustPathKind(t, ReleasedPath(storeRoot, id)) != PathRegular {
		t.Fatal(".released marker deleted before the directory step even though the directory step failed — must not skip ahead")
	}

	restore()
	res2 := decideUnit(context.Background(), storeRoot, repo.Dir, id, NewGitReconciler(storeRoot))
	if res2.Outcome != Reclaimed {
		t.Fatalf("re-entrant run: Outcome = %v (detail=%q), want Reclaimed once permissions are restored", res2.Outcome, res2.Detail)
	}
	if mustPathKind(t, unitPath) != PathAbsent {
		t.Fatal("unit directory still present after the re-entrant finish")
	}
}

// --- re-derivation: classify-then-invalidate, exercised via the test-only hook ---

func TestDecideUnit_Rank0_ReDerivation_UnitMaterializesUnderTheLock(t *testing.T) {
	storeRoot := newExecutionStoreRoot(t)
	repo := newReconcileTestRepo(t)
	id := gcUnitID(0)
	unitPath := UnitPath(storeRoot, id)

	fired := false
	gcHookAfterLockForTests = func(gotID string) {
		if fired || gotID != id {
			return
		}
		fired = true
		// Simulate a concurrent materialization landing between
		// classification (unit path was absent) and this gate's lock
		// acquisition: the unit path now exists.
		if err := os.MkdirAll(unitPath, 0o755); err != nil {
			t.Fatalf("simulating concurrent materialization: %v", err)
		}
	}
	defer func() { gcHookAfterLockForTests = nil }()

	res := decideUnit(context.Background(), storeRoot, repo.Dir, id, NewGitReconciler(storeRoot))
	if !fired {
		t.Fatal("test hook never fired — test did not exercise the re-derivation branch")
	}
	// The stale rank-0 classification (reclaim-orphaned) must NEVER be
	// applied to a unit that now has a real directory with no marker: the
	// re-decided outcome is rank 2's keep-not-eligible.
	if res.Outcome != KeepNotEligible {
		t.Fatalf("Outcome = %v (detail=%q), want KeepNotEligible (re-derived, never applied)", res.Outcome, res.Detail)
	}
	if mustPathKind(t, unitPath) != PathDir {
		t.Fatal("re-derivation incorrectly deleted the concurrently-materialized directory")
	}
}

func TestDecideUnit_Rank5_ReDerivation_MarkerRemovedUnderTheLock(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	id := gcUnitID(0)
	unitPath := cutGCUnit(t, repo, storeRoot, id)
	markReleased(t, storeRoot, id)

	fired := false
	gcHookAfterLockForTests = func(gotID string) {
		if fired || gotID != id {
			return
		}
		fired = true
		if err := os.Remove(ReleasedPath(storeRoot, id)); err != nil {
			t.Fatalf("simulating a concurrent .released removal: %v", err)
		}
	}
	defer func() { gcHookAfterLockForTests = nil }()

	res := decideUnit(context.Background(), storeRoot, repo.Dir, id, NewGitReconciler(storeRoot))
	if !fired {
		t.Fatal("test hook never fired")
	}
	if res.Outcome != KeepNotEligible {
		t.Fatalf("Outcome = %v (detail=%q), want KeepNotEligible (re-derived: no longer released, never reclaimed anyway)", res.Outcome, res.Detail)
	}
	if mustPathKind(t, unitPath) == PathAbsent {
		t.Fatal("re-derivation incorrectly reclaimed a unit whose marker was invalidated under the lock")
	}
}

// --- rank 3: the dirty predicate's own evaluability guard ---

// TestGC_ReleasedPlainDirectory_UnderGitignoredStore_KeptUnevaluable is the
// probe for whole-wave finding F1, built in the EXACT production shape
// `verdi gc` runs in (cmd/verdi's own execworkspace.GC(ctx, root, root)):
// storeRoot IS repoRoot, a real git repository, with `.verdi/.gitignore`
// naming `data/` so the whole data zone is invisible to `git status`.
//
// A released ABANDONED PARTIAL — a plain directory at the unit path that was
// never linked as a worktree (spec §GC slice: "Release may be invoked for an
// ABANDONED run regardless of how complete its materialization is") — has no
// `.git` of its own, so `gitx.StatusDirty` run inside it answers from the
// PARENT repository, which is clean because the data zone is gitignored.
// The predicate's answer is therefore about a DIFFERENT tree than the unit,
// and reading it as "this unit is clean" deletes a directory whose
// cleanliness was never established. §Safe cleanup's "Keep dirty, locked,
// ambiguous, or unverifiably eligible workspaces" and §GC slice's "a partial
// worktree whose cleanliness cannot be proven KEEPS, disclosed" both require
// the keep.
func TestGC_ReleasedPlainDirectory_UnderGitignoredStore_KeptUnevaluable(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml": "schema: verdi.layout/v1\n",
			".verdi/.gitignore": "data/\n",
		},
		Message: "store root",
	}})
	root := repo.Dir
	if err := os.MkdirAll(ExecutionRoot(root), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", ExecutionRoot(root), err)
	}
	id := gcUnitID(0)
	unitPath := UnitPath(root, id)
	if err := os.MkdirAll(unitPath, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", unitPath, err)
	}
	// Content a human would lose if the unevaluable predicate were read as
	// "clean": exactly the abandoned-partial residue the spec keeps.
	if err := os.WriteFile(filepath.Join(unitPath, "abandoned-work.txt"), []byte("unproven\n"), 0o644); err != nil {
		t.Fatalf("planting abandoned residue: %v", err)
	}
	markReleased(t, root, id)

	// Setup sanity: the parent repository really is clean, so an unguarded
	// StatusDirty against the unit path really would report "clean" here.
	if dirty, derr := gitx.StatusDirty(context.Background(), unitPath); derr != nil {
		t.Fatalf("setup sanity: StatusDirty inside the plain directory failed (%v) — this probe needs it to SUCCEED and answer from the parent repo", derr)
	} else if dirty {
		t.Fatal("setup sanity: parent repository is dirty — this probe needs a clean parent so the misread would be 'clean'")
	}

	results, _, err := GC(context.Background(), root, root)
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	res := mustFindResult(t, results, id)
	if res.Outcome != KeepDirty {
		t.Fatalf("Outcome = %v (detail=%q), want KeepDirty (the dirty predicate is UNEVALUABLE for a unit that is not a linked worktree; it keeps at its own rank, fail-closed)", res.Outcome, res.Detail)
	}
	if !strings.Contains(res.Detail, "dirty check unevaluable: unit path is not a linked worktree") {
		t.Fatalf("Detail = %q, want the disclosure naming the check that could not be evaluated", res.Detail)
	}
	if mustPathKind(t, unitPath) != PathDir {
		t.Fatal("released plain directory was deleted with its cleanliness never established")
	}
	if mustPathKind(t, filepath.Join(unitPath, "abandoned-work.txt")) != PathRegular {
		t.Fatal("abandoned residue inside the kept directory was destroyed")
	}
	if mustPathKind(t, ReleasedPath(root, id)) != PathRegular {
		t.Fatal(".released marker deleted for a kept unit")
	}
}

// TestDecideUnit_Rank3_UnevaluableKinds_KeptWithDisclosure covers the guard's
// remaining "anything else at .git" cases — absent, directory, and symlink —
// each of which leaves the dirty predicate unevaluable AS A STATEMENT ABOUT
// THIS UNIT and must keep at rank 3 with the disclosure naming the kind.
func TestDecideUnit_Rank3_UnevaluableKinds_KeptWithDisclosure(t *testing.T) {
	cases := []struct {
		name     string
		plant    func(t *testing.T, unitPath string)
		wantKind string
	}{
		{"absent", func(t *testing.T, unitPath string) {}, "absent"},
		{"directory", func(t *testing.T, unitPath string) {
			if err := os.MkdirAll(filepath.Join(unitPath, ".git"), 0o755); err != nil {
				t.Fatalf("planting .git directory: %v", err)
			}
		}, "dir"},
		{"symlink", func(t *testing.T, unitPath string) {
			if err := os.Symlink(t.TempDir(), filepath.Join(unitPath, ".git")); err != nil {
				t.Fatalf("planting .git symlink: %v", err)
			}
		}, "symlink"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newReconcileTestRepo(t)
			storeRoot := newExecutionStoreRoot(t)
			id := gcUnitID(0)
			unitPath := UnitPath(storeRoot, id)
			if err := os.MkdirAll(unitPath, 0o755); err != nil {
				t.Fatalf("MkdirAll(%s): %v", unitPath, err)
			}
			tc.plant(t, unitPath)
			markReleased(t, storeRoot, id)

			res := decideUnit(context.Background(), storeRoot, repo.Dir, id, NewGitReconciler(storeRoot))
			if res.Outcome != KeepDirty {
				t.Fatalf("Outcome = %v (detail=%q), want KeepDirty", res.Outcome, res.Detail)
			}
			want := "dirty check unevaluable: unit path is not a linked worktree (" + tc.wantKind + " at .git)"
			if res.Detail != want {
				t.Fatalf("Detail = %q, want %q", res.Detail, want)
			}
			if mustPathKind(t, unitPath) != PathDir {
				t.Fatal("unit kept at rank 3 was nevertheless deleted")
			}
		})
	}
}

// TestDecideUnit_Rank5_ReDerivation_LinkageBrokenUnderTheLock proves the
// SECOND site the guard must run at: rank 5 re-derives the whole decision
// under the acquired lock immediately before mutating, so a unit whose
// linked-worktree `.git` file is replaced inside that window must be
// re-decided into the rank-3 unevaluable keep, never reclaimed on the stale
// classification.
func TestDecideUnit_Rank5_ReDerivation_LinkageBrokenUnderTheLock(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	id := gcUnitID(0)
	unitPath := cutGCUnit(t, repo, storeRoot, id)
	markReleased(t, storeRoot, id)

	fired := false
	gcHookAfterLockForTests = func(gotID string) {
		if fired || gotID != id {
			return
		}
		fired = true
		// A real linked worktree's `.git` is a regular file. Replace it with
		// a directory inside the post-acquire window: the classification
		// that reached rank 5 no longer holds.
		gitLink := filepath.Join(unitPath, ".git")
		if err := os.Remove(gitLink); err != nil {
			t.Fatalf("removing worktree .git link: %v", err)
		}
		if err := os.MkdirAll(gitLink, 0o755); err != nil {
			t.Fatalf("planting .git directory under the lock: %v", err)
		}
	}
	defer func() { gcHookAfterLockForTests = nil }()

	res := decideUnit(context.Background(), storeRoot, repo.Dir, id, NewGitReconciler(storeRoot))
	if !fired {
		t.Fatal("test hook never fired — test did not exercise the re-derivation branch")
	}
	if res.Outcome != KeepDirty {
		t.Fatalf("Outcome = %v (detail=%q), want KeepDirty (re-derived under the lock: the dirty predicate became unevaluable)", res.Outcome, res.Detail)
	}
	if !strings.Contains(res.Detail, "dirty check unevaluable: unit path is not a linked worktree") {
		t.Fatalf("Detail = %q, want the unevaluable-predicate disclosure", res.Detail)
	}
	if mustPathKind(t, unitPath) != PathDir {
		t.Fatal("rank 5 reclaimed a unit whose dirty predicate became unevaluable under the lock")
	}
}

// --- scan set: grammar-external and administrative disclosures ---

// TestGC_ScanSet_NonDirectoryAdministrativeEntry_DisclosedNeverDeleted is
// whole-wave finding F5: a NON-DIRECTORY entry under
// $GIT_COMMON_DIR/worktrees/ names no unit and cannot be resolved, so it is
// the spec's disclosed-and-kept class ("one that CANNOT BE RESOLVED is
// likewise kept, because an entry that cannot be proven in scope is never
// ours to remove"), never a silent skip.
func TestGC_ScanSet_NonDirectoryAdministrativeEntry_DisclosedNeverDeleted(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	// A cut worktree guarantees $GIT_COMMON_DIR/worktrees/ exists.
	cutWorktree(t, repo.Dir, filepath.Join(t.TempDir(), "seed"))
	adminDir := adminWorktreesDir(t, repo.Dir)
	strayPath := filepath.Join(adminDir, "stray-regular-file")
	if err := os.WriteFile(strayPath, []byte("not a worktree entry\n"), 0o644); err != nil {
		t.Fatalf("planting stray administrative entry: %v", err)
	}

	results, disclosures, err := GC(context.Background(), storeRoot, repo.Dir)
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("results = %v, want none (a non-directory administrative entry names no unit)", results)
	}
	if len(disclosures) != 1 {
		t.Fatalf("disclosures = %v, want exactly 1 naming the stray entry", disclosures)
	}
	if !strings.Contains(disclosures[0], strayPath) || !strings.Contains(disclosures[0], "kept for human attention") {
		t.Fatalf("disclosure = %q, want the unclassified-administrative-entry disclosure naming %s", disclosures[0], strayPath)
	}
	if mustPathKind(t, strayPath) != PathRegular {
		t.Fatal("stray administrative entry was deleted")
	}
}

func TestGC_ScanSet_GrammarExternalEntries_DisclosedNeverDeleted(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	for _, name := range []string{"README", ".DS_Store"} {
		if err := os.WriteFile(filepath.Join(ExecutionRoot(storeRoot), name), []byte("x"), 0o644); err != nil {
			t.Fatalf("planting grammar-external entry %s: %v", name, err)
		}
	}

	results, disclosures, err := GC(context.Background(), storeRoot, repo.Dir)
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("results = %v, want none (grammar-external entries name no unit)", results)
	}
	if len(disclosures) != 2 {
		t.Fatalf("disclosures = %v, want exactly 2", disclosures)
	}
	for _, name := range []string{"README", ".DS_Store"} {
		if mustPathKind(t, filepath.Join(ExecutionRoot(storeRoot), name)) == PathAbsent {
			t.Fatalf("grammar-external entry %s was deleted", name)
		}
		if mustPathKind(t, filepath.Join(ExecutionRoot(storeRoot), name+".lock")) != PathAbsent {
			t.Fatalf("a lock file was created for grammar-external entry %s", name)
		}
	}
}

func TestGC_ScanSet_AdminEntries_UnclassifiableDisclosed_ForeignUntouched(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)

	// Foreign worktree: resolves OUTSIDE data/execution/ — another slice's,
	// never touched, no disclosure.
	foreignPath := filepath.Join(t.TempDir(), "foreign")
	cutWorktree(t, repo.Dir, foreignPath)

	adminDir := adminWorktreesDir(t, repo.Dir)
	foreignEntry := soleAdminEntry(t, adminDir)
	beforeForeign := hashAdminDir(t, filepath.Join(adminDir, foreignEntry))

	// Unclassifiable entry: a broken gitdir record that resolves nowhere.
	unitPath := UnitPath(storeRoot, gcUnitID(0)) // never actually cut
	cutWorktree(t, repo.Dir, unitPath)
	var brokenEntry string
	for _, e := range readDirNames(t, adminDir) {
		if e != foreignEntry {
			brokenEntry = e
		}
	}
	if brokenEntry == "" {
		t.Fatal("setup sanity: could not find the second admin entry")
	}
	if err := os.WriteFile(filepath.Join(adminDir, brokenEntry, "gitdir"), []byte("garbage\x00not a path\n"), 0o644); err != nil {
		t.Fatalf("corrupting gitdir record: %v", err)
	}

	_, disclosures, err := GC(context.Background(), storeRoot, repo.Dir)
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	if len(disclosures) != 1 {
		t.Fatalf("disclosures = %v, want exactly 1 (only the unclassifiable entry)", disclosures)
	}

	afterForeign := hashAdminDir(t, filepath.Join(adminDir, foreignEntry))
	if beforeForeign != afterForeign {
		t.Fatalf("foreign (out-of-scope) admin entry mutated: before=%s after=%s", beforeForeign, afterForeign)
	}
}

func readDirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}

func TestGC_MissingExecutionRoot_NotAnError(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := t.TempDir() // no .verdi/data/execution/ at all
	results, disclosures, err := GC(context.Background(), storeRoot, repo.Dir)
	if err != nil {
		t.Fatalf("GC over a store with no execution root: %v", err)
	}
	if len(results) != 0 || len(disclosures) != 0 {
		t.Fatalf("results=%v disclosures=%v, want both empty", results, disclosures)
	}
}

// --- determinism ---

func TestGC_Determinism_TwoRunsIdenticalOrdering(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)

	// A mix of non-mutating outcomes only, so a second run observes
	// byte-identical state (mutating outcomes are exercised, and proven
	// idempotent/re-entrant, by the dedicated rank-0/rank-5 tests above).
	notEligibleID := gcUnitID(0)
	cutGCUnit(t, repo, storeRoot, notEligibleID)

	dirtyID := gcUnitID(1)
	dirtyPath := cutGCUnit(t, repo, storeRoot, dirtyID)
	markReleased(t, storeRoot, dirtyID)
	if err := os.WriteFile(filepath.Join(dirtyPath, "wip.txt"), []byte("wip\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	malformedID := gcUnitID(2)
	if err := os.WriteFile(UnitPath(storeRoot, malformedID), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(ExecutionRoot(storeRoot), "README"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	results1, disclosures1, err := GC(ctx, storeRoot, repo.Dir)
	if err != nil {
		t.Fatalf("GC run 1: %v", err)
	}
	results2, disclosures2, err := GC(ctx, storeRoot, repo.Dir)
	if err != nil {
		t.Fatalf("GC run 2: %v", err)
	}
	if len(results1) != 3 {
		t.Fatalf("results1 = %v, want 3 units", results1)
	}
	if !equalResults(results1, results2) {
		t.Fatalf("non-deterministic results:\nrun1=%v\nrun2=%v", results1, results2)
	}
	if !equalStrings(disclosures1, disclosures2) {
		t.Fatalf("non-deterministic disclosures:\nrun1=%v\nrun2=%v", disclosures1, disclosures2)
	}
	// Sorted workspace-id order.
	for i := 1; i < len(results1); i++ {
		if results1[i-1].WorkspaceID >= results1[i].WorkspaceID {
			t.Fatalf("results not sorted by workspace id: %v", results1)
		}
	}
}

func equalResults(a, b []GCResult) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- no deletion outside the unit ---

func TestGC_NoDeletionOutsideExactUnit(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)

	reclaimID := gcUnitID(0)
	reclaimPath := cutGCUnit(t, repo, storeRoot, reclaimID)
	markReleased(t, storeRoot, reclaimID)

	keptID := gcUnitID(1)
	cutGCUnit(t, repo, storeRoot, keptID) // no marker: kept-not-eligible

	// Unrelated content elsewhere in the store root.
	otherDir := filepath.Join(storeRoot, ".verdi", "data", "other")
	if err := os.MkdirAll(otherDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherDir, "file.txt"), []byte("untouched\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	excluded := []string{
		RequestStagingPath(storeRoot, reclaimID),
		RequestPath(storeRoot, reclaimID),
		reclaimPath,
		ReleasedPath(storeRoot, reclaimID),
		LockPath(storeRoot, reclaimID),
	}
	before := hashTreeExcluding(t, storeRoot, excluded)

	results, _, err := GC(context.Background(), storeRoot, repo.Dir)
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	byID := map[string]GCResult{}
	for _, r := range results {
		byID[r.WorkspaceID] = r
	}
	if byID[reclaimID].Outcome != Reclaimed {
		t.Fatalf("reclaimID outcome = %v, want Reclaimed", byID[reclaimID].Outcome)
	}
	if byID[keptID].Outcome != KeepNotEligible {
		t.Fatalf("keptID outcome = %v, want KeepNotEligible", byID[keptID].Outcome)
	}

	after := hashTreeExcluding(t, storeRoot, excluded)
	if before != after {
		t.Fatalf("gc mutated content outside the reclaimed unit's own paths:\nbefore=%s\nafter=%s", before, after)
	}
	if mustPathKind(t, filepath.Join(otherDir, "file.txt")) != PathRegular {
		t.Fatal("unrelated file outside data/execution/ was deleted")
	}
}

// --- GCResult.Line rendering: distinct per outcome ---

func TestGCResult_Line_DistinctPerOutcome(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range []GCResult{
		{WorkspaceID: "u--0123456789ab", Outcome: ReclaimOrphaned},
		{WorkspaceID: "u--0123456789ab", Outcome: Reclaimed},
		{WorkspaceID: "u--0123456789ab", Outcome: KeepMalformed, Detail: "d"},
		{WorkspaceID: "u--0123456789ab", Outcome: KeepNotEligible},
		{WorkspaceID: "u--0123456789ab", Outcome: KeepDirty},
		{WorkspaceID: "u--0123456789ab", Outcome: KeepLocked, Detail: "d"},
		{WorkspaceID: "u--0123456789ab", Outcome: Partial, Detail: "d"},
	} {
		line := r.Line()
		if line == "" {
			t.Fatalf("empty line for %v", r)
		}
		if seen[line] {
			t.Fatalf("duplicate line %q for outcome %v", line, r.Outcome)
		}
		seen[line] = true
	}
}
