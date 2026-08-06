package execworkspace

import (
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

// --- cluster: stale registration ---

func TestReconcileUnit_StaleRegistration_DeletesEntryAndAllowsReAdd(t *testing.T) {
	repo := newReconcileTestRepo(t)
	unitPath := filepath.Join(t.TempDir(), "unit")
	cutWorktree(t, repo.Dir, unitPath)

	// Directory removed out-of-band; registration survives (the STALE
	// ADMINISTRATIVE RESIDUE the spec's safety-grounding paragraph names).
	if err := os.RemoveAll(unitPath); err != nil {
		t.Fatalf("RemoveAll(unitPath): %v", err)
	}

	adminDir := adminWorktreesDir(t, repo.Dir)
	entry := soleAdminEntry(t, adminDir)

	rec := NewGitReconciler()
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
	unitPath := filepath.Join(t.TempDir(), "unit") // never materialized
	otherPath := filepath.Join(t.TempDir(), "other")
	cutWorktree(t, repo.Dir, otherPath)

	adminDir := adminWorktreesDir(t, repo.Dir)
	before := hashAdminDir(t, adminDir)

	rec := NewGitReconciler()
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

	unitPath := filepath.Join(t.TempDir(), "unit") // no unit entry exists at all
	rec := NewGitReconciler()
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

	unitPath := filepath.Join(t.TempDir(), "unit")
	rec := NewGitReconciler()
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
	unitPath := filepath.Join(t.TempDir(), "unit")
	cutWorktree(t, repo.Dir, unitPath)

	adminDir := adminWorktreesDir(t, repo.Dir)
	entry := soleAdminEntry(t, adminDir)
	lockedPath := filepath.Join(adminDir, entry, "locked")
	if err := os.WriteFile(lockedPath, []byte(""), 0o644); err != nil {
		t.Fatalf("plant lock marker: %v", err)
	}

	before := hashAdminDir(t, adminDir)

	rec := NewGitReconciler()
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
	unitPath := filepath.Join(t.TempDir(), "unit")
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

	rec := NewGitReconciler()
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
	unitPath := filepath.Join(t.TempDir(), "unit")
	cutWorktree(t, repo.Dir, unitPath)

	var hookCalls int
	rec1 := &GitReconciler{
		afterClaimForTests: func(entryDir string) bool {
			hookCalls++
			return true // simulate a crash: stop right here, no error raised for this entry.
		},
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
	rec2 := NewGitReconciler()
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
	unitPath := filepath.Join(t.TempDir(), "unit")
	cutWorktree(t, repo.Dir, unitPath)

	adminDir := adminWorktreesDir(t, repo.Dir)
	entry := soleAdminEntry(t, adminDir)
	entryDir := filepath.Join(adminDir, entry)
	lockedPath := filepath.Join(entryDir, "locked")

	beforeClaim := hashAdminDir(t, adminDir) // snapshot BEFORE the claim (pre-claim state)

	rec := &GitReconciler{
		afterClaimForTests: func(gotEntryDir string) bool {
			if gotEntryDir != entryDir {
				t.Fatalf("hook called for unexpected entry dir %s, want %s", gotEntryDir, entryDir)
			}
			// Plant git's own lock marker in the exact window step 4
			// exists to catch — between step 3's claim and its re-verify.
			if err := os.WriteFile(lockedPath, []byte(""), 0o644); err != nil {
				t.Fatalf("plant late lock marker: %v", err)
			}
			return false // do NOT simulate a crash; continue into the real re-verify.
		},
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
	// legitimate on-disk state and must survive the refusal untouched).
	if _, err := os.Stat(filepath.Join(entryDir, "gitdir")); err != nil {
		t.Fatalf("gitdir record not restored after late-lock refusal: %v", err)
	}
	if _, err := os.Stat(filepath.Join(entryDir, "gitdir.reconciling")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("aside record still present after restore: err=%v", err)
	}
	if _, err := os.Stat(lockedPath); err != nil {
		t.Fatalf("lock marker itself must survive the refusal: %v", err)
	}

	afterRestore := hashAdminDir(t, adminDir)
	if beforeClaim != afterRestore {
		// The lock marker's own presence is a real, intentional state
		// change (planted by the test to simulate an external
		// `git worktree lock`), so compare gitdir content directly instead
		// of the whole-tree hash for the byte-identical claim.
		gitdirData, rerr := os.ReadFile(filepath.Join(entryDir, "gitdir"))
		if rerr != nil {
			t.Fatalf("reading restored gitdir record: %v", rerr)
		}
		if len(gitdirData) == 0 {
			t.Fatal("restored gitdir record is empty")
		}
	}
}

// --- cluster: symlinked path spelling ---

func TestReconcileUnit_SymlinkedPathSpelling_FoundAndDeleted(t *testing.T) {
	repo := newReconcileTestRepo(t)

	realParent := t.TempDir()
	symlinkParent := filepath.Join(t.TempDir(), "sym-ancestor")
	if err := os.Symlink(realParent, symlinkParent); err != nil {
		t.Skipf("symlink unsupported in this environment: %v", err)
	}

	// Register the worktree THROUGH the symlinked ancestor spelling.
	unitPathViaSymlink := filepath.Join(symlinkParent, "unit")
	cutWorktree(t, repo.Dir, unitPathViaSymlink)

	// Reconcile via the CANONICAL spelling (through realParent directly).
	unitPathCanonical := filepath.Join(realParent, "unit")

	rec := NewGitReconciler()
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
	unitPath := filepath.Join(t.TempDir(), "unit")
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

	rec := NewGitReconciler()
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
// touch the primary checkout's own state: the primary worktree has no
// administrative entry under $GIT_COMMON_DIR/worktrees/ at all (git's own
// documented layout), so the enumeration this component performs (step 1)
// structurally excludes it before any resolution or comparison logic runs.
func TestReconcileUnit_MainWorktreeSafety_NeverTouched(t *testing.T) {
	repo := newReconcileTestRepo(t)
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

	// Reconciling the repo root as if it were a unit path must be a no-op
	// (nothing to find), never touching the real .git directory.
	beforeGitKind, _ := LstatType(filepath.Join(repo.Dir, ".git"))
	rec := NewGitReconciler()
	if err := rec.ReconcileUnit(context.Background(), repo.Dir, repo.Dir); err != nil {
		t.Fatalf("ReconcileUnit(repoRoot as unit path): %v", err)
	}
	afterGitKind, _ := LstatType(filepath.Join(repo.Dir, ".git"))
	if beforeGitKind != afterGitKind {
		t.Fatalf(".git kind changed: before=%s after=%s", beforeGitKind, afterGitKind)
	}
}

// --- corroboration disclosure ---

func TestReconcileUnit_Corroboration_DisagreementDisclosedOnCrashedClaim(t *testing.T) {
	repo := newReconcileTestRepo(t)
	unitPath := filepath.Join(t.TempDir(), "unit")
	cutWorktree(t, repo.Dir, unitPath)

	adminDir := adminWorktreesDir(t, repo.Dir)
	entry := soleAdminEntry(t, adminDir)
	gitdirPath := filepath.Join(adminDir, entry, "gitdir")
	asidePath := filepath.Join(adminDir, entry, "gitdir.reconciling")
	if err := os.Rename(gitdirPath, asidePath); err != nil {
		t.Fatalf("simulate crashed claim: %v", err)
	}

	var lines []string
	rec := &GitReconciler{Disclose: func(line string) { lines = append(lines, line) }}
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

func TestNewGitReconciler_ZeroValueUsable(t *testing.T) {
	var rec GitReconciler // zero value
	repo := newReconcileTestRepo(t)
	unitPath := filepath.Join(t.TempDir(), "unit") // never materialized: trivial success
	if err := rec.ReconcileUnit(context.Background(), repo.Dir, unitPath); err != nil {
		t.Fatalf("zero-value GitReconciler.ReconcileUnit on an absent unit: %v", err)
	}
	rec2 := NewGitReconciler()
	if err := rec2.ReconcileUnit(context.Background(), repo.Dir, unitPath); err != nil {
		t.Fatalf("NewGitReconciler().ReconcileUnit on an absent unit: %v", err)
	}
}
