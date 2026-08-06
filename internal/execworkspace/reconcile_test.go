package execworkspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// --- shared reconcile-test fixtures ---

func newReconcileTestRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "hello\n"}, Message: "root"},
	})
}

// adminWorktreesDir resolves $GIT_COMMON_DIR/worktrees for repoRoot exactly
// as GitReconciler itself does, for test setup and assertions.
func adminWorktreesDir(t *testing.T, repoRoot string) string {
	t.Helper()
	commonDir, err := gitx.CommonDir(context.Background(), repoRoot)
	if err != nil {
		t.Fatalf("gitx.CommonDir(%s): %v", repoRoot, err)
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(repoRoot, commonDir)
	}
	return filepath.Join(commonDir, "worktrees")
}

// cutWorktree cuts a real, detached `git worktree add` at path, registering
// it against repoRoot's administrative directory.
func cutWorktree(t *testing.T, repoRoot, path string) {
	t.Helper()
	if err := gitx.WorktreeAddDetached(context.Background(), repoRoot, path, "HEAD"); err != nil {
		t.Fatalf("WorktreeAddDetached(%s): %v", path, err)
	}
}

// soleAdminEntry returns the one directory entry under adminDir, failing
// the test if the count is not exactly 1 — used by tests that cut exactly
// one worktree and need its administrative entry id.
func soleAdminEntry(t *testing.T, adminDir string) string {
	t.Helper()
	entries, err := os.ReadDir(adminDir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", adminDir, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	if len(names) != 1 {
		t.Fatalf("adminDir %s: want exactly 1 entry, got %v", adminDir, names)
	}
	return names[0]
}

// hashAdminDir walks adminDir (names, lstat kind, and regular-file content)
// into one deterministic digest — used to assert an untouched entry (or the
// whole tree) is left BYTE-IDENTICAL by a reconcile call that should not
// have mutated it.
func hashAdminDir(t *testing.T, adminDir string) string {
	t.Helper()
	h := sha256.New()
	var paths []string
	if err := filepath.WalkDir(adminDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, p)
		return nil
	}); err != nil {
		t.Fatalf("WalkDir(%s): %v", adminDir, err)
	}
	sort.Strings(paths)
	for _, p := range paths {
		rel, _ := filepath.Rel(adminDir, p)
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

// newExecutionStoreRoot builds a store root whose data/execution/ exists —
// the ONLY namespace a GitReconciler is permitted to reconcile under.
func newExecutionStoreRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(ExecutionRoot(root), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", ExecutionRoot(root), err)
	}
	return root
}

// --- cluster: structural confinement to data/execution/ (RED probes) ---

func TestReconcileUnit_UnitPathOutsideExecutionRoot_RefusedWithRegistrationIntact(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)

	external := filepath.Join(t.TempDir(), "external-worktree")
	cutWorktree(t, repo.Dir, external)

	adminDir := adminWorktreesDir(t, repo.Dir)
	before := hashAdminDir(t, adminDir)

	rec := NewGitReconciler(storeRoot)
	err := rec.ReconcileUnit(context.Background(), repo.Dir, external)
	if err == nil {
		t.Fatal("ReconcileUnit on a live worktree OUTSIDE data/execution/: want refusal, got nil")
	}
	var outside *ErrOutsideExecutionRoot
	if !errors.As(err, &outside) {
		t.Fatalf("error is not *ErrOutsideExecutionRoot: %v (%T)", err, err)
	}
	// The refusal names BOTH paths (the offending unit path and the root it
	// is not under), so a human reading it can see why.
	if msg := err.Error(); !strings.Contains(msg, external) || !strings.Contains(msg, canonicalPath(ExecutionRoot(storeRoot))) {
		t.Fatalf("refusal does not name both paths: %q", msg)
	}
	if after := hashAdminDir(t, adminDir); after != before {
		t.Fatalf("out-of-root registration mutated by a refused reconcile:\nbefore=%s\nafter=%s", before, after)
	}
	if _, serr := gitx.StatusDirty(context.Background(), external); serr != nil {
		t.Fatalf("external worktree no longer functional after refusal: %v", serr)
	}
}

func TestReconcileUnit_SymlinkAtUnitPathResolvingOutside_Refused(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)

	external := filepath.Join(t.TempDir(), "external-worktree")
	cutWorktree(t, repo.Dir, external)

	unitPath := UnitPath(storeRoot, "unit")
	if err := os.Symlink(external, unitPath); err != nil {
		t.Skipf("symlink unsupported in this environment: %v", err)
	}

	adminDir := adminWorktreesDir(t, repo.Dir)
	before := hashAdminDir(t, adminDir)

	rec := NewGitReconciler(storeRoot)
	err := rec.ReconcileUnit(context.Background(), repo.Dir, unitPath)
	if err == nil {
		t.Fatal("ReconcileUnit on a symlinked unit path resolving OUTSIDE data/execution/: want refusal, got nil")
	}
	var outside *ErrOutsideExecutionRoot
	if !errors.As(err, &outside) {
		t.Fatalf("error is not *ErrOutsideExecutionRoot: %v (%T)", err, err)
	}
	// A LEXICAL prefix check would have admitted this path: it spells out
	// as a direct child of the execution root. Only canonicalization catches
	// it, so the refusal must report a canonical path outside the root.
	if outside.CanonicalUnitPath != canonicalPath(external) {
		t.Fatalf("refusal reports canonical unit path %q, want the symlink target's canonical path %q", outside.CanonicalUnitPath, canonicalPath(external))
	}
	if after := hashAdminDir(t, adminDir); after != before {
		t.Fatalf("out-of-root registration mutated via a symlinked unit path:\nbefore=%s\nafter=%s", before, after)
	}
	if _, serr := gitx.StatusDirty(context.Background(), external); serr != nil {
		t.Fatalf("external worktree no longer functional after refusal: %v", serr)
	}
}

// --- cluster: locked crashed claim (refusal must not mutate the entry) ---

func TestReconcileUnit_CrashedClaimLocked_RefusesWithoutMutatingEntry(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	unitPath := UnitPath(storeRoot, "unit")
	cutWorktree(t, repo.Dir, unitPath)

	adminDir := adminWorktreesDir(t, repo.Dir)
	entry := soleAdminEntry(t, adminDir)
	entryDir := filepath.Join(adminDir, entry)

	if err := os.Rename(filepath.Join(entryDir, "gitdir"), filepath.Join(entryDir, "gitdir.reconciling")); err != nil {
		t.Fatalf("simulate crashed claim: %v", err)
	}
	if err := os.WriteFile(filepath.Join(entryDir, "locked"), []byte(""), 0o644); err != nil {
		t.Fatalf("plant lock marker: %v", err)
	}

	before := hashAdminDir(t, adminDir)

	rec := NewGitReconciler(storeRoot)
	err := rec.ReconcileUnit(context.Background(), repo.Dir, unitPath)
	if err == nil {
		t.Fatal("ReconcileUnit on a LOCKED crashed claim: want typed refusal, got nil")
	}
	var lockedErr *ErrWorktreeLocked
	if !errors.As(err, &lockedErr) {
		t.Fatalf("error is not *ErrWorktreeLocked: %v (%T)", err, err)
	}
	if _, serr := os.Stat(filepath.Join(entryDir, "gitdir.reconciling")); serr != nil {
		t.Fatalf("aside record must survive a locked refusal untouched: %v", serr)
	}
	if _, serr := os.Stat(filepath.Join(entryDir, "gitdir")); !errors.Is(serr, os.ErrNotExist) {
		t.Fatalf("locked refusal restored the gitdir record (entry mutated): stat err=%v", serr)
	}
	if after := hashAdminDir(t, adminDir); after != before {
		t.Fatalf("locked crashed-claim entry mutated by a refused reconcile:\nbefore=%s\nafter=%s", before, after)
	}
}

// --- cluster: stale registration ---

func TestReconcileUnit_StaleRegistration_DeletesEntryAndAllowsReAdd(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	unitPath := UnitPath(storeRoot, "unit")
	cutWorktree(t, repo.Dir, unitPath)

	// Directory removed out-of-band; registration survives (the STALE
	// ADMINISTRATIVE RESIDUE the spec's safety-grounding paragraph names).
	if err := os.RemoveAll(unitPath); err != nil {
		t.Fatalf("RemoveAll(unitPath): %v", err)
	}

	adminDir := adminWorktreesDir(t, repo.Dir)
	entry := soleAdminEntry(t, adminDir)

	rec := NewGitReconciler(storeRoot)
	if err := rec.ReconcileUnit(context.Background(), repo.Dir, unitPath); err != nil {
		t.Fatalf("ReconcileUnit: %v", err)
	}

	if _, err := os.Stat(filepath.Join(adminDir, entry)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale admin entry %s still present after reconcile (stat err=%v)", entry, err)
	}

	// `git worktree add` at unitPath must now succeed — git no longer
	// refuses it as "missing but already registered".
	if err := gitx.WorktreeAddDetached(context.Background(), repo.Dir, unitPath, "HEAD"); err != nil {
		t.Fatalf("WorktreeAddDetached after reconcile: %v", err)
	}

	// The test re-verifies the postcondition itself, independently of the
	// production re-enumeration: after this FRESH add, exactly one
	// administrative entry resolves to unitPath again (the new, legitimate
	// registration).
	adminDir2 := adminWorktreesDir(t, repo.Dir)
	matches, err := scanAdminDir(adminDir2, canonicalPath(unitPath))
	if err != nil {
		t.Fatalf("scanAdminDir: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("after fresh WorktreeAddDetached, want exactly 1 admin entry resolving to unitPath, got %v", matches)
	}
}

// --- cluster: out-of-scope entry ---

func TestReconcileUnit_OutOfScopeEntry_LeftByteIdentical(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	unitPath := UnitPath(storeRoot, "unit") // never materialized
	otherPath := filepath.Join(t.TempDir(), "other")
	cutWorktree(t, repo.Dir, otherPath)

	adminDir := adminWorktreesDir(t, repo.Dir)
	before := hashAdminDir(t, adminDir)

	rec := NewGitReconciler(storeRoot)
	if err := rec.ReconcileUnit(context.Background(), repo.Dir, unitPath); err != nil {
		t.Fatalf("ReconcileUnit: %v", err)
	}

	after := hashAdminDir(t, adminDir)
	if before != after {
		t.Fatalf("admin dir mutated by reconcile of an unrelated unit path:\nbefore=%s\nafter=%s", before, after)
	}
	// otherPath's worktree must still function.
	if _, err := gitx.StatusDirty(context.Background(), otherPath); err != nil {
		t.Fatalf("otherPath's worktree broken after unrelated reconcile: %v", err)
	}
}

// --- cluster: unresolvable entry ---

func TestReconcileUnit_UnresolvableEntry_LeftUntouched(t *testing.T) {
	repo := newReconcileTestRepo(t)
	otherPath := filepath.Join(t.TempDir(), "other")
	cutWorktree(t, repo.Dir, otherPath)

	adminDir := adminWorktreesDir(t, repo.Dir)
	entry := soleAdminEntry(t, adminDir)
	gitdirPath := filepath.Join(adminDir, entry, "gitdir")
	if err := os.Remove(gitdirPath); err != nil {
		t.Fatalf("remove gitdir record (simulate unresolvable entry): %v", err)
	}

	before := hashAdminDir(t, adminDir)

	storeRoot := newExecutionStoreRoot(t)
	unitPath := UnitPath(storeRoot, "unit") // no unit entry exists at all
	rec := NewGitReconciler(storeRoot)
	if err := rec.ReconcileUnit(context.Background(), repo.Dir, unitPath); err != nil {
		t.Fatalf("ReconcileUnit (unresolvable entry present, no unit entry): want success (no unit entry exists), got %v", err)
	}

	after := hashAdminDir(t, adminDir)
	if before != after {
		t.Fatalf("unresolvable entry mutated: before=%s after=%s", before, after)
	}
}

// --- cluster: broken gitdir record ---

func TestReconcileUnit_BrokenGitdirRecord_LeftUntouched(t *testing.T) {
	repo := newReconcileTestRepo(t)
	otherPath := filepath.Join(t.TempDir(), "other")
	cutWorktree(t, repo.Dir, otherPath)

	adminDir := adminWorktreesDir(t, repo.Dir)
	entry := soleAdminEntry(t, adminDir)
	gitdirPath := filepath.Join(adminDir, entry, "gitdir")
	if err := os.WriteFile(gitdirPath, []byte("not a real path\x00garbage\n"), 0o644); err != nil {
		t.Fatalf("corrupt gitdir record: %v", err)
	}

	before := hashAdminDir(t, adminDir)

	storeRoot := newExecutionStoreRoot(t)
	unitPath := UnitPath(storeRoot, "unit")
	rec := NewGitReconciler(storeRoot)
	if err := rec.ReconcileUnit(context.Background(), repo.Dir, unitPath); err != nil {
		t.Fatalf("ReconcileUnit: %v", err)
	}

	after := hashAdminDir(t, adminDir)
	if before != after {
		t.Fatalf("broken gitdir record mutated: before=%s after=%s", before, after)
	}
}

// --- cluster: locked entry ---

func TestReconcileUnit_LockedEntry_TypedRefusal(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	unitPath := UnitPath(storeRoot, "unit")
	cutWorktree(t, repo.Dir, unitPath)

	adminDir := adminWorktreesDir(t, repo.Dir)
	entry := soleAdminEntry(t, adminDir)
	lockedPath := filepath.Join(adminDir, entry, "locked")
	if err := os.WriteFile(lockedPath, []byte(""), 0o644); err != nil {
		t.Fatalf("plant lock marker: %v", err)
	}

	before := hashAdminDir(t, adminDir)

	rec := NewGitReconciler(storeRoot)
	err := rec.ReconcileUnit(context.Background(), repo.Dir, unitPath)
	if err == nil {
		t.Fatal("ReconcileUnit on locked entry: want error, got nil")
	}
	var lockedErr *ErrWorktreeLocked
	if !errors.As(err, &lockedErr) {
		t.Fatalf("error is not *ErrWorktreeLocked: %v (%T)", err, err)
	}

	msg := err.Error()
	unlockIdx := strings.Index(msg, "git worktree unlock")
	pruneIdx := strings.Index(msg, "git worktree prune --expire=now")
	if unlockIdx == -1 {
		t.Fatalf("message missing unlock remedy: %q", msg)
	}
	if pruneIdx == -1 {
		t.Fatalf("message missing prune remedy: %q", msg)
	}
	if unlockIdx > pruneIdx {
		t.Fatalf("message does not name unlock BEFORE prune: %q", msg)
	}

	after := hashAdminDir(t, adminDir)
	if before != after {
		t.Fatalf("locked entry (including its lock marker) mutated by a refused reconcile:\nbefore=%s\nafter=%s", before, after)
	}
}

// --- cluster: crashed claim (single invocation self-heal) ---

func TestReconcileUnit_CrashedClaim_ResolvesAndCompletes(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	unitPath := UnitPath(storeRoot, "unit")
	cutWorktree(t, repo.Dir, unitPath)

	adminDir := adminWorktreesDir(t, repo.Dir)
	entry := soleAdminEntry(t, adminDir)
	gitdirPath := filepath.Join(adminDir, entry, "gitdir")
	asidePath := filepath.Join(adminDir, entry, "gitdir.reconciling")

	// Hand-rename gitdir -> gitdir.reconciling: simulates a crash that
	// happened AFTER a prior reconciliation's step 3 claim but before its
	// step 5 delete — "a state only this component creates".
	if err := os.Rename(gitdirPath, asidePath); err != nil {
		t.Fatalf("simulate crashed claim: %v", err)
	}

	rec := NewGitReconciler(storeRoot)
	if err := rec.ReconcileUnit(context.Background(), repo.Dir, unitPath); err != nil {
		t.Fatalf("ReconcileUnit resolving crashed claim: want success, got %v", err)
	}

	if _, err := os.Stat(filepath.Join(adminDir, entry)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("crashed-claim entry still present after reconcile (stat err=%v)", err)
	}
}

// --- cluster: crashed claim, TWO-INVOCATION self-heal ---

// TestReconcileUnit_CrashedClaim_TwoInvocationSelfHeal proves the exact
// sequence the spec's crash-window paragraph names: invocation 1 claims the
// entry (step 3) and then is interrupted before deletion (step 5) — its
// postcondition then FAILS, disclosed, never a false success; invocation 2,
// a fresh reconciler, resolves through the surviving aside record,
// re-claims (already held, simply proceeds), completes the deletion, and
// its own postcondition passes.
func TestReconcileUnit_CrashedClaim_TwoInvocationSelfHeal(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	unitPath := UnitPath(storeRoot, "unit")
	cutWorktree(t, repo.Dir, unitPath)

	var hookCalls int
	rec1 := NewGitReconciler(storeRoot)
	rec1.afterClaimForTests = func(entryDir string) bool {
		hookCalls++
		return true // simulate a crash: stop right here, no error raised for this entry.
	}

	err1 := rec1.ReconcileUnit(context.Background(), repo.Dir, unitPath)
	if err1 == nil {
		t.Fatal("invocation 1 (simulated crash between claim and delete): want a disclosed postcondition failure, got nil (false success)")
	}
	if !strings.Contains(err1.Error(), "postcondition") {
		t.Fatalf("invocation 1's error is not the postcondition-failure disclosure: %v", err1)
	}
	if hookCalls != 1 {
		t.Fatalf("afterClaimForTests hook called %d times, want 1", hookCalls)
	}

	// The aside record must have survived invocation 1 (the claim held,
	// deletion never ran).
	adminDir := adminWorktreesDir(t, repo.Dir)
	entry := soleAdminEntry(t, adminDir)
	if _, err := os.Stat(filepath.Join(adminDir, entry, "gitdir.reconciling")); err != nil {
		t.Fatalf("aside record missing after simulated crash: %v", err)
	}
	if _, err := os.Stat(filepath.Join(adminDir, entry, "gitdir")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("gitdir record present after claim (should be aside-only): err=%v", err)
	}

	// Invocation 2: a FRESH reconciler (no hook — the crash is over) must
	// self-heal.
	rec2 := NewGitReconciler(storeRoot)
	if err := rec2.ReconcileUnit(context.Background(), repo.Dir, unitPath); err != nil {
		t.Fatalf("invocation 2 (fresh, after simulated crash): want success, got %v", err)
	}

	matches, serr := scanAdminDir(adminDir, canonicalPath(unitPath))
	if serr != nil {
		t.Fatalf("scanAdminDir: %v", serr)
	}
	if len(matches) != 0 {
		t.Fatalf("survivors after invocation 2: %v", matches)
	}
}

// --- cluster: re-verify window (RESTORE path) ---

func TestReconcileUnit_ReVerifyWindow_RestoresOnLateLockMarker(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	unitPath := UnitPath(storeRoot, "unit")
	cutWorktree(t, repo.Dir, unitPath)

	adminDir := adminWorktreesDir(t, repo.Dir)
	entry := soleAdminEntry(t, adminDir)
	entryDir := filepath.Join(adminDir, entry)
	lockedPath := filepath.Join(entryDir, "locked")

	// Snapshot the gitdir record's EXACT BYTES before the claim: step 4's
	// RESTORE must return the entry byte-identical to this pre-claim state.
	gitdirPath := filepath.Join(entryDir, "gitdir")
	gitdirBefore, rerr := os.ReadFile(gitdirPath)
	if rerr != nil {
		t.Fatalf("reading pre-claim gitdir record: %v", rerr)
	}
	if len(gitdirBefore) == 0 {
		t.Fatal("setup sanity: pre-claim gitdir record is empty")
	}

	rec := NewGitReconciler(storeRoot)
	rec.afterClaimForTests = func(gotEntryDir string) bool {
		if gotEntryDir != entryDir {
			t.Fatalf("hook called for unexpected entry dir %s, want %s", gotEntryDir, entryDir)
		}
		// Plant git's own lock marker in the exact window step 4
		// exists to catch — between step 3's claim and its re-verify.
		if err := os.WriteFile(lockedPath, []byte(""), 0o644); err != nil {
			t.Fatalf("plant late lock marker: %v", err)
		}
		return false // do NOT simulate a crash; continue into the real re-verify.
	}

	err := rec.ReconcileUnit(context.Background(), repo.Dir, unitPath)
	if err == nil {
		t.Fatal("ReconcileUnit with a late-landing lock marker: want typed refusal, got nil")
	}
	var lockedErr *ErrWorktreeLocked
	if !errors.As(err, &lockedErr) {
		t.Fatalf("error is not *ErrWorktreeLocked: %v (%T)", err, err)
	}

	// The rename-back must be byte-identical to the pre-claim state, except
	// for the freshly planted lock marker itself (which is now part of the
	// legitimate on-disk state and must survive the refusal untouched). The
	// whole-tree hash therefore cannot be compared directly — the marker's
	// own presence is a real, intentional state change planted by this test
	// to simulate an external `git worktree lock` — so the byte-identity
	// claim is asserted against the record's captured pre-claim bytes.
	gitdirAfter, rerr := os.ReadFile(gitdirPath)
	if rerr != nil {
		t.Fatalf("gitdir record not restored after late-lock refusal: %v", rerr)
	}
	if !bytes.Equal(gitdirBefore, gitdirAfter) {
		t.Fatalf("restored gitdir record is not byte-identical to its pre-claim state:\nbefore=%q\nafter=%q", gitdirBefore, gitdirAfter)
	}
	if _, err := os.Stat(filepath.Join(entryDir, "gitdir.reconciling")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("aside record still present after restore: err=%v", err)
	}
	if _, err := os.Stat(lockedPath); err != nil {
		t.Fatalf("lock marker itself must survive the refusal: %v", err)
	}
}

// --- cluster: symlinked path spelling ---

func TestReconcileUnit_SymlinkedPathSpelling_FoundAndDeleted(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)

	// A second spelling of the SAME execution root, reached through a
	// symlinked ancestor — the spec's "/tmp versus /private/tmp" case. Both
	// spellings name one location, so both are inside the confinement root.
	symlinkedExecRoot := filepath.Join(t.TempDir(), "sym-execution")
	if err := os.Symlink(ExecutionRoot(storeRoot), symlinkedExecRoot); err != nil {
		t.Skipf("symlink unsupported in this environment: %v", err)
	}

	// Register the worktree THROUGH the symlinked ancestor spelling.
	unitPathViaSymlink := filepath.Join(symlinkedExecRoot, "unit")
	cutWorktree(t, repo.Dir, unitPathViaSymlink)

	// Reconcile via the CANONICAL spelling (through the store root directly).
	unitPathCanonical := UnitPath(storeRoot, "unit")

	rec := NewGitReconciler(storeRoot)
	if err := rec.ReconcileUnit(context.Background(), repo.Dir, unitPathCanonical); err != nil {
		t.Fatalf("ReconcileUnit via canonical spelling: %v", err)
	}

	adminDir := adminWorktreesDir(t, repo.Dir)
	entries, err := os.ReadDir(adminDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ReadDir(%s): %v", adminDir, err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		resolved, _, ok := resolveAdminEntry(filepath.Join(adminDir, e.Name()))
		if ok && (resolved == canonicalPath(unitPathViaSymlink) || resolved == canonicalPath(unitPathCanonical)) {
			t.Fatalf("entry %s still resolves to the unit after reconcile via canonical spelling — vacuous postcondition", e.Name())
		}
	}
}

// --- cluster: registry-only unit ---

func TestReconcileUnit_RegistryOnlyUnit_Cleared(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	unitPath := UnitPath(storeRoot, "unit")
	cutWorktree(t, repo.Dir, unitPath)

	// Nothing on disk at the unit path at all (a registry-only unit: the
	// filesystem alone could never surface this state).
	if err := os.RemoveAll(unitPath); err != nil {
		t.Fatalf("RemoveAll(unitPath): %v", err)
	}
	if _, err := os.Stat(unitPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unitPath still present: err=%v", err)
	}

	adminDir := adminWorktreesDir(t, repo.Dir)
	matchesBefore, err := scanAdminDir(adminDir, canonicalPath(unitPath))
	if err != nil {
		t.Fatalf("scanAdminDir: %v", err)
	}
	if len(matchesBefore) != 1 {
		t.Fatalf("setup sanity: want exactly 1 registry entry before reconcile, got %v", matchesBefore)
	}

	rec := NewGitReconciler(storeRoot)
	if err := rec.ReconcileUnit(context.Background(), repo.Dir, unitPath); err != nil {
		t.Fatalf("ReconcileUnit(registry-only unit): %v", err)
	}

	matchesAfter, err := scanAdminDir(adminDir, canonicalPath(unitPath))
	if err != nil {
		t.Fatalf("scanAdminDir: %v", err)
	}
	if len(matchesAfter) != 0 {
		t.Fatalf("registry entry survives reconcile: %v", matchesAfter)
	}
}

// --- cluster: main-worktree safety ---

// TestReconcileUnit_MainWorktreeSafety_NeverTouched proves STRUCTURALLY —
// not merely by absence of a symptom — that reconciling any unit can never
// touch the primary checkout's own state, on TWO independent grounds: the
// primary worktree has no administrative entry under
// $GIT_COMMON_DIR/worktrees/ at all (git's own documented layout), so the
// enumeration this component performs (step 1) excludes it before any
// resolution or comparison logic runs; AND a repository root is not a
// direct child of any execution root, so naming it as a unit path is
// refused before that enumeration even begins.
func TestReconcileUnit_MainWorktreeSafety_NeverTouched(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	otherPath := filepath.Join(t.TempDir(), "other")
	cutWorktree(t, repo.Dir, otherPath)

	adminDir := adminWorktreesDir(t, repo.Dir)
	entries, err := os.ReadDir(adminDir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", adminDir, err)
	}
	canonicalRepoRoot := canonicalPath(repo.Dir)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		resolved, _, ok := resolveAdminEntry(filepath.Join(adminDir, e.Name()))
		if ok && resolved == canonicalRepoRoot {
			t.Fatalf("administrative directory unexpectedly contains an entry for the main worktree itself: %s", e.Name())
		}
	}

	// Reconciling the repo root as if it were a unit path is REFUSED (it is
	// not under this reconciler's execution root), and never touches the
	// real .git directory.
	beforeGitKind, _ := LstatType(filepath.Join(repo.Dir, ".git"))
	rec := NewGitReconciler(storeRoot)
	err = rec.ReconcileUnit(context.Background(), repo.Dir, repo.Dir)
	if err == nil {
		t.Fatal("ReconcileUnit(repoRoot as unit path): want confinement refusal, got nil")
	}
	var outside *ErrOutsideExecutionRoot
	if !errors.As(err, &outside) {
		t.Fatalf("error is not *ErrOutsideExecutionRoot: %v (%T)", err, err)
	}
	afterGitKind, _ := LstatType(filepath.Join(repo.Dir, ".git"))
	if beforeGitKind != afterGitKind {
		t.Fatalf(".git kind changed: before=%s after=%s", beforeGitKind, afterGitKind)
	}
}

// --- corroboration disclosure ---

func TestReconcileUnit_Corroboration_DisagreementDisclosedOnCrashedClaim(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	unitPath := UnitPath(storeRoot, "unit")
	cutWorktree(t, repo.Dir, unitPath)

	adminDir := adminWorktreesDir(t, repo.Dir)
	entry := soleAdminEntry(t, adminDir)
	gitdirPath := filepath.Join(adminDir, entry, "gitdir")
	asidePath := filepath.Join(adminDir, entry, "gitdir.reconciling")
	if err := os.Rename(gitdirPath, asidePath); err != nil {
		t.Fatalf("simulate crashed claim: %v", err)
	}

	var lines []string
	rec := NewGitReconciler(storeRoot)
	rec.Disclose = func(line string) { lines = append(lines, line) }
	if err := rec.ReconcileUnit(context.Background(), repo.Dir, unitPath); err != nil {
		t.Fatalf("ReconcileUnit: %v", err)
	}

	found := false
	for _, l := range lines {
		if strings.Contains(l, "corroboration disagreement") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a corroboration-disagreement disclosure (WorktreeList cannot see a crashed-claim entry), got lines: %v", lines)
	}
}

// --- constructor ---

// TestNewGitReconciler_ZeroValueFailsClosed proves the deliberate reversal
// of this type's former "zero value is usable" property: a GitReconciler
// that was not given a store root carries no confinement root, and so must
// reconcile NOTHING rather than anything it can resolve.
func TestNewGitReconciler_ZeroValueFailsClosed(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	unitPath := UnitPath(storeRoot, "unit") // never materialized

	var zero GitReconciler
	err := zero.ReconcileUnit(context.Background(), repo.Dir, unitPath)
	if err == nil {
		t.Fatal("zero-value GitReconciler.ReconcileUnit: want a fail-closed refusal, got nil")
	}
	var outside *ErrOutsideExecutionRoot
	if !errors.As(err, &outside) {
		t.Fatalf("error is not *ErrOutsideExecutionRoot: %v (%T)", err, err)
	}
	if !strings.Contains(err.Error(), "not built by NewGitReconciler") {
		t.Fatalf("zero-value refusal does not name its cause: %q", err.Error())
	}

	// A properly constructed reconciler accepts the same in-root path (an
	// absent unit is a trivial success: nothing is registered).
	if err := NewGitReconciler(storeRoot).ReconcileUnit(context.Background(), repo.Dir, unitPath); err != nil {
		t.Fatalf("NewGitReconciler(storeRoot).ReconcileUnit on an absent in-root unit: %v", err)
	}
}

func TestNewGitReconciler_CanonicalizesExecutionRoot(t *testing.T) {
	storeRoot := newExecutionStoreRoot(t)
	rec := NewGitReconciler(storeRoot)
	if want := canonicalPath(ExecutionRoot(storeRoot)); rec.canonicalExecutionRoot != want {
		t.Fatalf("canonicalExecutionRoot = %q, want %q", rec.canonicalExecutionRoot, want)
	}
	// Constructed from a NON-canonical spelling of the same store root, the
	// captured root must be identical — otherwise the confinement check
	// would depend on how the caller spelled its store root.
	symlinkedStore := filepath.Join(t.TempDir(), "sym-store")
	if err := os.Symlink(storeRoot, symlinkedStore); err != nil {
		t.Skipf("symlink unsupported in this environment: %v", err)
	}
	if got := NewGitReconciler(symlinkedStore).canonicalExecutionRoot; got != rec.canonicalExecutionRoot {
		t.Fatalf("symlinked store root captured %q, want %q", got, rec.canonicalExecutionRoot)
	}
}

// TestReconcileUnit_NestedBelowUnit_Refused proves the check is
// DIRECT-CHILD equality, not a prefix test: a path nested deeper under the
// execution root is not a unit and is refused.
func TestReconcileUnit_NestedBelowUnit_Refused(t *testing.T) {
	repo := newReconcileTestRepo(t)
	storeRoot := newExecutionStoreRoot(t)
	nested := filepath.Join(UnitPath(storeRoot, "unit"), "src")

	err := NewGitReconciler(storeRoot).ReconcileUnit(context.Background(), repo.Dir, nested)
	if err == nil {
		t.Fatal("ReconcileUnit on a path nested BELOW a unit: want refusal, got nil")
	}
	var outside *ErrOutsideExecutionRoot
	if !errors.As(err, &outside) {
		t.Fatalf("error is not *ErrOutsideExecutionRoot: %v (%T)", err, err)
	}
}

// --- unexported units: parseGitdirRecord ---

// TestParseGitdirRecord tests the record parser DIRECTLY, at the byte
// level: the enumeration/resolution path only ever reaches it with records
// git itself wrote, so the whitespace, relative-path and parent-of-record
// rules it enforces are otherwise never exercised on their own terms.
func TestParseGitdirRecord(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
		ok   bool
	}{
		{name: "trailing newline (git's own spelling)", data: "/repo/unit/.git\n", want: "/repo/unit", ok: true},
		{name: "CRLF line ending", data: "/repo/unit/.git\r\n", want: "/repo/unit", ok: true},
		{name: "no trailing newline", data: "/repo/unit/.git", want: "/repo/unit", ok: true},
		{name: "surrounding spaces and tabs", data: "  \t/repo/unit/.git \t\n", want: "/repo/unit", ok: true},
		{name: "record naming the unit directly (parent-of-record rule is unconditional)", data: "/repo/unit\n", want: "/repo", ok: true},
		// Git never writes a trailing separator here. The parser applies the
		// parent-of-record rule to the record exactly as written, so this
		// spelling yields the record path itself — which then matches NO
		// unit path and so leaves the entry untouched. Recorded as the
		// actual, fail-closed behavior rather than silently widened.
		{name: "trailing slash (never git's spelling; yields a non-matching path)", data: "/repo/unit/.git/\n", want: "/repo/unit/.git", ok: true},
		{name: "uncleaned components", data: "/repo/./sub/../unit/.git\n", want: "/repo/unit", ok: true},
		{name: "empty", data: "", ok: false},
		{name: "whitespace only", data: " \n\t", ok: false},
		{name: "relative record", data: "unit/.git\n", ok: false},
		{name: "dot-relative record", data: "./.git\n", ok: false},
		{name: "parent-relative record", data: "../unit/.git\n", ok: false},
		{name: "garbage bytes resolving nowhere", data: "not a real path\x00garbage\n", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseGitdirRecord([]byte(tt.data))
			if ok != tt.ok {
				t.Fatalf("parseGitdirRecord(%q) ok = %v, want %v (path=%q)", tt.data, ok, tt.ok, got)
			}
			if !tt.ok {
				if got != "" {
					t.Fatalf("rejected record returned a non-empty path %q", got)
				}
				return
			}
			if got != tt.want {
				t.Fatalf("parseGitdirRecord(%q) = %q, want %q", tt.data, got, tt.want)
			}
		})
	}
}

// --- unexported units: canonicalPath ---

func TestCanonicalPath(t *testing.T) {
	tests := []struct {
		name string
		// build returns the path to canonicalize and the expected result.
		build func(t *testing.T) (in, want string)
	}{
		{
			name: "existing path resolves to its real spelling",
			build: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				real, err := filepath.EvalSymlinks(dir)
				if err != nil {
					t.Fatalf("EvalSymlinks(%s): %v", dir, err)
				}
				return dir, filepath.Clean(real)
			},
		},
		{
			name: "symlinked ANCESTOR of an absent leaf is still resolved",
			build: func(t *testing.T) (string, string) {
				target := t.TempDir()
				link := filepath.Join(t.TempDir(), "link")
				if err := os.Symlink(target, link); err != nil {
					t.Skipf("symlink unsupported in this environment: %v", err)
				}
				realTarget, err := filepath.EvalSymlinks(target)
				if err != nil {
					t.Fatalf("EvalSymlinks(%s): %v", target, err)
				}
				// The leaf does NOT exist: EvalSymlinks on the whole path
				// fails, and a bare Clean would leave the symlinked ancestor
				// unresolved — the exact failure this function exists to avoid.
				return filepath.Join(link, "absent-leaf"), filepath.Join(filepath.Clean(realTarget), "absent-leaf")
			},
		},
		{
			name: "several absent components below an existing ancestor",
			build: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				real, err := filepath.EvalSymlinks(dir)
				if err != nil {
					t.Fatalf("EvalSymlinks(%s): %v", dir, err)
				}
				return filepath.Join(dir, "a", "b", "c"), filepath.Join(filepath.Clean(real), "a", "b", "c")
			},
		},
		{
			name: "wholly absent absolute path is cleaned, never rejected",
			build: func(t *testing.T) (string, string) {
				return "/no/such/path/anywhere/./x", "/no/such/path/anywhere/x"
			},
		},
		{
			name:  "empty stays empty",
			build: func(t *testing.T) (string, string) { return "", "" },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in, want := tt.build(t)
			if got := canonicalPath(in); got != want {
				t.Fatalf("canonicalPath(%q) = %q, want %q", in, got, want)
			}
		})
	}
}

// --- unexported units: lockedMarkerPresent ---

func TestLockedMarkerPresent(t *testing.T) {
	tests := []struct {
		name    string
		build   func(t *testing.T) string
		want    bool
		wantErr bool
	}{
		{
			name:  "absent marker",
			build: func(t *testing.T) string { return filepath.Join(t.TempDir(), "locked") },
			want:  false,
		},
		{
			name: "regular file (git's own marker shape)",
			build: func(t *testing.T) string {
				p := filepath.Join(t.TempDir(), "locked")
				if err := os.WriteFile(p, []byte("locked by a human\n"), 0o644); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
				return p
			},
			want: true,
		},
		{
			name: "directory at the marker path is still PRESENCE",
			build: func(t *testing.T) string {
				p := filepath.Join(t.TempDir(), "locked")
				if err := os.Mkdir(p, 0o755); err != nil {
					t.Fatalf("Mkdir: %v", err)
				}
				return p
			},
			want: true,
		},
		{
			name: "DANGLING symlink is presence (lstat, never a following stat)",
			build: func(t *testing.T) string {
				p := filepath.Join(t.TempDir(), "locked")
				if err := os.Symlink(filepath.Join(t.TempDir(), "nowhere"), p); err != nil {
					t.Skipf("symlink unsupported in this environment: %v", err)
				}
				return p
			},
			want: true,
		},
		{
			name: "permission denied is an ERROR, never a silent false",
			build: func(t *testing.T) string {
				if os.Geteuid() == 0 {
					t.Skip("running as root: permission denial is unobservable")
				}
				parent := filepath.Join(t.TempDir(), "sealed")
				if err := os.Mkdir(parent, 0o755); err != nil {
					t.Fatalf("Mkdir: %v", err)
				}
				p := filepath.Join(parent, "locked")
				if err := os.WriteFile(p, nil, 0o644); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
				if err := os.Chmod(parent, 0o000); err != nil {
					t.Fatalf("Chmod: %v", err)
				}
				t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })
				return p
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.build(t)
			got, err := lockedMarkerPresent(path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("lockedMarkerPresent(%s) = (%v, nil), want a non-nil error", path, got)
				}
				// The (false, err) pair must never be read as absence — the
				// caller checks err first, and reconcileEntry returns it.
				return
			}
			if err != nil {
				t.Fatalf("lockedMarkerPresent(%s): unexpected error %v", path, err)
			}
			if got != tt.want {
				t.Fatalf("lockedMarkerPresent(%s) = %v, want %v", path, got, tt.want)
			}
		})
	}
}

// --- unexported units: adminDirFor / resolveAdminDir ---

func TestAdminDirFor(t *testing.T) {
	tests := []struct {
		name      string
		repoRoot  string
		commonDir string
		want      string
	}{
		{name: "absolute common dir is used as given", repoRoot: "/repo", commonDir: "/repo/.git", want: "/repo/.git/worktrees"},
		{name: "absolute common dir elsewhere on the filesystem", repoRoot: "/repo/wt", commonDir: "/elsewhere/bare.git", want: "/elsewhere/bare.git/worktrees"},
		{name: "relative parent common dir resolves against repoRoot", repoRoot: "/repo/wt", commonDir: "../.git", want: "/repo/.git/worktrees"},
		{name: "relative same-dir common dir resolves against repoRoot", repoRoot: "/repo", commonDir: ".git", want: "/repo/.git/worktrees"},
		{name: "dot-relative common dir", repoRoot: "/repo", commonDir: "./.git", want: "/repo/.git/worktrees"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := adminDirFor(tt.repoRoot, tt.commonDir); got != tt.want {
				t.Fatalf("adminDirFor(%q, %q) = %q, want %q", tt.repoRoot, tt.commonDir, got, tt.want)
			}
		})
	}
}

func TestResolveAdminDir(t *testing.T) {
	repo := newReconcileTestRepo(t)
	rec := NewGitReconciler(newExecutionStoreRoot(t))

	// Happy path: a real repository, whose worktrees/ dir does not exist yet
	// (nothing has ever been cut) — resolveAdminDir reports the path without
	// requiring it to exist, and reconcilePass treats that absence as
	// "nothing registered".
	adminDir, err := rec.resolveAdminDir(context.Background(), repo.Dir)
	if err != nil {
		t.Fatalf("resolveAdminDir(%s): %v", repo.Dir, err)
	}
	if !filepath.IsAbs(adminDir) {
		t.Fatalf("resolveAdminDir returned a relative path: %q", adminDir)
	}
	if filepath.Base(adminDir) != "worktrees" {
		t.Fatalf("resolveAdminDir = %q, want a path ending in /worktrees", adminDir)
	}
	if _, serr := os.Stat(adminDir); !errors.Is(serr, os.ErrNotExist) {
		t.Fatalf("setup sanity: worktrees dir unexpectedly exists before any worktree is cut: %v", serr)
	}
	if err := rec.reconcilePass(context.Background(), adminDir, canonicalPath("/no/such/unit")); err != nil {
		t.Fatalf("reconcilePass with a MISSING worktrees dir: want trivial success, got %v", err)
	}

	// The same resolution from inside a LINKED worktree (where git may report
	// the common dir relatively) must land on the SAME administrative
	// directory as the main repository's.
	unitStore := newExecutionStoreRoot(t)
	unitPath := UnitPath(unitStore, "unit")
	cutWorktree(t, repo.Dir, unitPath)
	fromLinked, err := rec.resolveAdminDir(context.Background(), unitPath)
	if err != nil {
		t.Fatalf("resolveAdminDir(%s): %v", unitPath, err)
	}
	if canonicalPath(fromLinked) != canonicalPath(adminDir) {
		t.Fatalf("resolveAdminDir from a linked worktree = %q, want the main repo's %q", fromLinked, adminDir)
	}

	// Negative: a directory that is not a git repository at all.
	if got, err := rec.resolveAdminDir(context.Background(), t.TempDir()); err == nil {
		t.Fatalf("resolveAdminDir on a non-repository: want error, got %q", got)
	}
}

// --- unexported units: corroborate ---

// TestCorroborate covers the disclosure's BOTH directions. The
// disagreement direction was already proven end to end; the silent-when-
// agreeing direction never was, so a corroborate that disclosed on every
// call (or on none) would have passed the old suite either way.
func TestCorroborate(t *testing.T) {
	t.Run("agreement: both see the registration -> NO disclosure", func(t *testing.T) {
		repo := newReconcileTestRepo(t)
		storeRoot := newExecutionStoreRoot(t)
		unitPath := UnitPath(storeRoot, "unit")
		cutWorktree(t, repo.Dir, unitPath)

		var lines []string
		rec := NewGitReconciler(storeRoot)
		rec.Disclose = func(l string) { lines = append(lines, l) }
		rec.corroborate(context.Background(), repo.Dir, adminWorktreesDir(t, repo.Dir), unitPath, canonicalPath(unitPath))
		if len(lines) != 0 {
			t.Fatalf("corroboration disclosed while the two views AGREE: %v", lines)
		}
	})

	t.Run("agreement: neither sees a registration -> NO disclosure", func(t *testing.T) {
		repo := newReconcileTestRepo(t)
		storeRoot := newExecutionStoreRoot(t)
		other := filepath.Join(t.TempDir(), "other")
		cutWorktree(t, repo.Dir, other) // an unrelated registration exists
		unitPath := UnitPath(storeRoot, "unit")

		var lines []string
		rec := NewGitReconciler(storeRoot)
		rec.Disclose = func(l string) { lines = append(lines, l) }
		rec.corroborate(context.Background(), repo.Dir, adminWorktreesDir(t, repo.Dir), unitPath, canonicalPath(unitPath))
		if len(lines) != 0 {
			t.Fatalf("corroboration disclosed while the two views AGREE (both empty): %v", lines)
		}
	})

	t.Run("disagreement: crashed claim is invisible to WorktreeList -> disclosure", func(t *testing.T) {
		repo := newReconcileTestRepo(t)
		storeRoot := newExecutionStoreRoot(t)
		unitPath := UnitPath(storeRoot, "unit")
		cutWorktree(t, repo.Dir, unitPath)

		adminDir := adminWorktreesDir(t, repo.Dir)
		entryDir := filepath.Join(adminDir, soleAdminEntry(t, adminDir))
		if err := os.Rename(filepath.Join(entryDir, "gitdir"), filepath.Join(entryDir, "gitdir.reconciling")); err != nil {
			t.Fatalf("simulate crashed claim: %v", err)
		}

		var lines []string
		rec := NewGitReconciler(storeRoot)
		rec.Disclose = func(l string) { lines = append(lines, l) }
		rec.corroborate(context.Background(), repo.Dir, adminDir, unitPath, canonicalPath(unitPath))
		if len(lines) != 1 || !strings.Contains(lines[0], "corroboration disagreement") {
			t.Fatalf("want exactly one disagreement disclosure, got %v", lines)
		}
	})

	t.Run("WorktreeList failure is disclosed and never fatal", func(t *testing.T) {
		storeRoot := newExecutionStoreRoot(t)
		notARepo := t.TempDir()
		unitPath := UnitPath(storeRoot, "unit")

		var lines []string
		rec := NewGitReconciler(storeRoot)
		rec.Disclose = func(l string) { lines = append(lines, l) }
		rec.corroborate(context.Background(), notARepo, filepath.Join(notARepo, "worktrees"), unitPath, canonicalPath(unitPath))
		if len(lines) != 1 || !strings.Contains(lines[0], "gitx.WorktreeList failed") {
			t.Fatalf("want exactly one WorktreeList-failure disclosure, got %v", lines)
		}
	})

	t.Run("nil Disclose is a no-op default (corroboration still runs)", func(t *testing.T) {
		repo := newReconcileTestRepo(t)
		storeRoot := newExecutionStoreRoot(t)
		unitPath := UnitPath(storeRoot, "unit")
		rec := NewGitReconciler(storeRoot) // Disclose left nil
		rec.corroborate(context.Background(), repo.Dir, adminWorktreesDir(t, repo.Dir), unitPath, canonicalPath(unitPath))
	})
}
